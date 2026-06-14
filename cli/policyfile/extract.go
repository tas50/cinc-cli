// Package policyfile deploys an existing Policyfile.lock.json to a Cinc/Chef
// Server. It fetches and caches the cookbooks the lock pins (from the sources
// the lock records), then drives the server-side push via cinc-api, and
// assembles a standalone export bundle. It owns the local, multi-source
// concerns; every server call goes through cinc-api or cinc-supermarket.
package policyfile

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractCookbookTarball reads a gzip-compressed tar stream and writes its
// entries under dest, stripping the single leading path segment that
// Supermarket tarballs wrap a cookbook in (e.g. "nginx/metadata.rb" lands at
// dest/metadata.rb), so dest ends up holding the cookbook root directly. Paths
// that would escape dest are rejected.
func extractCookbookTarball(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("supermarket: open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("supermarket: read tar: %w", err)
		}
		rel := stripLeadingSegment(hdr.Name)
		if rel == "" {
			continue
		}
		target := filepath.Join(dest, rel)
		if !withinDir(dest, target) {
			return fmt.Errorf("supermarket: unsafe path in tarball: %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777|0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
}

// stripLeadingSegment drops the first path element of a slash-separated tar
// entry name, returning "" when there is nothing below it.
func stripLeadingSegment(name string) string {
	name = strings.TrimPrefix(filepath.ToSlash(name), "./")
	idx := strings.IndexByte(name, '/')
	if idx < 0 {
		return ""
	}
	return strings.Trim(name[idx+1:], "/")
}

// withinDir reports whether target stays inside dir (no "../" escape).
func withinDir(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
