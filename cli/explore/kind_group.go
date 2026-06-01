package explore

import (
	"context"
	"encoding/json"
	"fmt"

	cinc "github.com/tas50/cinc-api"
)

// groupKind is the Groups kind. Groups are created by name only (a
// NamedCreatable), then edited to add members.
type groupKind struct{}

func newGroupKind() Kind { return groupKind{} }

func (groupKind) Title() string     { return "Groups" }
func (groupKind) Columns() []string { return []string{"NAME"} }

func (groupKind) List(ctx context.Context, c *cinc.Client) ([]Row, error) {
	index, _, err := c.Groups.List(ctx)
	if err != nil {
		return nil, err
	}
	return nameRows(index), nil
}

func (groupKind) Describe(ctx context.Context, c *cinc.Client, name string) (string, error) {
	g, _, err := c.Groups.Get(ctx, name)
	if err != nil {
		return "", err
	}
	return prettyJSON(g)
}

func (groupKind) Save(ctx context.Context, c *cinc.Client, name string, edited []byte) error {
	var g cinc.Group
	if err := json.Unmarshal(edited, &g); err != nil {
		return fmt.Errorf("parse edited group: %w", err)
	}
	// Update builds its URL from Name; the GET response populates
	// GroupName, so pin the known name to address the right group.
	g.Name = name
	_, _, err := c.Groups.Update(ctx, &g)
	return err
}

func (groupKind) CreateNamed(ctx context.Context, c *cinc.Client, name string) (CreateResult, error) {
	if _, err := c.Groups.Create(ctx, name); err != nil {
		return CreateResult{}, err
	}
	return CreateResult{Name: name}, nil
}

func (groupKind) Delete(ctx context.Context, c *cinc.Client, name string) error {
	_, err := c.Groups.Delete(ctx, name)
	return err
}
