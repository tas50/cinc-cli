package explore

import (
	"context"
	"time"

	cinc "github.com/tas50/cinc-api"
)

// newNodeKind builds the Nodes kind: full view/edit/create/delete.
func newNodeKind() Kind {
	return editorKind[cinc.Node]{
		title: "Nodes",
		summaryFn: func(n *cinc.Node) []summaryField {
			return nodeSummaryFields(n, time.Now())
		},
		listFn: func(ctx context.Context, c *cinc.Client) (map[string]string, error) {
			index, _, err := c.Nodes.List(ctx)
			return index, err
		},
		getFn: func(ctx context.Context, c *cinc.Client, name string) (*cinc.Node, error) {
			n, _, err := c.Nodes.Get(ctx, name)
			return n, err
		},
		createFn: func(ctx context.Context, c *cinc.Client, n *cinc.Node) (CreateResult, error) {
			// The create endpoint returns only a URI, not the object, so
			// report the name from the document the user submitted.
			if _, _, err := c.Nodes.Create(ctx, n); err != nil {
				return CreateResult{}, err
			}
			return CreateResult{Name: n.Name}, nil
		},
		updateFn: func(ctx context.Context, c *cinc.Client, n *cinc.Node) error {
			_, _, err := c.Nodes.Update(ctx, n)
			return err
		},
		deleteFn: func(ctx context.Context, c *cinc.Client, name string) error {
			_, err := c.Nodes.Delete(ctx, name)
			return err
		},
		template: func() []byte {
			return []byte(`{
  "name": "new-node",
  "chef_environment": "_default",
  "run_list": []
}`)
		},
	}
}
