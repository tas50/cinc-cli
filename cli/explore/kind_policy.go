package explore

import (
	"context"
	"fmt"
	"sort"

	cinc "github.com/tas50/cinc-api"
)

// policyKind is the top-level Policies kind. A policy is a named set of
// revisions; you drill into it to reach individual revisions. The
// policy itself can be deleted (removing all revisions).
type policyKind struct{}

func (policyKind) Title() string     { return "Policies" }
func (policyKind) Columns() []string { return []string{"NAME", "REVISIONS"} }

func (policyKind) List(ctx context.Context, c *cinc.Client) ([]Row, error) {
	index, _, err := c.Policies.List(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(index))
	for name, entry := range index {
		rows = append(rows, Row{
			Name:  name,
			Cells: []string{name, fmt.Sprintf("%d", len(entry.Revisions))},
		})
	}
	sortRows(rows)
	return rows, nil
}

func (policyKind) Delete(ctx context.Context, c *cinc.Client, name string) error {
	_, err := c.Policies.Delete(ctx, name)
	return err
}

func (policyKind) Child(parent string) Kind { return policyRevisionsKind{policy: parent} }

// policyRevisionsKind lists the revisions of one policy. A revision can
// be viewed or deleted.
type policyRevisionsKind struct{ policy string }

func (k policyRevisionsKind) Title() string   { return k.policy }
func (policyRevisionsKind) Columns() []string { return []string{"REVISION"} }

func (k policyRevisionsKind) List(ctx context.Context, c *cinc.Client) ([]Row, error) {
	revs, _, err := c.Policies.Get(ctx, k.policy)
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(revs.Revisions))
	for id := range revs.Revisions {
		rows = append(rows, Row{Name: id, Cells: []string{id}})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows, nil
}

func (k policyRevisionsKind) Describe(ctx context.Context, c *cinc.Client, revision string) (string, error) {
	rev, _, err := c.Policies.GetRevision(ctx, k.policy, revision)
	if err != nil {
		return "", err
	}
	return prettyJSON(rev)
}

func (k policyRevisionsKind) Delete(ctx context.Context, c *cinc.Client, revision string) error {
	_, err := c.Policies.DeleteRevision(ctx, k.policy, revision)
	return err
}
