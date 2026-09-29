package pdf

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5" //nolint:gosec // ISO 32000-1 Algorithm 2 names MD5; the format cannot use another hash.
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
)

// The standard security handler. ISO 32000-1 clause 7.6.4 defines the password
// algorithms, the file key, and the crypt filters; ISO 32000-2 clause 7.6.4.3
// adds the AES-256 revisions. The reader opens a document whose empty user or
// owner password authenticates, which is the reader's answer to the password
// prompt a person leaves blank, and refuses one whose key material does not.
//
// The key derivation and the password checks are pure functions so they can be
// tested against published vectors without a file. Everything else is the
// per-object work Algorithm 1 defines: the object number and generation extend
// the file key, and the crypt filter named by the stream or the encryption
// dictionary selects RC4 or AES.
const (
	handlerStandard = "Standard"

	// cryptFilterVersion is the /V at which a handler gains the /CF crypt
	// filter table. Below it every string and stream uses the legacy RC4 key.
	cryptFilterVersion = 4

	// cryptVersionAES256 is the /V at which the file key is 32 bytes and is
	// used directly, without the per-object extension of Algorithm 1.
	cryptVersionAES256 = 5

	// objectKeyExtraBytes is the number of bytes Algorithm 1 appends to the
	// file key before truncating: the five bytes of object and generation.
	objectKeyExtraBytes = 5

	// objectKeyShift2 is the third and fourth byte's shift, two bytes in.
	objectKeyShift2 = 2

	keyO     = "O"
	keyU     = "U"
	keyOE    = "OE"
	keyUE    = "UE"
	keyPerms = "Perms"
	keyV     = "V"
	keyR     = "R"
	keyCF    = "CF"
	keyCFM   = "CFM"
	keyStmF  = "StmF"
	keyStrF  = "StrF"
	keyEFF   = "EFF"
	keyName  = "Name"

	keyAuthEvent       = "AuthEvent"
	keyEncryptMetadata = "EncryptMetadata"
	keyMetadata        = "Metadata"

	filterCrypt    = "Crypt"
	filterIdentity = "Identity"
	filterV2       = "V2"
	filterAESV2    = "AESV2"
	filterAESV3    = "AESV3"

	// passwordPad is the 32-byte padding string of Algorithm 2 step (a).
	passwordPad = "\x28\xbf\x4e\x5e\x4e\x75\x8a\x41\x64\x00\x4e\x56\xff\xfa\x01\x08" +
		"\x2e\x2e\x00\xb6\xd0\x68\x3e\x80\x2f\x0c\xa9\xfe\x64\x53\x69\x7a"

	passwordPadLen    = 32
	legacyKeyRounds   = 50
	legacyKeyMinBytes = 5
	legacyKeyMaxBytes = 16
	legacyKeyBits     = 40

	revisionR2 = 2
	revisionR3 = 3
	revisionR5 = 5
	revisionR6 = 6

	userCheckRounds = 20
	ownerRounds     = 20
	xorRounds       = 20

	aesBlockSize    = 16
	aesKey128       = 16
	aesKey256       = 32
	aes256EntryLen  = 48
	aes256SaltAt    = 32
	aes256KeySaltAt = 40
	aes256HashLen   = 32

	hash2BRepeats = 64
	hash2BRounds  = 64
	hash2BSlack   = 32
	hash2BMod     = 3
	hash2BMod256  = 0
	hash2BMod384  = 1
	hash2BMod512  = 2
	byteBase      = 256

	md5Len = 16

	// noObjectNum marks a security handler written inline in the trailer. No
	// indirect object number is negative, so the skip never matches one.
	noObjectNum = -1
)

// cryptState is the derived key and crypt filter selection of one encrypted
// document. A nil *cryptState means the document is not encrypted.
type cryptState struct {
	version         int
	revision        int
	key             []byte
	encryptMetadata bool
	stmFilter       string
	strFilter       string
	filters         map[string]string
	encryptNum      int
}

// openCrypt derives the crypt state for a trailer that names an /Encrypt
// handler. A handler this reader cannot open keeps the Encrypt refusal, so a
// file with a real password, a public-key handler, or damaged key material
// reports invalidaccess in Encrypt rather than reading garbage.
//
// A trailer with no /Encrypt is not an error and not an encrypted file: it is
// an ordinary document, reported as errNoEncrypt so the caller can tell that
// apart from a handler this reader cannot open.
// errNoEncrypt reports a trailer with no /Encrypt entry. It is not a failure:
// the document is simply not encrypted.
var errNoEncrypt = errors.New("no /Encrypt entry")

