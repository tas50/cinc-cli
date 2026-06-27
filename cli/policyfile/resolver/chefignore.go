package resolver

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// chefignore holds the glob patterns from a cookbook's chefignore file and
// reports whether a cookbook-relative path is ignored, matching
// Chef::Cookbook::Chefignore#ignored? (File.fnmatch? with default flags).
type chefignore struct {
	globs []*regexp.Regexp
}

// loadChefignore reads a chefignore file at the root of dir, if present. A
// missing file means nothing is ignored, which is chef's behavior.
func loadChefignore(dir string) (*chefignore, error) {
	f, err := os.Open(filepath.Join(dir, "chefignore"))
	if err != nil {
		if os.IsNotExist(err) {
			return &chefignore{}, nil
		}
		return nil, err
	}
	defer f.Close()

	ci := &chefignore{}
	commentOrBlank := regexp.MustCompile(`^\s*(?:#.*)?$`)
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()
		if commentOrBlank.MatchString(line) {
			continue
		}
		ci.globs = append(ci.globs, fnmatchToRegexp(strings.TrimSpace(line)))
	}
	return ci, scan.Err()
}

// ignored reports whether the relative path matches any chefignore glob.
func (c *chefignore) ignored(relPath string) bool {
	for _, g := range c.globs {
		if g.MatchString(relPath) {
			return true
		}
	}
	return false
}

// fnmatchToRegexp translates a glob into a regexp with Ruby File.fnmatch's
// default semantics: "*" matches any run of characters (including "/"), "?"
// matches a single character, and "[...]" is a character class. This covers the
// chefignore patterns cookbooks use in practice (e.g. "*~", "*.bak", ".git").
func fnmatchToRegexp(glob string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString(`\A`)
	for i := 0; i < len(glob); i++ {
		ch := glob[i]
		switch ch {
		case '*':
			b.WriteString(`.*`)
		case '?':
			b.WriteString(`.`)
		case '[':
			// Pass a character class through, copying up to the matching ']'.
			j := i + 1
			for j < len(glob) && glob[j] != ']' {
				j++
			}
			if j < len(glob) {
				b.WriteByte('[')
				b.WriteString(glob[i+1 : j])
				b.WriteByte(']')
				i = j
			} else {
				b.WriteString(regexp.QuoteMeta(string(ch)))
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	b.WriteString(`\z`)
	return regexp.MustCompile(b.String())
}
