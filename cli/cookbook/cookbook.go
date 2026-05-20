// Package cookbook handles local cookbook discovery and packaging helpers.
package cookbook

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	_ "unsafe"

	cinc "github.com/tas50/cinc-api"
)

// Metadata is the minimal cookbook metadata this CLI needs for upload flows.
type Metadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Archive is an in-memory gzip-compressed tarball and its manifest.
type Archive struct {
	Name  string
	Bytes []byte
	Files []string
}

// Locate finds a cookbook by name. cookbookPath may be empty, a single path,
// or a filepath-list of parent directories.
func Locate(name, cookbookPath string) (string, error) {
	for _, base := range candidateBases(cookbookPath) {
		if dir, ok := cookbookDir(base, name); ok {
			return dir, nil
		}
	}
	if cookbookPath == "" {
		return "", fmt.Errorf("cookbook %q not found in current directory or ./<name>", name)
	}
	return "", fmt.Errorf("cookbook %q not found in cookbook path %q", name, cookbookPath)
}

// LoadMetadataJSON reads metadata.json, returning a clear compatibility error
// when only metadata.rb is present.
func LoadMetadataJSON(dir string) (Metadata, error) {
	path := filepath.Join(dir, "metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if _, statErr := os.Stat(filepath.Join(dir, "metadata.rb")); statErr == nil {
				return Metadata{}, fmt.Errorf("metadata.json is required for Supermarket upload. Generate it with knife cookbook metadata COOKBOOK or run with Chef Workstation")
			}
		}
		return Metadata{}, fmt.Errorf("read metadata.json: %w", err)
	}
	var md Metadata
	if err := json.Unmarshal(data, &md); err != nil {
		return Metadata{}, fmt.Errorf("parse metadata.json: %w", err)
	}
	if md.Name == "" {
		return Metadata{}, fmt.Errorf("metadata.json is missing name")
	}
	if md.Version == "" {
		return Metadata{}, fmt.Errorf("metadata.json is missing version")
	}
	return md, nil
}

// ReadVersion returns the cookbook version from metadata.json or from the
// common literal metadata.rb form: version "1.2.3".
func ReadVersion(dir string) (string, error) {
	if md, err := LoadMetadataJSON(dir); err == nil {
		return md.Version, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "metadata.rb"))
	if err != nil {
		return "", fmt.Errorf("read cookbook metadata: %w", err)
	}
	m := metadataRBVersion.FindSubmatch(data)
	if len(m) != 2 {
		return "", fmt.Errorf("metadata.rb must contain a literal version for cookbook upload")
	}
	return string(m[1]), nil
}

var metadataRBVersion = regexp.MustCompile(`(?m)^\s*version\s+['"]([^'"]+)['"]`)

// BuildArchive builds a deterministic cookbook tarball rooted at cookbookName/.
func BuildArchive(dir, cookbookName string) (Archive, error) {
	return buildArchive(dir, cookbookName, true)
}

// BuildUploadArchive builds the upload tarball without retaining a separate
// file-name manifest, which is only needed for dry-run output.
func BuildUploadArchive(dir, cookbookName string) (Archive, error) {
	return buildArchive(dir, cookbookName, false)
}

func buildArchive(dir, cookbookName string, includeFiles bool) (Archive, error) {
	entries, err := archiveEntries(dir)
	if err != nil {
		return Archive{}, err
	}
	if len(entries) == 0 {
		return Archive{}, fmt.Errorf("cookbook %s has no files", dir)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Name = cookbookName + ".tgz"
	gz.ModTime = time.Unix(0, 0)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(entry.path)))
		if err != nil {
			return Archive{}, fmt.Errorf("open %s: %w", entry.path, err)
		}
		hdr := &tar.Header{
			Name:    cookbookName + "/" + entry.path,
			Mode:    0o644,
			Size:    entry.size,
			ModTime: time.Unix(0, 0),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			_ = f.Close()
			return Archive{}, fmt.Errorf("write tar header %s: %w", entry.path, err)
		}
		if _, err := io.Copy(tw, f); err != nil {
			_ = f.Close()
			return Archive{}, fmt.Errorf("write tar entry %s: %w", entry.path, err)
		}
		if err := f.Close(); err != nil {
			return Archive{}, fmt.Errorf("close %s: %w", entry.path, err)
		}
	}
	if err := tw.Close(); err != nil {
		return Archive{}, fmt.Errorf("close tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return Archive{}, fmt.Errorf("close gzip: %w", err)
	}
	var files []string
	if includeFiles {
		files = make([]string, len(entries))
		for i, entry := range entries {
			files[i] = cookbookName + "/" + entry.path
		}
	}
	return Archive{Name: cookbookName + ".tgz", Bytes: buf.Bytes(), Files: files}, nil
}

// UploadableFromDir builds the cinc-api LocalCookbook value used by the API's
// server upload implementation.
func UploadableFromDir(dir, version string) (*cinc.LocalCookbook, error) {
	return apiCookbookFromDir(dir, version)
}

//go:linkname apiCookbookFromDir github.com/tas50/cinc-api.cookbookFromDir
func apiCookbookFromDir(dir, version string) (*cinc.LocalCookbook, error)

func candidateBases(cookbookPath string) []string {
	if cookbookPath == "" {
		return []string{"."}
	}
	parts := filepath.SplitList(cookbookPath)
	if len(parts) == 0 {
		return []string{cookbookPath}
	}
	return parts
}

func cookbookDir(base, name string) (string, bool) {
	info, err := os.Stat(base)
	if err == nil && info.IsDir() && filepath.Base(base) == name && hasMetadata(base) {
		return base, true
	}
	dir := filepath.Join(base, name)
	info, err = os.Stat(dir)
	if err == nil && info.IsDir() && hasMetadata(dir) {
		return dir, true
	}
	return "", false
}

func hasMetadata(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "metadata.json")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "metadata.rb")); err == nil {
		return true
	}
	return false
}

type archiveEntry struct {
	path string
	size int64
}

func archiveEntries(dir string) ([]archiveEntry, error) {
	var entries []archiveEntry
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, ".git/") {
			return nil
		}
		entries = append(entries, archiveEntry{path: rel, size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk cookbook: %w", err)
	}
	slices.SortFunc(entries, func(a, b archiveEntry) int {
		return strings.Compare(a.path, b.path)
	})
	return entries, nil
}

// ExtractArchiveFiles returns archive entry names. It is used by tests and
// dry-run output validation without shelling out to tar.
func ExtractArchiveFiles(data []byte) ([]string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		files = append(files, hdr.Name)
	}
	return files, nil
}
