package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tas50/cinc-cli/cli/policyfile/rubyeval"
)

// cookbookInfo is everything the resolver needs about one cookbook it has
// located from a source: its identity (name/version), its dependencies, where
// it lives on disk (for identifier computation and upload), and the lock fields
// describing how it is sourced.
type cookbookInfo struct {
	name    string
	version Version
	verStr  string // the metadata version string, written verbatim into the lock
	deps    []rubyeval.Dependency
	dir     string // on-disk cookbook root

	// Lock fields.
	source        string      // relative "source" path for local cookbooks
	sourceOptions *jsonObject // ordered source_options for the lock

	fixed bool // version_fixed? (path/git resolve to a single version)
}

// loadPathCookbook resolves a `cookbook "name", path: "..."` declaration: it
// reads the cookbook's metadata (metadata.json if present, else metadata.rb via
// the embedded Ruby engine) and records the on-disk directory plus the lock's
// source / source_options, computed relative to the Policyfile directory the
// way chef's CookbookOmnifetch::PathLocation does.
func loadPathCookbook(ctx context.Context, eng *rubyeval.Engine, policyfileDir, name, declaredPath string) (*cookbookInfo, error) {
	abs := declaredPath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(policyfileDir, declaredPath)
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("cinc: cookbook %q path source %q is not a directory: %w", name, declaredPath, err)
	}

	rel, err := filepath.Rel(policyfileDir, abs)
	if err != nil {
		rel = declaredPath
	}
	relSlash := filepath.ToSlash(rel)

	md, err := readCookbookMetadata(ctx, eng, abs)
	if err != nil {
		return nil, fmt.Errorf("cinc: cookbook %q: %w", name, err)
	}
	ver, err := ParseVersion(md.Version)
	if err != nil {
		return nil, fmt.Errorf("cinc: cookbook %q has invalid version %q: %w", name, md.Version, err)
	}

	so := newJSONObject()
	so.set("path", relSlash)

	return &cookbookInfo{
		name:          name,
		version:       ver,
		verStr:        md.Version,
		deps:          md.Dependencies,
		dir:           abs,
		source:        relSlash,
		sourceOptions: so,
		fixed:         true,
	}, nil
}

// readCookbookMetadata loads a cookbook's metadata from dir. metadata.json wins
// when present (chef reads it directly); otherwise metadata.rb is evaluated in
// the embedded CRuby engine. Dependency constraints come back Semverse-
// normalized either way.
func readCookbookMetadata(ctx context.Context, eng *rubyeval.Engine, dir string) (*rubyeval.Metadata, error) {
	jsonPath := filepath.Join(dir, "metadata.json")
	if _, err := os.Stat(jsonPath); err == nil {
		return readMetadataJSON(jsonPath)
	}
	rbPath := filepath.Join(dir, "metadata.rb")
	if _, err := os.Stat(rbPath); err == nil {
		return eng.EvaluateMetadataFile(ctx, rbPath)
	}
	return nil, fmt.Errorf("no metadata.rb or metadata.json found in %s", dir)
}

// metadataJSONFile is the subset of a cookbook's metadata.json the resolver
// reads. dependencies is an object of name => constraint string.
type metadataJSONFile struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

// readMetadataJSON parses a cookbook metadata.json, normalizing each dependency
// constraint through the same Semverse semantics chef applies so the lock's
// solution_dependencies match. Dependency order from a JSON object is not
// guaranteed, so it is sorted by name for determinism.
func readMetadataJSON(path string) (*rubyeval.Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var mj metadataJSONFile
	if err := json.Unmarshal(data, &mj); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	md := &rubyeval.Metadata{Name: mj.Name, Version: mj.Version}
	names := make([]string, 0, len(mj.Dependencies))
	for n := range mj.Dependencies {
		names = append(names, n)
	}
	sortStrings(names)
	for _, n := range names {
		c, err := ParseConstraint(mj.Dependencies[n])
		if err != nil {
			return nil, fmt.Errorf("%s: dependency %q: %w", path, n, err)
		}
		md.Dependencies = append(md.Dependencies, rubyeval.Dependency{Name: n, Constraint: normalizeConstraint(c)})
	}
	return md, nil
}
