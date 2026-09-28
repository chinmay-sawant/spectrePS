//go:build ignore

// Command gen restores the validation corpus from its pinned sources. Run it
// from anywhere in the module:
//
//	go run internal/validation/gen.go                    # the committed tier
//	go run internal/validation/gen.go -fetch-external    # the fetched tier
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
// # Politeness
//
// Per-host concurrency and rate limits live in hostPolicy below, not in a flag,
// so the numbers are reviewable in a diff. Every request carries a descriptive
// User-Agent naming the tool and a contact, which sec.gov and Wikimedia both
// require and both enforce with a 403.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
		return
	}

	cache := cacheRoot(*cacheDir)
	if *dryRun {
		printPlan(selected, cache)
		return
	}

	if err := resolveAll(newClient(), selected, cache, *verifyOnly, *workers); err != nil {
		fail("%v", err)
	}

	// Every row resolved before this point. Only now touch the tree, so a
	// failure above leaves the working tree as it was.
	dir := validation.CorpusDir()
	for _, row := range selected {
		blob := filepath.Join(cache, blobName(row.SHA256))
		path := filepath.Join(dir, filepath.FromSlash(row.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fail("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := place(blob, path); err != nil {
			fail("place %s: %v", row.Path, err)
		}
	}
	fmt.Printf("gen: %d rows, all present and verified\n", len(selected))
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
