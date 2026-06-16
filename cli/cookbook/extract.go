package cookbook

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ExtractArchive unpacks a gzipped cookbook tarball read from r into
// destDir and returns the path of the single top-level directory the
// archive creates (Supermarket tarballs are rooted at <cookbook>/...).
//
// Entries whose paths escape destDir are refused, so a hostile tarball
// can't write outside the destination.
func ExtractArchive(r io.Reader, destDir string) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	roots := map[string]struct{}{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}

		target, err := safeJoin(destDir, hdr.Name)
		if err != nil {
			return "", err
		}
		if root := topLevel(hdr.Name); root != "" {
			roots[root] = struct{}{}
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := writeFile(tr, target); err != nil {
				return "", err
			}
		default:
			// Skip symlinks, devices, and other non-regular entries.
		}
	}

	if len(roots) != 1 {
		return "", fmt.Errorf("cookbook archive has %d top-level entries, want exactly 1", len(roots))
	}
	var root string
	for r := range roots {
		root = r
	}
	return filepath.Join(destDir, root), nil
}

// safeJoin joins name onto destDir and verifies the result stays within
// destDir, guarding against path-traversal ("zip slip") entries.
func safeJoin(destDir, name string) (string, error) {
	target := filepath.Join(destDir, filepath.FromSlash(name))
	rel, err := filepath.Rel(destDir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry %q escapes the destination directory", name)
	}
	return target, nil
}

// topLevel returns the first real path segment of a tar entry name, or ""
// if the entry has none. It normalizes away leading "./" and "/" so a
// "./nginx/metadata.rb" entry (the layout real Supermarket tarballs use)
// reports "nginx" rather than ".", and bare "." / ".." entries report "".
func topLevel(name string) string {
	clean := strings.TrimPrefix(path.Clean(filepath.ToSlash(name)), "/")
	if clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	root, _, _ := strings.Cut(clean, "/")
	return root
}

// writeFile creates target (with parent directories) and copies the
// current tar entry into it.
func writeFile(r io.Reader, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}
	f, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", target, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", target, err)
	}
	return nil
}
