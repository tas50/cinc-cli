package rubyeval

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// metadataShimSource is the cookbook metadata.rb DSL shim run inside ruby.wasm.
//
//go:embed metadata_shim.rb
var metadataShimSource string

// Dependency is one cookbook dependency from a metadata.rb `depends`
// declaration: the dependency name and its Semverse-normalized version
// constraint (e.g. ["beta", "~> 1.0"]). The order of a cookbook's Dependencies
// is the declaration order, matching chef.
type Dependency struct {
	Name       string
	Constraint string
}

// Metadata is the resolution-relevant subset of a cookbook's metadata.rb /
// metadata.json: its name, version, and ordered dependency list. It is exactly
// what the Policyfile depsolver needs to build the dependency graph and the
// cookbook identifier inputs.
type Metadata struct {
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Dependencies []Dependency `json:"dependencies"`
	Errors       []string     `json:"errors"`
}

// metadataJSON mirrors the shim's JSON output, where dependencies are
// [name, constraint] pairs.
type metadataJSON struct {
	Name         string     `json:"name"`
	Version      string     `json:"version"`
	Dependencies [][]string `json:"dependencies"`
	Errors       []string   `json:"errors"`
}

// EvaluateMetadata runs a cookbook's metadata.rb source through the embedded
// CRuby engine and returns its name, version, and dependencies. Because the
// file runs in a real Ruby VM, any valid Ruby works (version helpers,
// conditionals, require_relative). Dependency constraints come back normalized
// by the same Semverse chef uses.
func (e *Engine) EvaluateMetadata(ctx context.Context, source string, opts Options) (*Metadata, error) {
	if opts.Filename == "" {
		opts.Filename = "metadata.rb"
	}
	raw, err := e.run(ctx, metadataShimSource, source, opts)
	if err != nil {
		return nil, err
	}
	var mj metadataJSON
	if err := json.Unmarshal(raw, &mj); err != nil {
		return nil, fmt.Errorf("policyfile: decode metadata engine output: %w", err)
	}
	md := &Metadata{Name: mj.Name, Version: mj.Version, Errors: mj.Errors}
	for _, pair := range mj.Dependencies {
		if len(pair) == 2 {
			md.Dependencies = append(md.Dependencies, Dependency{Name: pair[0], Constraint: pair[1]})
		}
	}
	if len(md.Errors) > 0 {
		return md, fmt.Errorf("policyfile: metadata.rb: %s", md.Errors[0])
	}
	return md, nil
}

// EvaluateMetadataFile reads a cookbook's metadata.rb from disk and evaluates
// it, using its directory for sibling resolution (require_relative of version
// helpers, etc.).
func (e *Engine) EvaluateMetadataFile(ctx context.Context, path string) (*Metadata, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return e.EvaluateMetadata(ctx, string(source), Options{
		Filename: filepath.Base(path),
		Dir:      filepath.Dir(path),
		Env:      hostEnv(),
	})
}
