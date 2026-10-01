//go:build ignore

// Command gen restores the validation corpus from its pinned sources. Run it
// from anywhere in the module:
//
//	go run internal/validation/gen.go                    # the committed tier
//	go run internal/validation/gen.go -fetch-external    # the fetched tier
//	go run internal/validation/gen.go -fetch-external -fetch-bulk   # and the 4.5 GB bulk tier
//	go run internal/validation/gen.go -fetch-external -verify-only
//	go run internal/validation/gen.go -fetch-external -dry-run -area archival
//
// # Behavior
//
// The manifest at sampledata/validation/manifest.tsv names a pinned URL and a
// SHA-256 for every fetched row. Every URL carries an immutable commit, a
// dated release, or a tag with a digest, so a URL that rots can be changed
// without changing the hash. Repo-authored rows are skipped, because their
// bytes already live in the tree and the manifest test checks their digests.
// External rows are skipped unless -fetch-external is set.
//
// The command writes no timestamp: running it twice leaves the working tree
// unchanged.
//
// # The fetched tier
//
// The external tier lands under sampledata/validation/external, which is
// gitignored. It is fetched into a content-addressed cache first and hardlinked
// into the tree from there, so a killed run never leaves a half-written file
// that looks complete, a second run costs no network, and the tree costs no
// extra bytes.
//
// Cache root defaults to os.UserCacheDir()/spectreps/validation and is
// overridable with -cache or SPECTREPS_VALIDATION_CACHE, so a CI cache step can
// point it at a restored directory.
//
// # The bulk tier
//
// bulk.tsv pins whole tarballs whose files are too many to list as manifest
// rows. -fetch-bulk downloads each archive into the cache, checks the
// archive's SHA-512 and byte count, extracts it to a staging directory, and
// checks every member against bulk/<name>.members.tsv before moving a single
// file into the tree. The batch2 archive is 4.5 GB and 5,613 files, so the
// tier is always opt-in: a run without -fetch-bulk prints the cost and changes
// nothing.
//
// # Politeness
//
// Per-host concurrency and rate limits live in hostPolicy below, not in a flag,
// so the numbers are reviewable in a diff. Every request carries a descriptive
// User-Agent naming the tool and a contact, which sec.gov and Wikimedia both
// require and both enforce with a 403.
package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chinmay-sawant/spectrePS/internal/validation"
)

// userAgent identifies the fetcher to a host operator.
const userAgent = "spectreps-validation (https://github.com/chinmay-sawant/spectrePS)"

// A host's fetch budget. concurrent bounds in-flight requests to the host,
// perSecond bounds the request rate, and minSpacing enforces a floor between
// two requests even when concurrency would allow more.
type budget struct {
	concurrent int
	perSecond  float64
	minSpacing time.Duration
}

// hostPolicy is the politeness table. Every value is a published limit or a
// deliberately conservative default, and a new rate-limited host gets an entry
// here rather than a flag.
var hostPolicy = map[string]budget{
	"raw.githubusercontent.com": {concurrent: 8, perSecond: 20},
	// sec.gov publishes "no more than 10 requests per second, regardless of
	// the number of machines". The ceiling is shared behind a NAT, so aim under.
	"www.sec.gov": {concurrent: 1, perSecond: 8},
	// archive.org asks for 4 concurrent with a 1 second delay.
	"archive.org": {concurrent: 4, perSecond: 4, minSpacing: time.Second},
}

// defaultBudget is the fallback for a host with no entry.
var defaultBudget = budget{concurrent: 4, perSecond: 5}

const (
	// maxAttempts bounds one row's retries.
	maxAttempts = 5
	// retryBase and retryCap bound the exponential backoff.
	retryBase = 500 * time.Millisecond
	retryCap  = 30 * time.Second
	// cacheShard is the hex prefix depth of a cache directory.
	cacheShard = 4
)

