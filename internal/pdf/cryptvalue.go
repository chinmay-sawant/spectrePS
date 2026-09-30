package pdf

// Decryption is applied where an object is read, not where it is used. A
// resolved object arrives here once, and every string and stream it carries is
// plaintext afterwards, so the content reader, the font reader, and the image
// reader need no changes. ISO 32000-1 clause 7.6.2 lists the exceptions this
// file keeps: the encryption dictionary itself, cross-reference streams, and
// metadata when /EncryptMetadata is false. A stream that names a /Crypt filter
// keeps that filter's own answer even when it is metadata.

// decryptValue returns val with every direct string and stream decrypted for
// the object numbered num with generation gen. An indirect reference is left
// alone: the object it names is decrypted when it is resolved. A nil crypt
// state, and the security handler's own object, come back unchanged.
func (file *File) decryptValue(num, gen int, val Value) Value {
	if file.crypt == nil {
		return val
	}
	return file.crypt.decryptValue(num, gen, val)
}

// decryptValue walks one value tree. Strings use /StrF, streams use /StmF or
// their own /Crypt filter, and dictionaries and arrays keep their shape.
func (state *cryptState) decryptValue(num, gen int, val Value) Value {
	if num == state.encryptNum {
		return val
	}
	switch val.Kind {
	case KindString:
		return StringVal(string(state.decryptBytes(state.strFilter, num, gen, []byte(val.String))))
	case KindArray:
		items := make([]Value, len(val.Array))
		for index, item := range val.Array {
			items[index] = state.decryptValue(num, gen, item)
		}
		return ArrayVal(items)
	case KindDict:
		return DictVal(state.decryptDict(num, gen, val.Dict))
	case KindStream:
		return state.decryptStream(num, gen, val)
	case KindNull, KindBool, KindInt, KindReal, KindName, KindRef:
		return val
	default:
		return val
	}
}

// decryptDict copies a dictionary with every direct value decrypted.
func (state *cryptState) decryptDict(num, gen int, dict map[string]Value) map[string]Value {
	entries := make(map[string]Value, len(dict))
	for key, item := range dict {
		entries[key] = state.decryptValue(num, gen, item)
	}
	return entries
}

// decryptStream returns a stream whose dictionary strings and body are
// decrypted. A /Crypt filter is removed from the decode chain after it selects
// the cipher, because the filter itself is not a decoder.
func (state *cryptState) decryptStream(num, gen int, val Value) Value {
	dict := state.decryptDict(num, gen, val.Dict)
	cryptAt := cryptFilterIndex(dict)
	if cryptAt >= 0 {
		dict = withoutCryptFilter(dict, cryptAt)
	}
	method := state.streamMethod(val, cryptAt)
	if plainStream(val, cryptAt, state.encryptMetadata) {
		method = filterIdentity
	}
	return StreamVal(dict, state.decryptBytes(method, num, gen, val.Stream))
}

// cryptFilterIndex returns the position of the /Crypt filter in a stream's
// /Filter chain, or -1. ISO 32000-1 clause 7.4.10 puts the Crypt filter first
// in the chain, which is the only position where decryption precedes the
// decoders, so only that position is honoured.
func cryptFilterIndex(dict map[string]Value) int {
	filter, ok := dict[keyFilter]
	if !ok {
		return -1
	}
	if filter.Kind == KindName && filter.Name == filterCrypt {
		return 0
	}
	if filter.Kind != KindArray || len(filter.Array) == 0 {
		return -1
	}
	first := filter.Array[0]
	if first.Kind == KindName && first.Name == filterCrypt {
		return 0
	}
	return -1
}

// withoutCryptFilter returns a copy of the dictionary whose /Filter chain and
// matching /DecodeParms entry no longer carry the Crypt filter at index.
func withoutCryptFilter(dict map[string]Value, index int) map[string]Value {
	copied := make(map[string]Value, len(dict))
	for key, item := range dict {
		copied[key] = item
	}
	filter := copied[keyFilter]
	if filter.Kind == KindName || len(filter.Array) <= 1 {
		delete(copied, keyFilter)
		delete(copied, keyParms)
		return copied
	}
	items := make([]Value, 0, len(filter.Array)-1)
	items = append(items, filter.Array[:index]...)
	items = append(items, filter.Array[index+1:]...)
	copied[keyFilter] = ArrayVal(items)
	if parms, ok := copied[keyParms]; ok {
		copied[keyParms] = dropParms(parms, index)
	}
	return copied
}

// dropParms removes the /DecodeParms element that belonged to the removed
// filter. A single dictionary belongs to a single filter, so it is dropped
// with the filter at index zero.
func dropParms(parms Value, index int) Value {
	if parms.Kind == KindArray && index < len(parms.Array) {
		items := make([]Value, 0, len(parms.Array)-1)
		items = append(items, parms.Array[:index]...)
		items = append(items, parms.Array[index+1:]...)
		return ArrayVal(items)
	}
	if parms.Kind == KindDict && index == 0 {
		return NullVal()
	}
	return parms
}

// streamMethod returns the cipher for a stream body. Without a /Crypt filter
// that is /StmF. With one, the filter's /Name selects a /CF entry, and an
// absent /Name means the default stream filter.
func (state *cryptState) streamMethod(val Value, cryptAt int) string {
	if cryptAt < 0 {
		return state.stmFilter
	}
	name, ok := parmsName(val, cryptAt)
	if !ok || name == filterIdentity {
		return state.stmFilter
	}
	if method, found := state.filters[name]; found {
		return method
	}
	return filterIdentity
}

// parmsName returns the /Name of the /DecodeParms entry for one filter.
func parmsName(val Value, index int) (string, bool) {
	parms, ok := val.ValueEntry(keyParms)
	if !ok {
		return "", false
	}
	if parms.Kind == KindArray {
		if index >= len(parms.Array) {
			return "", false
		}
		parms = parms.Array[index]
	}
	if parms.Kind != KindDict {
		return "", false
	}
	name, ok := parms.NameEntry(keyName)
	return name, ok
}

// plainStream reports whether a stream is exempt from the default stream
// filter. A cross-reference stream is never encrypted, and a metadata stream
// is exempt when /EncryptMetadata is false and the stream did not name a crypt
// filter itself.
func plainStream(val Value, cryptAt int, encryptMetadata bool) bool {
	typeName, _ := val.NameEntry(keyType)
	if typeName == keyXRef {
		return true
	}
	return typeName == keyMetadata && !encryptMetadata && cryptAt < 0
}
