package explore

import (
	"context"
	"fmt"
	"sort"

	cinc "github.com/tas50/cinc-api"
)

// cookbookKind is the top-level Cookbooks kind. A cookbook is a named
// set of versions; you drill into it to reach individual versions.
// Cookbooks have no name-level mutation — versions are uploaded by the
// `cinc cookbook upload` command and deleted/downloaded per version.
type cookbookKind struct{}

func (cookbookKind) Title() string     { return "Cookbooks" }
func (cookbookKind) Columns() []string { return []string{"NAME", "VERSIONS"} }

func (cookbookKind) List(ctx context.Context, c *cinc.Client) ([]Row, error) {
	index, _, err := c.Cookbooks.List(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(index))
	for name, entry := range index {
		rows = append(rows, Row{
			Name:  name,
			Cells: []string{name, fmt.Sprintf("%d", len(entry.Versions))},
		})
	}
	sortRows(rows)
	return rows, nil
}

func (cookbookKind) Child(parent string) Kind { return cookbookVersionsKind{name: parent} }

// Summary shows how many versions a cookbook has and which is newest — the
// shape of the version set you're about to drill into.
func (cookbookKind) Summary(ctx context.Context, c *cinc.Client, name string) (summaryView, error) {
	return summarize(ctx, c, name,
		func(ctx context.Context, c *cinc.Client, n string) (*cinc.CookbookListEntry, error) {
			index, _, err := c.Cookbooks.List(ctx)
			if err != nil {
				return nil, err
			}
			entry, ok := index[n]
			if !ok {
				return nil, fmt.Errorf("cookbook %q not found", n)
			}
			return &entry, nil
		},
		nil,
		func(_ context.Context, _ *cinc.Client, entry *cinc.CookbookListEntry) []summaryField {
			return []summaryField{
				{"Versions", count(len(entry.Versions))},
				{"Latest", orDash(latestCookbookVersion(entry.Versions))},
			}
		})
}

// latestCookbookVersion returns the newest version string, picking the
// greatest by string comparison to match the newest-first ordering the
// version list itself uses.
func latestCookbookVersion(versions []cinc.CookbookVersion) string {
	latest := ""
	for _, v := range versions {
		if v.Version > latest {
			latest = v.Version
		}
	}
	return latest
}

// cookbookVersionsKind lists the versions of one cookbook. A version
// can be viewed (its manifest), downloaded, or deleted.
type cookbookVersionsKind struct{ name string }

func (k cookbookVersionsKind) Title() string   { return k.name }
func (cookbookVersionsKind) Columns() []string { return []string{"VERSION"} }

func (k cookbookVersionsKind) List(ctx context.Context, c *cinc.Client) ([]Row, error) {
	index, _, err := c.Cookbooks.List(ctx)
	if err != nil {
		return nil, err
	}
	entry, ok := index[k.name]
	if !ok {
		return nil, fmt.Errorf("cookbook %q not found", k.name)
	}
	rows := make([]Row, 0, len(entry.Versions))
	for _, v := range entry.Versions {
		rows = append(rows, Row{Name: v.Version, Cells: []string{v.Version}})
	}
	// Versions sort newest-first so the latest is at the top.
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name > rows[j].Name })
	return rows, nil
}

func (k cookbookVersionsKind) Describe(ctx context.Context, c *cinc.Client, version string) (string, error) {
	cb, _, err := c.Cookbooks.Get(ctx, k.name, version)
	if err != nil {
		return "", err
	}
	return prettyJSON(cb)
}

// Summary shows a version's identity — what it does, who maintains it, and
// under what license — drawn from the cookbook metadata, plus how many files
// its manifest carries.
func (k cookbookVersionsKind) Summary(ctx context.Context, c *cinc.Client, version string) (summaryView, error) {
	return summarize(ctx, c, version,
		func(ctx context.Context, c *cinc.Client, v string) (*cinc.Cookbook, error) {
			cb, _, err := c.Cookbooks.Get(ctx, k.name, v)
			return cb, err
		},
		nil,
		func(_ context.Context, _ *cinc.Client, cb *cinc.Cookbook) []summaryField {
			return cookbookVersionSummaryFields(cb)
		})
}

// cookbookVersionSummaryFields builds the curated facts panel for a cookbook
// version: its version and metadata identity, then the manifest file count.
func cookbookVersionSummaryFields(cb *cinc.Cookbook) []summaryField {
	return []summaryField{
		{"Version", orDash(cb.Version)},
		{"Description", orDash(cb.Metadata.Description)},
		{"Maintainer", orDash(cb.Metadata.Maintainer)},
		{"License", orDash(cb.Metadata.License)},
		{"Files", count(len(cb.AllFiles()))},
	}
}

func (k cookbookVersionsKind) Delete(ctx context.Context, c *cinc.Client, version string) error {
	_, err := c.Cookbooks.Delete(ctx, k.name, version)
	return err
}

func (k cookbookVersionsKind) Download(ctx context.Context, c *cinc.Client, version, destDir string) (string, error) {
	// Download writes the cookbook's manifest files (recipes/…,
	// metadata.rb, …) directly under destDir, so that's the path to
	// report back.
	if err := c.Cookbooks.Download(ctx, k.name, version, destDir); err != nil {
		return "", err
	}
	return destDir, nil
}