func main() {
	fetchExternal := flag.Bool("fetch-external", false,
		"also fetch the external tier under sampledata/validation/external")
	fetchBulk := flag.Bool("fetch-bulk", false,
		"also fetch the tarball tier in bulk.tsv; batch2 is a 4.5 GB download")
	verifyOnly := flag.Bool("verify-only", false,
		"check the cache against the manifest and touch no network")
	dryRun := flag.Bool("dry-run", false,
		"print the plan and the per-host budget, fetch nothing")
	area := flag.String("area", "", "restrict to one manifest area")
	workers := flag.Int("workers", 0,
		"bound the whole run; the per-host budget still applies and can only be lowered")
	cacheDir := flag.String("cache", "", "override the content-addressed cache root")
	flag.Parse()

	rows, err := validation.Load()
	if err != nil {
		fail("load manifest: %v", err)
	}
	selected := selectRows(rows, *fetchExternal, *area)
	if len(selected) == 0 {
		fmt.Println("gen: no rows selected")
	}
	bulk, err := loadBulk(validation.BulkManifestName)
	if err != nil {
		fail("load bulk manifest: %v", err)
	}

	cache := cacheRoot(*cacheDir)
	if *dryRun {
		if len(selected) > 0 {
			printPlan(selected, cache)
		}
		printBulkPlan(bulk, *fetchBulk)
		return
	}

	client := newClient()
	if len(selected) > 0 {
		if err := resolveAll(client, selected, cache, *verifyOnly, *workers); err != nil {
			fail("%v", err)
		}

		// Every row resolved before this point. Only now touch the tree, so a
		// failure above leaves the working tree as it was.
		dir := validation.CorpusDir()
		for _, row := range selected {
			blob := filepath.Join(cache, blobName(row.SHA256))
			dest := filepath.Join(dir, filepath.FromSlash(row.Path))
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				fail("mkdir %s: %v", filepath.Dir(dest), err)
			}
			if err := place(blob, dest); err != nil {
				fail("place %s: %v", row.Path, err)
			}
		}
		fmt.Printf("gen: %d rows, all present and verified\n", len(selected))
	}

	if err := runBulk(newBulkClient(), bulk, *fetchBulk, *verifyOnly, cache); err != nil {
		fail("%v", err)
	}
}

// selectRows returns the rows this run acts on: the external tier only with
// -fetch-external, one area only with -area, and never a repo-authored row,
// because its bytes already live in the tree and the manifest test checks them.
func selectRows(rows []validation.Row, external bool, area string) []validation.Row {
	selected := make([]validation.Row, 0, len(rows))
	for _, row := range rows {
		if row.Source == validation.LicenseRepoAuthored {
			continue
		}
		if row.External() && !external {
			continue
		}
		if area != "" && row.Area != area {
			continue
		}
		selected = append(selected, row)
	}
	return selected
}

// printPlan reports the work and the politeness budget without fetching.
func printPlan(rows []validation.Row, cache string) {
	byHost := map[string]int{}
	var total int64
	for _, row := range rows {
		byHost[hostOf(row.Source)]++
		total += row.Bytes
	}
	fmt.Printf("gen: %d rows, %d bytes (%.1f MiB) into %s\n",
		len(rows), total, float64(total)/(1<<20), cache)
	hosts := make([]string, 0, len(byHost))
	for host := range byHost {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	for _, host := range hosts {
		policy := policyFor(host)
		fmt.Printf("  %-26s %4d rows  concurrent=%d rate=%.0f/s\n",
			host, byHost[host], policy.concurrent, policy.perSecond)
	}
}

// resolveAll fetches or verifies every row, honouring the per-host budget.
func resolveAll(client *http.Client, rows []validation.Row, cache string, verifyOnly bool, workers int) error {
	// One limiter per host, shared by every row on it, so the rate limit is a
	// real limit and not a fresh budget per file.
	var mu sync.Mutex
	limiters := map[string]*hostLimiter{}
	limiterFor := func(host string) *hostLimiter {
		mu.Lock()
		defer mu.Unlock()
		l, ok := limiters[host]
		if !ok {
			l = newHostLimiter(policyFor(host))
			limiters[host] = l
		}
		return l
	}

	var sem chan struct{}
	if workers > 0 {
		sem = make(chan struct{}, workers)
	}

	results := make([]error, len(rows))
	var wg sync.WaitGroup
	var done int
	var doneMu sync.Mutex

	for i, row := range rows {
		blob := filepath.Join(cache, blobName(row.SHA256))
		// A cache hit costs no network and no rate-limit token.
		if ok, err := verifyBlob(blob, row); err == nil && ok {
			continue
		}
		wg.Add(1)
		go func(i int, row validation.Row, blob string) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			results[i] = resolveOne(client, row, blob, verifyOnly, limiterFor)
			doneMu.Lock()
			done++
			if done%50 == 0 || done == len(rows) {
				log.Printf("gen: %d of %d rows", done, len(rows))
			}
			doneMu.Unlock()
		}(i, row, blob)
	}
	wg.Wait()

	var failed []string
	for i, err := range results {
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", rows[i].Path, err))
		}
	}
	if len(failed) > 0 {
		sort.Strings(failed)
		return fmt.Errorf("%d of %d rows failed:\n  %s",
			len(failed), len(rows), strings.Join(failed, "\n  "))
	}
	return nil
}