func (file *File) openCrypt(trailer Value) (*cryptState, error) {
	entry, ok := trailer.ValueEntry(keyEncrypt)
	if !ok || entry.Kind == KindNull {
		return nil, errNoEncrypt
	}
	handler, num, err := file.encryptHandler(entry)
	if err != nil {
		return nil, err
	}
	return newCryptState(handler, file.idEntry(trailer), num)
}

// encryptHandler returns the security dictionary and the object number that
// carries it, or noObjectNum for an inline dictionary. The handler is read
// before the crypt state is installed, so its own /O and /U are never
// decrypted.
func (file *File) encryptHandler(entry Value) (Value, int, error) {
	switch entry.Kind {
	case KindRef:
		value, err := file.resolve(entry.RefNum)
		if err != nil {
			return NullVal(), 0, NewError(opEncrypt, errAccess)
		}
		return value, entry.RefNum, nil
	case KindDict:
		return entry, noObjectNum, nil
	case KindNull, KindBool, KindInt, KindReal, KindName, KindString,
		KindArray, KindStream:
		return NullVal(), 0, NewError(opEncrypt, errAccess)
	default:
		return NullVal(), 0, NewError(opEncrypt, errAccess)
	}
}

// idEntry returns the first /ID string of the trailer, or an empty slice. A
// file with no /ID, an empty /ID, and an /ID whose first entry is not a string
// all derive the legacy key from the empty string, which is what the batch2
// files without an ID need.
func (file *File) idEntry(trailer Value) []byte {
	items, ok := trailer.ArrayEntry(keyID)
	if !ok || len(items) == 0 || items[0].Kind != KindString {
		return nil
	}
	return []byte(items[0].String)
}

// newCryptState builds the crypt state for one standard security handler. The
// empty user password is tried first and the empty owner password second, the
// same order a reader that recovered the key at a password prompt would use.
// Any missing or malformed entry the algorithms need is invalidaccess in
// Encrypt.
func newCryptState(handler Value, id0 []byte, encryptNum int) (*cryptState, error) {
	refuse := NewError(opEncrypt, errAccess)
	if handler.Kind != KindDict {
		return nil, refuse
	}
	if name, ok := handler.NameEntry(keyFilter); ok && name != handlerStandard {
		return nil, refuse
	}
	version, ok := handler.IntEntry(keyV)
	if !ok {
		return nil, refuse
	}
	revision, ok := handler.IntEntry(keyR)
	if !ok || revision < revisionR2 || revision > revisionR6 {
		return nil, refuse
	}
	state := &cryptState{
		version:         version,
		revision:        revision,
		key:             nil,
		encryptMetadata: encryptMetadataOf(handler),
		stmFilter:       filterIdentity,
		strFilter:       filterIdentity,
		filters:         map[string]string{},
		encryptNum:      encryptNum,
	}
	if err := state.readFilters(handler); err != nil {
		return nil, err
	}
	key, err := state.deriveKey(handler, id0)
	if err != nil {
		return nil, err
	}
	state.key = key
	return state, nil
}

// encryptMetadataOf reads /EncryptMetadata. The default is true, so an absent
// entry keeps metadata encrypted.
func encryptMetadataOf(handler Value) bool {
	entry, ok := handler.ValueEntry(keyEncryptMetadata)
	if !ok || entry.Kind != KindBool {
		return true
	}
	return entry.Bool
}

// readFilters builds the /CF table and the default stream and string filters.
// A handler below version 4 has no crypt filters: every string and stream uses
// the legacy RC4 key. A named filter the /CF table does not carry is refused,
// because reading it as Identity would emit ciphertext.
func (state *cryptState) readFilters(handler Value) error {
	if state.version < cryptFilterVersion {
		state.stmFilter = filterV2
		state.strFilter = filterV2
		return nil
	}
	if cf, ok := handler.ValueEntry(keyCF); ok && cf.Kind == KindDict {
		for name, entry := range cf.Dict {
			method, err := cryptFilterMethod(entry)
			if err != nil {
				return err
			}
			state.filters[name] = method
		}
	}
	stm, err := state.defaultFilter(handler, keyStmF)
	if err != nil {
		return err
	}
	str, err := state.defaultFilter(handler, keyStrF)
	if err != nil {
		return err
	}
	state.stmFilter = stm
	state.strFilter = str
	return nil
}

