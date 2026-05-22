package cookbook

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// LoadChefignore reads a `chefignore` file at dir and returns its
// patterns. A missing file is not an error — it returns (nil, nil).
func LoadChefignore(dir string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "chefignore"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return parseChefignore(data), nil
}

func parseChefignore(data []byte) []string {
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

// chefignoreMatches reports whether relPath should be excluded by any of
// the supplied glob patterns. It matches knife: each pattern is tested
// against the full slash-separated relative path, the basename, and
// every ancestor directory along the path. This makes patterns like
// `spec/*` exclude files nested arbitrarily deep under `spec/`.
func chefignoreMatches(patterns []string, relPath string) bool {
	if len(patterns) == 0 || relPath == "" || relPath == "." {
		return false
	}
	candidates := chefignoreCandidates(relPath)
	for _, pattern := range patterns {
		for _, c := range candidates {
			if ok, _ := path.Match(pattern, c); ok {
				return true
			}
		}
	}
	return false
}

func chefignoreCandidates(relPath string) []string {
	candidates := []string{relPath, path.Base(relPath)}
	dir := path.Dir(relPath)
	for dir != "." && dir != "/" && dir != "" {
		candidates = append(candidates, dir, path.Base(dir))
		dir = path.Dir(dir)
	}
	return candidates
}