// resolveOne fetches one row into the cache and verifies it, or verifies an
// existing cache entry when verifyOnly is set.
func resolveOne(client *http.Client, row validation.Row, blob string, verifyOnly bool, limiterFor func(string) *hostLimiter) error {
	if verifyOnly {
		ok, err := verifyBlob(blob, row)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("absent from the cache and -verify-only forbids the network")
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(blob), 0o755); err != nil {
		return err
	}
	// Write to a temp name in the same directory and rename, so a killed run
	// never leaves a file that looks complete.
	tmp, err := os.CreateTemp(filepath.Dir(blob), ".fetch-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	sum, size, err := fetch(client, row, tmp, limiterFor(hostOf(row.Source)))
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(sum[:]); got != row.SHA256 {
		return fmt.Errorf("sha256 %s, want %s", got, row.SHA256)
	}
	if int64(size) != row.Bytes {
		return fmt.Errorf("%d bytes, want %d", size, row.Bytes)
	}
	return os.Rename(tmpName, blob)
}

// fetch streams one row to w, retrying the failures worth retrying.
func fetch(client *http.Client, row validation.Row, w io.Writer, limiter *hostLimiter) ([32]byte, int64, error) {
	var zero [32]byte
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		release, err := limiter.wait()
		if err != nil {
			return zero, 0, err
		}
		sum, size, retryable, err := fetchOnce(client, row, w)
		release()
		if err == nil {
			return sum, size, nil
		}
		lastErr = err
		if !retryable {
			return zero, 0, err
		}
		if attempt == maxAttempts {
			break
		}
		wait := backoff(attempt)
		log.Printf("gen: %s: %v, retrying in %s", row.Path, err, wait)
		time.Sleep(wait)
	}
	return zero, 0, fmt.Errorf("after %d attempts: %w", maxAttempts, lastErr)
}

// fetchOnce performs one request. It reports whether the failure is worth
// retrying: a transport error, a 408, a 429, or a 5xx is; a 404 is not. A
// digest or size mismatch is not detected here, because the caller compares
// after the stream closes and treats it as fatal.
func fetchOnce(client *http.Client, row validation.Row, w io.Writer) ([32]byte, int64, bool, error) {
	var zero [32]byte
	req, err := http.NewRequest(http.MethodGet, row.Source, nil)
	if err != nil {
		return zero, 0, false, err
	}
	req.Header.Set("User-Agent", userAgent)
	// Accept-Encoding is deliberately NOT set. net/http adds gzip itself and
	// decompresses the response transparently, but only when the header is
	// absent. Setting it by hand makes the transport hand back the compressed
	// bytes, so the digest is taken over the gzip stream and every row whose
	// body is big enough for the server to compress fails with a mismatch that
	// looks like a wrong pin.
	resp, err := client.Do(req)
	if err != nil {
		return zero, 0, true, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusRequestTimeout,
		resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode >= 500:
		return zero, 0, true, fmt.Errorf("get: %s", resp.Status)
	default:
		return zero, 0, false, fmt.Errorf("get: %s", resp.Status)
	}
	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(w, hasher), resp.Body)
	if err != nil {
		return zero, 0, true, fmt.Errorf("read: %w", err)
	}
	var sum [32]byte
	copy(sum[:], hasher.Sum(nil))
	return sum, size, false, nil
}