// defaultFilter resolves /StmF or /StrF. An absent entry, and the Identity
// name, mean no decryption; any other name must be in the /CF table.
func (state *cryptState) defaultFilter(handler Value, key string) (string, error) {
	name, ok := handler.NameEntry(key)
	if !ok || name == filterIdentity {
		return filterIdentity, nil
	}
	method, ok := state.filters[name]
	if !ok {
		return "", NewError(opEncrypt, errAccess)
	}
	return method, nil
}

// cryptFilterMethod maps one /CF dictionary to the cipher its /CFM names. An
// absent /CFM is None, which decrypts nothing.
func cryptFilterMethod(entry Value) (string, error) {
	if entry.Kind != KindDict {
		return "", NewError(opEncrypt, errAccess)
	}
	cfm, ok := entry.NameEntry(keyCFM)
	if !ok {
		return filterIdentity, nil
	}
	switch cfm {
	case filterIdentity, "None":
		return filterIdentity, nil
	case filterV2:
		return filterV2, nil
	case filterAESV2:
		return filterAESV2, nil
	case filterAESV3:
		return filterAESV3, nil
	default:
		return "", NewError(opEncrypt, errAccess)
	}
}

// deriveKey returns the file encryption key the empty password recovers. A
// password that does not authenticate the handler is invalidaccess in Encrypt.
func (state *cryptState) deriveKey(handler Value, id0 []byte) ([]byte, error) {
	if state.revision >= revisionR5 {
		return state.deriveAES256Key(handler)
	}
	owner, ok := handlerByteEntry(handler, keyO)
	if !ok || len(owner) < passwordPadLen {
		return nil, NewError(opEncrypt, errAccess)
	}
	user, ok := handlerByteEntry(handler, keyU)
	if !ok || len(user) < passwordPadLen {
		return nil, NewError(opEncrypt, errAccess)
	}
	perm, _ := handler.IntEntry(keyP)
	keyLen := state.legacyKeyLen(handler)
	key, ok := authenticateLegacy(owner[:passwordPadLen], user, id0, int64(perm), state.revision,
		keyLen, state.encryptMetadata)
	if !ok {
		return nil, NewError(opEncrypt, errAccess)
	}
	return key, nil
}

// legacyKeyLen returns the file key length in bytes for revisions 2 to 4. The
// /Length entry is in bits; an absent or implausible value falls back to the
// 40-bit default and the 5-to-16 byte range the algorithms allow.
func (state *cryptState) legacyKeyLen(handler Value) int {
	if state.revision == revisionR2 {
		return legacyKeyMinBytes
	}
	bits, ok := handler.IntEntry(keyLength)
	if !ok || bits <= 0 {
		bits = legacyKeyBits
	}
	length := bits / bitsPerByte
	if length < legacyKeyMinBytes {
		length = legacyKeyMinBytes
	}
	if length > legacyKeyMaxBytes {
		length = legacyKeyMaxBytes
	}
	return length
}

// handlerByteEntry returns one byte-string entry of the security dictionary.
func handlerByteEntry(handler Value, key string) ([]byte, bool) {
	entry, ok := handler.ValueEntry(key)
	if !ok || entry.Kind != KindString {
		return nil, false
	}
	return []byte(entry.String), true
}

// authenticateLegacy runs the empty password through the user path and then
// the owner path. The returned key is the file key of whichever path
// authenticated.
func authenticateLegacy(
	owner, user, id0 []byte,
	perm int64,
	revision, keyLen int,
	encryptMetadata bool,
) ([]byte, bool) {
	key := deriveLegacyKey(nil, owner, id0, perm, revision, keyLen, encryptMetadata)
	if userKeyMatches(key, user, id0, revision) {
		return key, true
	}
	ownerUser := ownerUserPassword(owner, keyLen, revision)
	key = deriveLegacyKey(ownerUser, owner, id0, perm, revision, keyLen, encryptMetadata)
	if userKeyMatches(key, user, id0, revision) {
		return key, true
	}
	return nil, false
}

