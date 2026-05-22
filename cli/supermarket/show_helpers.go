package supermarket

import (
	"sort"
	"strings"
)

// VersionFromURL extracts the version segment from a Supermarket
// version URL like
// https://supermarket.chef.io/api/v1/cookbooks/nginx/versions/12_0_4
// returning "12.0.4". Supermarket encodes dots in URLs as underscores;
// we tolerate either form. If the URL doesn't look like a version
// URL, the input is returned unchanged so callers can pass through
// raw "12.0.4" version strings.
func VersionFromURL(s string) string {
	if s == "" {
		return ""
	}
	tail := s
	if i := strings.LastIndex(tail, "/versions/"); i >= 0 {
		tail = tail[i+len("/versions/"):]
	}
	if i := strings.IndexAny(tail, "/?"); i >= 0 {
		tail = tail[:i]
	}
	return strings.ReplaceAll(tail, "_", ".")
}

// VersionListFromURLs decodes a slice of Supermarket version URLs and
// returns the version strings sorted newest-first.
func VersionListFromURLs(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if v := VersionFromURL(u); v != "" {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return compareSemver(out[i], out[j]) > 0
	})
	return out
}