// backoff returns an exponential delay with full jitter, capped. A 429 is
// treated as retryable, so the next attempt backs off again without reading
// Retry-After.
func backoff(attempt int) time.Duration {
	window := retryBase << (attempt - 1)
	if window > retryCap {
		window = retryCap
	}
	//nolint:gosec // jitter needs no cryptographic source
	return time.Duration(rand.Int63n(int64(window)) + 1)
}

// hostLimiter bounds one host's concurrency and rate. wait hands back a
// release function so the caller cannot forget to return the slot.
type hostLimiter struct {
	slots  chan struct{}
	ticker *time.Ticker
	// mu and last enforce minSpacing across concurrent callers.
	mu   sync.Mutex
	last time.Time
	gap  time.Duration
}

// newHostLimiter builds a limiter for one host's budget.
func newHostLimiter(p budget) *hostLimiter {
	l := &hostLimiter{
		slots: make(chan struct{}, p.concurrent),
		gap:   p.minSpacing,
	}
	if p.perSecond > 0 {
		l.ticker = time.NewTicker(time.Duration(float64(time.Second) / p.perSecond))
	}
	return l
}

// wait takes a concurrency slot, honours the rate, and applies the minimum
// spacing. The returned function returns the slot and must be called.
func (l *hostLimiter) wait() (func(), error) {
	l.slots <- struct{}{}
	if l.ticker != nil {
		<-l.ticker.C
	}
	if l.gap > 0 {
		l.mu.Lock()
		if !l.last.IsZero() {
			if since := time.Since(l.last); since < l.gap {
				time.Sleep(l.gap - since)
			}
		}
		l.last = time.Now()
		l.mu.Unlock()
	}
	return func() { <-l.slots }, nil
}

// verifyBlob reports whether the cache holds exactly the row's bytes.
func verifyBlob(blob string, row validation.Row) (bool, error) {
	f, err := os.Open(blob)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()
	hasher := sha256.New()
	size, err := io.Copy(hasher, f)
	if err != nil {
		return false, err
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); got != row.SHA256 {
		return false, fmt.Errorf("%s: cache sha256 %s, want %s", row.Path, got, row.SHA256)
	}
	if int64(size) != row.Bytes {
		return false, fmt.Errorf("%s: cache %d bytes, want %d", row.Path, size, row.Bytes)
	}
	return true, nil
}

// blobName is the cache file name for a digest, sharded so no directory grows
// without bound.
func blobName(digest string) string {
	if len(digest) < cacheShard {
		return digest
	}
	return filepath.Join(digest[:2], digest[2:cacheShard], digest)
}

// cacheRoot returns the cache directory, creating it.
func cacheRoot(override string) string {
	root := override
	if root == "" {
		root = os.Getenv("SPECTREPS_VALIDATION_CACHE")
	}
	if root == "" {
		user, err := os.UserCacheDir()
		if err != nil {
			fail("user cache dir: %v", err)
		}
		root = filepath.Join(user, "spectreps", "validation")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		fail("cache root %s: %v", root, err)
	}
	return root
}

// place hardlinks a cached blob into the corpus, falling back to a copy across
// a filesystem boundary. A hardlink means the tree costs no extra bytes and a
// cache eviction cannot corrupt the corpus.
func place(blob, dest string) error {
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Link(blob, dest); err == nil {
		return nil
	}
	in, err := os.Open(blob)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(dest), ".copy-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(out.Name(), dest)
}

func newClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// The default is two idle connections per host, which is invisible while
	// the loop is sequential and wasteful once it is not.
	transport.MaxIdleConnsPerHost = 16
	transport.MaxConnsPerHost = 32
	return &http.Client{Timeout: 5 * time.Minute, Transport: transport}
}

// newBulkClient returns the client for the tarball tier. The row client sets a
// five-minute total timeout, which is right for a file measured in KiB and
// wrong for a 4.5 GB body: Client.Timeout covers the read, so a slow link
// would abort every attempt part way through. This client bounds the header
// wait and leaves the body to the TCP stream and the retry loop.
func newBulkClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 16
	transport.MaxConnsPerHost = 32
	transport.ResponseHeaderTimeout = 60 * time.Second
	return &http.Client{Transport: transport}
}