// deriveLegacyKey is Algorithm 2 steps (a) to (i): the padded password, the
// /O entry, /P little-endian, the first /ID string, the metadata marker for
// revision 4, and the 50-round MD5 loop for revision 3 and later.
func deriveLegacyKey(password, owner, id0 []byte, perm int64, revision, keyLen int, encryptMetadata bool) []byte {
	hash := md5.New() //nolint:gosec // ISO 32000-1 Algorithm 2 names MD5.
	hash.Write(padPassword(password))
	hash.Write(owner)
	var permBytes [4]byte
	// The /P permissions word is a 32-bit signed value in the file; the
	// algorithm hashes its low four bytes little-endian.
	binary.LittleEndian.PutUint32(permBytes[:], uint32(perm)) //nolint:gosec // ISO 32000-1 Algorithm 2 step (d).
	hash.Write(permBytes[:])
	hash.Write(id0)
	if revision >= 4 && !encryptMetadata {
		hash.Write([]byte{0xff, 0xff, 0xff, 0xff})
	}
	digest := hash.Sum(nil)
	for round := 0; round < legacyKeyRounds && revision >= revisionR3; round++ {
		sum := md5.Sum(digest[:keyLen]) //nolint:gosec // ISO 32000-1 Algorithm 2.

		digest = sum[:]
	}
	return digest[:keyLen]
}

// userKeyMatches is Algorithm 6: the computed /U value, compared on the whole
// entry for revision 2 and the first 16 bytes for revision 3 and later.
func userKeyMatches(key, user, id0 []byte, revision int) bool {
	if revision == revisionR2 {
		return bytes.Equal(rc4Apply(key, []byte(passwordPad)), user[:passwordPadLen])
	}
	digest := md5.Sum(append([]byte(passwordPad), id0...)) //nolint:gosec // ISO 32000-1 step (a).

	check := rc4Apply(key, digest[:])
	for round := 1; round < userCheckRounds; round++ {
		check = rc4Apply(xorKey(key, byte(round)), check)
	}
	return bytes.Equal(check[:md5Len], user[:md5Len])
}

// ownerUserPassword is Algorithm 3.7 steps (a) to (e): recover the padded user
// password from /O with the empty owner password. Revision 3 and later use 20
// RC4 rounds with a key byte XORed by the round number, counted down.
func ownerUserPassword(owner []byte, keyLen, revision int) []byte {
	digest := md5.Sum(padPassword(nil)) //nolint:gosec // ISO 32000-1 names MD5.
	key := digest[:]
	for round := 0; round < legacyKeyRounds && revision >= revisionR3; round++ {
		sum := md5.Sum(key[:keyLen]) //nolint:gosec // ISO 32000-1 names MD5.
		key = sum[:]
	}
	key = key[:keyLen]
	if revision == revisionR2 {
		return rc4Apply(key, owner)
	}
	user := owner
	for round := xorRounds - 1; round >= 0; round-- {
		user = rc4Apply(xorKey(key, byte(round)), user)
	}
	return user
}

// padPassword pads or truncates a password to the 32 bytes Algorithm 2 step
// (a) requires.
func padPassword(password []byte) []byte {
	padded := make([]byte, 0, passwordPadLen)
	padded = append(padded, password...)
	if len(padded) > passwordPadLen {
		return padded[:passwordPadLen]
	}
	return append(padded, passwordPad[len(padded):]...)
}

// deriveAES256Key recovers the file key for revisions 5 and 6. The user path
// checks the password against the /U validation salt and decrypts /UE; the
// owner path checks /O and decrypts /OE. An entry longer than 48 bytes keeps
// its first 48, the shape the batch2 over-long handlers carry.
func (state *cryptState) deriveAES256Key(handler Value) ([]byte, error) {
	owner, ok := handlerByteEntry(handler, keyO)
	if !ok || len(owner) < aes256EntryLen {
		return nil, NewError(opEncrypt, errAccess)
	}
	user, ok := handlerByteEntry(handler, keyU)
	if !ok || len(user) < aes256EntryLen {
		return nil, NewError(opEncrypt, errAccess)
	}
	owner = owner[:aes256EntryLen]
	user = user[:aes256EntryLen]
	if key, ok := state.emptyUserKey(handler, user); ok {
		return key, nil
	}
	if key, ok := state.emptyOwnerKey(handler, owner, user); ok {
		return key, nil
	}
	return nil, NewError(opEncrypt, errAccess)
}

