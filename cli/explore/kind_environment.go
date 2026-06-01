package explore

import (
	"context"

	cinc "github.com/tas50/cinc-api"
)

// newEnvironmentKind builds the Environments kind: full
// view/edit/create/delete.
func newEnvironmentKind() Kind {
	return editorKind[cinc.Environment]{
		title: "Environments",
		listFn: func(ctx context.Context, c *cinc.Client) (map[string]string, error) {
			index, _, err := c.Environments.List(ctx)
			return index, err
		},
		getFn: func(ctx context.Context, c *cinc.Client, name string) (*cinc.Environment, error) {
			e, _, err := c.Environments.Get(ctx, name)
			return e, err
		},
		createFn: func(ctx context.Context, c *cinc.Client, e *cinc.Environment) (CreateResult, error) {
			// The create endpoint returns only a URI, so report the name
			// from the submitted document.
			if _, _, err := c.Environments.Create(ctx, e); err != nil {
				return CreateResult{}, err
			}
			return CreateResult{Name: e.Name}, nil
		},
		updateFn: func(ctx context.Context, c *cinc.Client, e *cinc.Environment) error {
			_, _, err := c.Environments.Update(ctx, e)
			return err
		},
		deleteFn: func(ctx context.Context, c *cinc.Client, name string) error {
			_, err := c.Environments.Delete(ctx, name)
			return err
		},
		template: func() []byte {
			return []byte(`{
  "name": "new-environment",
  "description": ""
}`)
		},
	}
}