func hostOf(source string) string {
	rest := strings.TrimPrefix(source, "https://")
	rest = strings.TrimPrefix(rest, "http://")
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func policyFor(host string) budget {
	if p, ok := hostPolicy[host]; ok {
		return p
	}
	return defaultBudget
}

func fail(format string, args ...any) {
	log.Fatalf("gen: "+format, args...)
}

// A bulkArchive pins one tarball tier. The members file lists every regular
// file the archive holds, with its SHA-256, so extraction verifies each file
// before it reaches the tree.
type bulkArchive struct {
	Path    string
	URL     string
	SHA512  string
	Bytes   int64
	Members string
}

// A bulkMember is one regular file inside a bulk archive, relative to the
// archive's destination root.
type bulkMember struct {
	Path   string
	SHA256 string
	Bytes  int64
}

// loadBulk reads the committed bulk manifest. A missing file is an empty
// tier, so an older checkout still runs.
func loadBulk(rel string) ([]bulkArchive, error) {
	f, err := os.Open(filepath.Join(validation.CorpusDir(), filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var archives []bulkArchive
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSuffix(scanner.Text(), "\r")
		if line == 1 {
			if text != "path\turl\tsha512\tbytes\tmembers" {
				return nil, fmt.Errorf("%s line 1: unexpected header %q", rel, text)
			}
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 5 {
			return nil, fmt.Errorf("%s line %d: %d columns, want 5", rel, line, len(fields))
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil || size <= 0 {
			return nil, fmt.Errorf("%s line %d: bytes %q", rel, line, fields[3])
		}
		if len(fields[2]) != 128 {
			return nil, fmt.Errorf("%s line %d: sha512 %q", rel, line, fields[2])
		}
		if fields[0] == "" || path.IsAbs(fields[0]) || strings.Contains(fields[0], "..") {
			return nil, fmt.Errorf("%s line %d: unsafe path %q", rel, line, fields[0])
		}
		archives = append(archives, bulkArchive{
			Path: fields[0], URL: fields[1], SHA512: fields[2],
			Bytes: size, Members: fields[4],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return archives, nil
}

// loadBulkMembers reads one archive's member list, sorted by path.
func loadBulkMembers(rel string) ([]bulkMember, error) {
	f, err := os.Open(filepath.Join(validation.CorpusDir(), filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var members []bulkMember
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSuffix(scanner.Text(), "\r")
		if line == 1 {
			if text != "sha256\tbytes\tpath" {
				return nil, fmt.Errorf("%s line 1: unexpected header %q", rel, text)
			}
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s line %d: %d columns, want 3", rel, line, len(fields))
		}
		size, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || size < 0 {
			return nil, fmt.Errorf("%s line %d: bytes %q", rel, line, fields[1])
		}
		if len(fields[0]) != 64 {
			return nil, fmt.Errorf("%s line %d: sha256 %q", rel, line, fields[0])
		}
		members = append(members, bulkMember{
			Path: fields[2], SHA256: fields[0], Bytes: size,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

// humanBytes prints a byte count in the two units a user checks it against:
// the decimal figure a download page shows and the binary figure `du` shows.
func humanBytes(n int64) string {
	return fmt.Sprintf("%.1f GB (%.2f GiB)", float64(n)/1e9, float64(n)/(1<<30))
}

// bulkTotals counts the tier's files and download bytes.
func bulkTotals(archives []bulkArchive) (files int, bytes int64) {
	for _, a := range archives {
		members, err := loadBulkMembers(a.Members)
		if err != nil {
			continue
		}
		files += len(members)
		bytes += a.Bytes
	}
	return files, bytes
}

// printBulkPlan reports the tier's cost without touching the network.
func printBulkPlan(archives []bulkArchive, fetch bool) {
	if len(archives) == 0 {
		return
	}
	files, bytes := bulkTotals(archives)
	fmt.Printf("gen: bulk tier: %d archives, %d files, %s download\n",
		len(archives), files, humanBytes(bytes))
	if fetch {
		fmt.Println("gen: -fetch-bulk is set, the archives would be fetched and verified")
	}
}

// printBulkNotice names the tier and its cost when a run leaves it alone.
func printBulkNotice(archives []bulkArchive) {
	files, bytes := bulkTotals(archives)
	names := make([]string, 0, len(archives))
	for _, a := range archives {
		names = append(names, path.Base(a.URL))
	}
	fmt.Printf("gen: bulk tier not fetched: %s, %d files, %s download; run make validation-fetch BULK=1 to add it\n",
		strings.Join(names, ", "), files, humanBytes(bytes))
}

// runBulk fetches or verifies the bulk tier. Without -fetch-bulk it prints the
// cost so a user knows the tier exists before choosing to add 4.5 GB.
func runBulk(client *http.Client, archives []bulkArchive, fetch, verifyOnly bool, cache string) error {
	if len(archives) == 0 {
		return nil
	}
	if !fetch {
		printBulkNotice(archives)
		return nil
	}
	for _, a := range archives {
		if err := fetchBulkArchive(client, a, verifyOnly, cache); err != nil {
			return err
		}
	}
	return nil
}

// fetchBulkArchive brings one archive's files into the tree. The archive is
// verified as a whole, then extracted to staging, then verified member by
// member, so the tree only ever sees files whose digest is pinned.
func fetchBulkArchive(client *http.Client, a bulkArchive, verifyOnly bool, cache string) error {
	members, err := loadBulkMembers(a.Members)
	if err != nil {
		return fmt.Errorf("%s: %w", a.Members, err)
	}
	destRoot := filepath.Join(validation.CorpusDir(), filepath.FromSlash(a.Path))
	log.Printf("gen: bulk %s: %d files, %s, into %s",
		path.Base(a.URL), len(members), humanBytes(a.Bytes), destRoot)

	if verifyOnly {
		if err := verifyBulkTree(destRoot, members); err != nil {
			return fmt.Errorf("%s: %w", a.Path, err)
		}
		fmt.Printf("gen: bulk %s: %d files present and verified\n", path.Base(a.URL), len(members))
		return nil
	}

	blob := filepath.Join(cache, "bulk", a.SHA512)
	ok, err := verifyBulkBlob(blob, a)
	if err != nil {
		return err
	}
	if !ok {
		if err := downloadBulk(client, a, blob); err != nil {
			return err
		}
	}
	// A complete tree costs no extraction, which matters because the archive
	// is 4.5 GB and the extraction is 5.7 GB.
	if err := verifyBulkTree(destRoot, members); err == nil {
		fmt.Printf("gen: bulk %s: %d files present and verified\n", path.Base(a.URL), len(members))
		return nil
	}
	return extractBulk(blob, destRoot, members)
}

// verifyBulkTree checks every member of one archive against the tree.
func verifyBulkTree(root string, members []bulkMember) error {
	for _, m := range members {
		full := filepath.Join(root, filepath.FromSlash(m.Path))
		f, err := os.Open(full)
		if err != nil {
			return err
		}
		hasher := sha256.New()
		size, err := io.Copy(hasher, f)
		f.Close()
		if err != nil {
			return err
		}
		if size != m.Bytes {
			return fmt.Errorf("%s: %d bytes, want %d", m.Path, size, m.Bytes)
		}
		if got := hex.EncodeToString(hasher.Sum(nil)); got != m.SHA256 {
			return fmt.Errorf("%s: sha256 %s, want %s", m.Path, got, m.SHA256)
		}
	}
	return nil
}

// verifyBulkBlob reports whether the cache holds exactly the archive's bytes.
func verifyBulkBlob(blob string, a bulkArchive) (bool, error) {
	f, err := os.Open(blob)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()
	hasher := sha512.New()
	size, err := io.Copy(hasher, f)
	if err != nil {
		return false, err
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); got != a.SHA512 {
		return false, fmt.Errorf("%s: cache sha512 %s, want %s", a.URL, got, a.SHA512)
	}
	if size != a.Bytes {
		return false, fmt.Errorf("%s: cache %d bytes, want %d", a.URL, size, a.Bytes)
	}
	return true, nil
}

// downloadBulk streams one archive into the cache, retrying what is worth
// retrying. The digest is checked before the blob gets its final name.
func downloadBulk(client *http.Client, a bulkArchive, blob string) error {
	if err := os.MkdirAll(filepath.Dir(blob), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(blob), ".bulk-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		sum, size, retryable, err := fetchBulkOnce(client, a, tmp)
		if err == nil {
			if got := hex.EncodeToString(sum[:]); got != a.SHA512 {
				return fmt.Errorf("sha512 %s, want %s", got, a.SHA512)
			}
			if size != a.Bytes {
				return fmt.Errorf("%d bytes, want %d", size, a.Bytes)
			}
			if err := tmp.Close(); err != nil {
				return err
			}
			return os.Rename(tmpName, blob)
		}
		if !retryable {
			tmp.Close()
			return err
		}
		lastErr = err
		if attempt == maxAttempts {
			break
		}
		wait := backoff(attempt)
		log.Printf("gen: %s: %v, retrying in %s", a.URL, err, wait)
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := tmp.Truncate(0); err != nil {
			return err
		}
		time.Sleep(wait)
	}
	tmp.Close()
	return fmt.Errorf("after %d attempts: %w", maxAttempts, lastErr)
}

// fetchBulkOnce performs one archive request and reports whether the failure
// is worth retrying, matching fetchOnce.
func fetchBulkOnce(client *http.Client, a bulkArchive, w io.Writer) ([64]byte, int64, bool, error) {
	var zero [64]byte
	req, err := http.NewRequest(http.MethodGet, a.URL, nil)
	if err != nil {
		return zero, 0, false, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return zero, 0, true, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusRequestTimeout,
		resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode >= 500:
		return zero, 0, true, fmt.Errorf("get: %s", resp.Status)
	default:
		return zero, 0, false, fmt.Errorf("get: %s", resp.Status)
	}
	hasher := sha512.New()
	size, err := io.Copy(io.MultiWriter(w, hasher), resp.Body)
	if err != nil {
		return zero, 0, true, fmt.Errorf("read: %w", err)
	}
	var sum [64]byte
	copy(sum[:], hasher.Sum(nil))
	return sum, size, false, nil
}

// extractBulk unpacks a verified archive into staging, checks every member
// against the members manifest, and only then moves files into the tree, so a
// kill or a bad member leaves the tree as it was.
func extractBulk(blob, destRoot string, members []bulkMember) error {
	byPath := make(map[string]bulkMember, len(members))
	for _, m := range members {
		byPath[m.Path] = m
	}
	staging, err := os.MkdirTemp(filepath.Dir(blob), "extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	f, err := os.Open(blob)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	seen := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		name := path.Clean(hdr.Name)
		if name != hdr.Name || path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("unsafe archive member %q", hdr.Name)
		}
		member, ok := byPath[name]
		if !ok {
			return fmt.Errorf("archive member not in the members manifest: %s", name)
		}
		target := filepath.Join(staging, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		hasher := sha256.New()
		size, copyErr := io.Copy(io.MultiWriter(out, hasher), tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if size != member.Bytes {
			return fmt.Errorf("%s: %d bytes, want %d", name, size, member.Bytes)
		}
		if got := hex.EncodeToString(hasher.Sum(nil)); got != member.SHA256 {
			return fmt.Errorf("%s: sha256 %s, want %s", name, got, member.SHA256)
		}
		seen++
	}
	if seen != len(byPath) {
		return fmt.Errorf("%d files in the archive, %d in the members manifest", seen, len(byPath))
	}
	// Every member verified in staging. Only now touch the tree.
	for _, m := range members {
		src := filepath.Join(staging, filepath.FromSlash(m.Path))
		dst := filepath.Join(destRoot, filepath.FromSlash(m.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := moveFile(src, dst); err != nil {
			return err
		}
	}
	fmt.Printf("gen: bulk %s: %d files extracted and verified\n", path.Base(destRoot), seen)
	return nil
}

// moveFile renames src to dst, falling back to a copy across filesystems.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