// emptyUserKey validates the user password and unwraps the file key with /UE.
// It reports false when the empty user password does not authenticate, which is
// not an error: the caller then tries the owner path.
func (state *cryptState) emptyUserKey(handler Value, user []byte) ([]byte, bool) {
	check := state.aes256Hash(user[aes256SaltAt:aes256KeySaltAt], nil)
	if !bytes.Equal(check, user[:aes256HashLen]) {
		return nil, false
	}
	wrapped, ok := handlerByteEntry(handler, keyUE)
	if !ok {
		return nil, false
	}
	return aesCBCDecryptNoPad(state.aes256Hash(user[aes256KeySaltAt:aes256EntryLen], nil), wrapped)
}

// emptyOwnerKey validates the owner password and unwraps the file key with /OE.
// The owner hash takes the user entry as its extra input, so it is passed in.
func (state *cryptState) emptyOwnerKey(handler Value, owner, user []byte) ([]byte, bool) {
	check := state.aes256Hash(owner[aes256SaltAt:aes256KeySaltAt], user)
	if !bytes.Equal(check, owner[:aes256HashLen]) {
		return nil, false
	}
	wrapped, ok := handlerByteEntry(handler, keyOE)
	if !ok {
		return nil, false
	}
	return aesCBCDecryptNoPad(state.aes256Hash(owner[aes256KeySaltAt:aes256EntryLen], user), wrapped)
}

// aes256Hash is Algorithm 2.A step (a) for revision 5 and Algorithm 2.B for
// revision 6. The password is the empty user password, which is what these
// files carry, so it contributes its padding only and is passed as nil.
func (state *cryptState) aes256Hash(salt, udata []byte) []byte {
	if state.revision < revisionR6 {
		sum := sha256.Sum256(concatBytes(nil, salt, udata))
		return sum[:]
	}
	return hashR6(nil, salt, udata)
}

// hashR6 is Algorithm 2.B. The AES-128-CBC round hashes K1, the 64-fold
// repetition of the password, the running key, and the user data, and the
// first 16 bytes of the ciphertext pick the next hash. The loop ends only
// after 64 rounds and once the last ciphertext byte is at most round-32.
func hashR6(password, salt, udata []byte) []byte {
	seed := sha256.Sum256(concatBytes(password, salt, udata))
	key := seed[:]
	for round := 1; ; round++ {
		k1 := repeatBytes(concatBytes(password, key, udata), hash2BRepeats)
		encrypted := aesCBCEncryptNoPad(key[:aesKey128], key[aesKey128:aesKey256], k1)
		if len(encrypted) == 0 {
			return nil
		}
		switch first16Mod3(encrypted) {
		case hash2BMod256:
			sum := sha256.Sum256(encrypted)
			key = sum[:]
		case hash2BMod384:
			sum := sha512.Sum384(encrypted)
			key = sum[:]
		case hash2BMod512:
			sum := sha512.Sum512(encrypted)
			key = sum[:]
		}
		if round >= hash2BRounds && int(encrypted[len(encrypted)-1]) <= round-hash2BSlack {
			break
		}
	}
	return key[:aes256HashLen]
}

// first16Mod3 treats the first 16 bytes as a big-endian integer and returns it
// modulo 3, the running form Algorithm 2.B step (e) defines.
func first16Mod3(data []byte) int {
	value := 0
	for index := 0; index < md5Len && index < len(data); index++ {
		value = (value*byteBase + int(data[index])) % hash2BMod
	}
	return value
}

// concatBytes joins slices without mutating any of them.
func concatBytes(parts ...[]byte) []byte {
	size := 0
	for _, part := range parts {
		size += len(part)
	}
	joined := make([]byte, 0, size)
	for _, part := range parts {
		joined = append(joined, part...)
	}
	return joined
}

// repeatBytes returns data repeated count times.
func repeatBytes(data []byte, count int) []byte {
	repeated := make([]byte, 0, len(data)*count)
	for range count {
		repeated = append(repeated, data...)
	}
	return repeated
}

// xorKey returns key with every byte XORed by value.
func xorKey(key []byte, value byte) []byte {
	flipped := make([]byte, len(key))
	for index, cur := range key {
		flipped[index] = cur ^ value
	}
	return flipped
}

