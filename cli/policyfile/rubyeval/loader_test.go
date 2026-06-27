package rubyeval

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPinnedConstants guards the download contract: a version, a URL that
// references the pinned version and asset, and a 64-hex-char SHA-256. These are
// the single source of truth for which ruby.wasm we run.
func TestPinnedConstants(t *testing.T) {
	if rubyWasmVersion == "" {
		t.Error("rubyWasmVersion must be pinned")
	}
	if !strings.Contains(rubyWasmURL, rubyWasmVersion) || !strings.Contains(rubyWasmURL, rubyWasmAsset) {
		t.Errorf("rubyWasmURL %q must reference the pinned version and asset", rubyWasmURL)
	}
	if len(rubyWasmSHA256) != 64 {
		t.Errorf("rubyWasmSHA256 must be 64 hex chars, got %d", len(rubyWasmSHA256))
	}
	if _, err := hex.DecodeString(rubyWasmSHA256); err != nil {
		t.Errorf("rubyWasmSHA256 is not valid hex: %v", err)
	}
}

func TestVerifySHA256(t *testing.T) {
	data := []byte("hello cinc")
	sum := sha256.Sum256(data)
	good := hex.EncodeToString(sum[:])

	if err := verifySHA256(data, good); err != nil {
		t.Errorf("matching checksum should pass: %v", err)
	}
	if err := verifySHA256(data, strings.Repeat("0", 64)); err == nil {
		t.Error("mismatched checksum must be rejected")
	}
}

// TestMaterializeRejectsBadChecksum proves a download whose bytes do not match
// the expected SHA is rejected and nothing is written into the cache dir — all
// without touching the network (the fetcher is injected).
func TestMaterializeRejectsBadChecksum(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "release")
	fetch := func(url string) ([]byte, error) { return []byte("not the real ruby.wasm"), nil }

	err := materializeWith(dir, fetch, rubyWasmSHA256)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}
	if _, statErr := os.Stat(dir); statErr == nil {
		t.Error("cache dir must not be created on a checksum failure")
	}
}

// TestMaterializeExtractsVerifiedArchive proves the happy path: a tar.gz whose
// SHA matches is extracted into the cache dir. Uses a tiny crafted archive and
// its real checksum, so no network and no multi-MB blob are needed.
func TestMaterializeExtractsVerifiedArchive(t *testing.T) {
	archive := makeTarGz(t, map[string]string{
		"ruby-3.4-wasm32-unknown-wasip1-full/usr/local/bin/ruby":  "\x00asm fake",
		"ruby-3.4-wasm32-unknown-wasip1-full/usr/local/lib/x.txt": "lib",
	})
	sum := sha256.Sum256(archive)
	wantSHA := hex.EncodeToString(sum[:])

	dir := filepath.Join(t.TempDir(), "release")
	fetch := func(url string) ([]byte, error) { return archive, nil }
	if err := materializeWith(dir, fetch, wantSHA); err != nil {
		t.Fatalf("materialize: %v", err)
	}

	if !fileExists(filepath.Join(dir, rubyWasmTreeBinary)) {
		t.Error("expected the wasm binary to be extracted")
	}
	if !dirExists(filepath.Join(dir, rubyWasmTreeUsr)) {
		t.Error("expected the usr tree to be extracted")
	}
}

// TestExtractTarGzRejectsTraversal ensures a malicious "../" entry cannot
// escape the extraction directory.
func TestExtractTarGzRejectsTraversal(t *testing.T) {
	archive := makeTarGz(t, map[string]string{"../escape.txt": "pwned"})
	dir := t.TempDir()
	err := extractTarGz(archive, dir)
	if err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
}

func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Size:     int64(len(content)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
