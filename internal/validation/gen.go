//go:build ignore

// Command gen restores the validation corpus from its pinned sources. Run it
// from anywhere in the module:
//
//	go run internal/validation/gen.go
//	go run internal/validation/gen.go -fetch-external
//
// The command needs the network. The product does not: tests read the bytes
// that are checked in, and the external tier is optional.
//
// # Behavior
//
// The manifest at sampledata/validation/manifest.tsv names a pinned URL and a
// SHA-256 for every fetched file. Every URL is a raw.githubusercontent.com URL
// with an immutable commit in it. Repo-authored rows are skipped, because
// their bytes already live in the tree and TestValidationManifest checks their
// digests. External rows are skipped unless -fetch-external is set.
//
// Every file is downloaded and verified before any file is written, so a
// digest or byte-count mismatch exits non-zero and writes nothing. The command
// writes no timestamp: running it twice leaves the working tree unchanged.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chinmay-sawant/spectrePS/internal/validation"
)

func main() {
	fetchExternal := flag.Bool("fetch-external", false, "also fetch the external tier under sampledata/validation/external")
	flag.Parse()

	rows, err := validation.Load()
	if err != nil {
		fail("load manifest: %v", err)
	}
	client := &http.Client{Timeout: 120 * time.Second}

	type file struct {
		row  validation.Row
		data []byte
	}
	verified := make([]file, 0, len(rows))
	for _, row := range rows {
		switch {
		case row.Source == validation.LicenseRepoAuthored:
			continue
		case row.External() && !*fetchExternal:
			continue
		}
		data := fetch(client, row)
		verified = append(verified, file{row: row, data: data})
	}

	// Every fetch passed before this point. Only now touch the tree.
	dir := validation.CorpusDir()
	for _, item := range verified {
		path := filepath.Join(dir, filepath.FromSlash(item.row.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fail("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, item.data, 0o644); err != nil {
			fail("write %s: %v", path, err)
		}
		fmt.Printf("wrote %s (%d bytes)\n", item.row.Path, len(item.data))
	}
	if len(verified) == 0 {
		fmt.Println("gen: no fetched rows selected")
	}
}

// fetch downloads one row and verifies its SHA-256 and byte count.
func fetch(client *http.Client, row validation.Row) []byte {
	resp, err := client.Get(row.Source)
	if err != nil {
		fail("%s: get: %v", row.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fail("%s: get: %s", row.Path, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fail("%s: read: %v", row.Path, err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != row.SHA256 {
		fail("%s: sha256 %s, want %s", row.Path, got, row.SHA256)
	}
	if int64(len(data)) != row.Bytes {
		fail("%s: %d bytes, want %d", row.Path, len(data), row.Bytes)
	}
	return data
}

func fail(format string, args ...any) {
	log.Fatalf("gen: "+format, args...)
}
