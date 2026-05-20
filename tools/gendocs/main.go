// gendocs regenerates the per-command Markdown reference under
// docs/commands/ from the live cobra command tree.
//
// Usage: go run ./tools/gendocs [output-dir]
//
// The default output directory is docs/commands relative to the
// current working directory. The tool wipes the target directory
// before writing so deleted commands do not linger as stale files.
//
// To keep generated output reproducible the cobra "auto-generated"
// timestamp footer is suppressed.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra/doc"

	"github.com/tas50/cinc-cli/apps/cinc/cmd"
)

func main() {
	outDir := "docs/commands"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	if err := os.RemoveAll(outDir); err != nil {
		exit("clear %s: %v", outDir, err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		exit("create %s: %v", outDir, err)
	}

	root := cmd.NewRootCmd()
	root.DisableAutoGenTag = true

	if err := doc.GenMarkdownTreeCustom(root, outDir, frontmatter, linkHandler); err != nil {
		exit("generate docs: %v", err)
	}

	abs, _ := filepath.Abs(outDir)
	fmt.Printf("Wrote %s\n", abs)
}

// frontmatter returns an empty prefix; we keep the generated output as
// plain Markdown without any static-site frontmatter.
func frontmatter(string) string { return "" }

// linkHandler rewrites the cross-references cobra emits in the "SEE
// ALSO" section so they resolve relative to docs/commands/ rather than
// pointing at the parent docs/ directory.
func linkHandler(name string) string {
	base := strings.TrimSuffix(name, ".md")
	return base + ".md"
}

func exit(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "gendocs: "+format+"\n", a...)
	os.Exit(1)
}