// objectKey is Algorithm 1 steps (b) to (d): the file key extended with the
// object number and generation, the AES salt, and truncated to 16 bytes. A
// version 5 handler uses the 32-byte file key directly, the Algorithm 3.1a
// path qpdf and Ghostscript take, so a non-conforming AESV2 or V2 crypt filter
// under version 5 still reads.
func (state *cryptState) objectKey(num, gen int, aes bool) []byte {
	if state.version >= cryptVersionAES256 {
		return state.key
	}
	hash := md5.New() //nolint:gosec // ISO 32000-1 names MD5.
	hash.Write(state.key)
	hash.Write([]byte{
		byte(num), byte(num >> bitsPerByte), byte(num >> (objectKeyShift2 * bitsPerByte)),
		byte(gen), byte(gen >> bitsPerByte),
	})
	if aes {
		hash.Write([]byte("sAlT"))
	}
	digest := hash.Sum(nil)
	length := len(state.key) + objectKeyExtraBytes
	if length > md5Len {
		length = md5Len
	}
	return digest[:length]
}

// decryptBytes decrypts one string or stream body with the named crypt filter.
// The Identity filter and an empty name leave the bytes alone.
func (state *cryptState) decryptBytes(method string, num, gen int, data []byte) []byte {
	switch method {
	case filterV2:
		return rc4Apply(state.objectKey(num, gen, false), data)
	case filterAESV2:
		decrypted := aesCBCDecrypt(state.objectKey(num, gen, true), data)
		return decrypted
	case filterAESV3:
		decrypted := aesCBCDecrypt(state.key, data)
		return decrypted
	default:
		return data
	}
}

// rc4Apply runs the RC4 stream cipher. ISO 32000-1 names RC4; the Go standard
// library marks its implementation deprecated, so this file carries the 30
// lines the format needs.
func rc4Apply(key, data []byte) []byte {
	if len(key) == 0 {
		return data
	}
	var perm [byteBase]byte
	for index := range perm {
		perm[index] = byte(index)
	}
	swap := 0
	for index := range perm {
		swap = (swap + int(perm[index]) + int(key[index%len(key)])) & (byteBase - 1)
		perm[index], perm[swap] = perm[swap], perm[index]
	}
	out := make([]byte, len(data))
	low := 0
	high := 0
	for index, cur := range data {
		low = (low + 1) & (byteBase - 1)
		high = (high + int(perm[low])) & (byteBase - 1)
		perm[low], perm[high] = perm[high], perm[low]
		out[index] = cur ^ perm[(int(perm[low])+int(perm[high]))&(byteBase-1)]
	}
	return out
}

// aesCBCDecrypt decrypts an AES-CBC body whose first 16 bytes are the
// initialization vector, then strips the PKCS#7 padding. A body too short, a
// wrong block size, and padding that does not check all return nil; the caller
// keeps the refusal for a key that cannot produce the body.
func aesCBCDecrypt(key, data []byte) []byte {
	if len(data) < 2*aesBlockSize || (len(data)-aesBlockSize)%aesBlockSize != 0 {
		return nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	out := make([]byte, len(data)-aesBlockSize)
	cipher.NewCBCDecrypter(block, data[:aesBlockSize]).CryptBlocks(out, data[aesBlockSize:])
	return stripPKCS7(out)
}

// aesCBCEncryptNoPad encrypts one block-aligned buffer for Algorithm 2.B. The
// key and buffer sizes there are always valid, and an impossible one returns
// false.
func aesCBCEncryptNoPad(key, vector, data []byte) []byte {
	if len(data) == 0 || len(data)%aesBlockSize != 0 {
		return nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, vector).CryptBlocks(out, data)
	return out
}

// aesCBCDecryptNoPad decrypts /UE or /OE, which carry a whole number of blocks
// and no padding, with an all-zero initialization vector.
func aesCBCDecryptNoPad(key, data []byte) ([]byte, bool) {
	if len(data) == 0 || len(data)%aesBlockSize != 0 {
		return nil, false
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, false
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, make([]byte, aesBlockSize)).CryptBlocks(out, data)
	return out, true
}

// stripPKCS7 removes the PKCS#7 padding AES producers append. A final byte
// outside 1 to 16, or padding bytes that disagree with it, leave the buffer
// alone: the password already authenticated, so a bad pad means a damaged
// body, not a wrong key.
func stripPKCS7(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	pad := int(data[len(data)-1])
	if pad < 1 || pad > aesBlockSize || pad > len(data) {
		return data
	}
	for _, cur := range data[len(data)-pad:] {
		if int(cur) != pad {
			return data
		}
	}
	return data[:len(data)-pad]
}
