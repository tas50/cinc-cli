package explore

import (
	"context"

	cinc "github.com/tas50/cinc-api"
)

// newUserKind builds the Users kind. Creating a user with create_key
// returns a one-time private key, surfaced through CreateResult.Secret.
func newUserKind() Kind {
	return editorKind[cinc.User]{
		title:     "Users",
		summaryFn: userSummaryFields,
		listFn: func(ctx context.Context, c *cinc.Client) (map[string]string, error) {
			index, _, err := c.Users.List(ctx)
			return index, err
		},
		getFn: func(ctx context.Context, c *cinc.Client, name string) (*cinc.User, error) {
			u, _, err := c.Users.Get(ctx, name)
			return u, err
		},
		createFn: func(ctx context.Context, c *cinc.Client, u *cinc.User) (CreateResult, error) {
			result, _, err := c.Users.Create(ctx, u)
			if err != nil {
				return CreateResult{}, err
			}
			return CreateResult{Name: u.UserName, Secret: result.ChefKey.PrivateKey}, nil
		},
		updateFn: func(ctx context.Context, c *cinc.Client, u *cinc.User) error {
			_, _, err := c.Users.Update(ctx, u)
			return err
		},
		deleteFn: func(ctx context.Context, c *cinc.Client, name string) error {
			_, err := c.Users.Delete(ctx, name)
			return err
		},
		template: func() []byte {
			return []byte(`{
  "username": "new-user",
  "display_name": "New User",
  "email": "new-user@example.com",
  "first_name": "New",
  "last_name": "User",
  "password": "change-me-please",
  "create_key": true
}`)
		},
	}
}
