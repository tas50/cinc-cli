package explore

import (
	"context"

	cinc "github.com/tas50/cinc-api"
)

// newClientKind builds the Clients kind. Creating a client may return a
// one-time private key, surfaced through CreateResult.Secret.
func newClientKind() Kind {
	return editorKind[cinc.APIClient]{
		title: "Clients",
		listFn: func(ctx context.Context, c *cinc.Client) (map[string]string, error) {
			index, _, err := c.Clients.List(ctx)
			return index, err
		},
		getFn: func(ctx context.Context, c *cinc.Client, name string) (*cinc.APIClient, error) {
			cl, _, err := c.Clients.Get(ctx, name)
			return cl, err
		},
		createFn: func(ctx context.Context, c *cinc.Client, cl *cinc.APIClient) (CreateResult, error) {
			// The response carries the generated key but not the name, so
			// take the name from the submitted document.
			created, _, err := c.Clients.Create(ctx, cl)
			if err != nil {
				return CreateResult{}, err
			}
			return CreateResult{Name: cl.Name, Secret: created.ChefKey.PrivateKey}, nil
		},
		updateFn: func(ctx context.Context, c *cinc.Client, cl *cinc.APIClient) error {
			_, _, err := c.Clients.Update(ctx, cl)
			return err
		},
		deleteFn: func(ctx context.Context, c *cinc.Client, name string) error {
			_, err := c.Clients.Delete(ctx, name)
			return err
		},
		template: func() []byte {
			return []byte(`{
  "name": "new-client",
  "validator": false
}`)
		},
	}
}
