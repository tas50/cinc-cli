package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/components"
	"github.com/tas50/cinc-cli/cli/printer"
)

// newUserCmd builds the `cinc user` command group. Users are global
// Cinc/Chef Server accounts, not scoped to any one organization.
func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage users on the Cinc Server",
	}
	cmd.AddCommand(newUserListCmd())
	cmd.AddCommand(newUserShowCmd())
	cmd.AddCommand(newUserCreateCmd())
	cmd.AddCommand(newUserEditCmd())
	cmd.AddCommand(newUserDeleteCmd())
	cmd.AddCommand(newUserPasswordCmd())
	cmd.AddCommand(newKeyCmd(userKeyOwner))
	return cmd
}

// newUserEditCmd builds the `cinc user edit <name>` command. It fetches the
// user, opens its JSON in the shared editor, and PUTs the result back. The
// path arg pins the username. `--file` reads the updated JSON from disk for
// scripted use.
func newUserEditCmd() *cobra.Command {
	var inputFile string
	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit a user on the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			name := args[0]

			var updated cinc.User
			if inputFile != "" {
				data, err := os.ReadFile(inputFile)
				if err != nil {
					return fmt.Errorf("cinc: read %s: %w", inputFile, err)
				}
				if err := json.Unmarshal(data, &updated); err != nil {
					return fmt.Errorf("cinc: parse %s: %w", inputFile, err)
				}
			} else {
				current, _, err := c.Users.Get(cmd.Context(), name)
				if err != nil {
					return err
				}
				edited, err := editUser(current)
				if err != nil {
					return err
				}
				if reflect.DeepEqual(*current, *edited) {
					fmt.Fprintf(cmd.OutOrStdout(), "User %q unchanged\n", name)
					return nil
				}
				updated = *edited
			}
			updated.UserName = name

			if _, _, err := c.Users.Update(cmd.Context(), &updated); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated user %q\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&inputFile, "file", "", "read the updated user JSON from this file instead of launching the editor")
	return cmd
}

// userCreateFlags collects the editable fields and key options for
// `user create`.
type userCreateFlags struct {
	email         string
	displayName   string
	firstName     string
	middleName    string
	lastName      string
	password      string
	keyFile       string
	publicKeyFile string
}

// newUserCreateCmd builds the `cinc user create <name>` command.
//
// Like `client create`, the server generates an RSA key pair and
// returns the private key, which is streamed to stdout by default or
// written to `--key-file`. Supplying `--public-key` makes the server
// use that key instead and return no private key.
func newUserCreateCmd() *cobra.Command {
	var flags userCreateFlags
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a user on the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			req := &cinc.User{
				UserName:    args[0],
				DisplayName: flags.displayName,
				Email:       flags.email,
				FirstName:   flags.firstName,
				MiddleName:  flags.middleName,
				LastName:    flags.lastName,
				Password:    flags.password,
			}
			if flags.publicKeyFile != "" {
				pem, err := os.ReadFile(flags.publicKeyFile)
				if err != nil {
					return fmt.Errorf("cinc: read public key: %w", err)
				}
				req.PublicKey = string(pem)
			} else {
				req.CreateKey = true
			}
			created, _, err := c.Users.Create(cmd.Context(), req)
			if err != nil {
				return err
			}
			return emitUserCreateResult(cmd, req.UserName, created, flags.keyFile)
		},
	}
	cmd.Flags().StringVar(&flags.email, "email", "", "user's email address")
	cmd.Flags().StringVar(&flags.displayName, "display-name", "", "user's display name")
	cmd.Flags().StringVar(&flags.firstName, "first-name", "", "user's first name")
	cmd.Flags().StringVar(&flags.middleName, "middle-name", "", "user's middle name")
	cmd.Flags().StringVar(&flags.lastName, "last-name", "", "user's last name")
	cmd.Flags().StringVar(&flags.password, "password", "", "user's initial password")
	cmd.Flags().StringVarP(&flags.keyFile, "key-file", "f", "", "write the generated private key to this file instead of stdout")
	cmd.Flags().StringVar(&flags.publicKeyFile, "public-key", "", "path to a PEM public key; the server will not generate a key pair")
	return cmd
}

// emitUserCreateResult renders the response from a successful user
// create, mirroring the client-create behavior: a server-generated
// private key is written to keyFile if given, otherwise streamed to
// stdout; the bring-your-own-public-key path prints a confirmation.
func emitUserCreateResult(cmd *cobra.Command, name string, created *cinc.UserCreateResult, keyFile string) error {
	out := cmd.OutOrStdout()
	priv := created.ChefKey.PrivateKey
	if priv == "" {
		fmt.Fprintf(out, "Created user %q\n", name)
		return nil
	}
	fileMsg := fmt.Sprintf("Created user %q (key written to %s)", name, keyFile)
	return writePrivateKey(out, priv, keyFile, fileMsg)
}

// newUserDeleteCmd builds the `cinc user delete <name>` command.
func newUserDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a user from the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			if _, err := c.Users.Delete(cmd.Context(), name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted user %q\n", name)
			return nil
		},
	}
}

// newUserPasswordCmd builds the `cinc user password <name>` command. It
// fetches the user so the existing metadata survives, sets the new
// password, and PUTs the result back. The password comes from
// `--password` or, if omitted, an interactive prompt.
func newUserPasswordCmd() *cobra.Command {
	var password string
	cmd := &cobra.Command{
		Use:   "password <name>",
		Short: "Set a user's password",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			if password == "" {
				reader := bufio.NewReader(cmd.InOrStdin())
				password, err = components.PromptWithDefault(reader, cmd.OutOrStdout(), "New password", "")
				if err != nil {
					return err
				}
			}
			if password == "" {
				return fmt.Errorf("a non-empty password is required")
			}
			user, _, err := c.Users.Get(cmd.Context(), name)
			if err != nil {
				return err
			}
			user.UserName = name
			user.Password = password
			if _, _, err := c.Users.Update(cmd.Context(), user); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated password for user %q\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&password, "password", "", "the new password (prompted for if omitted)")
	return cmd
}

// newUserShowCmd builds the `cinc user show <name>` command.
func newUserShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			user, _, err := c.Users.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printer.New(cmd.OutOrStdout(), format).Value(user)
		},
	}
}

// newUserListCmd builds the `cinc user list` command.
func newUserListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List users on the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			names, err := fetchUserNames(cmd.Context(), c)
			if err != nil {
				return err
			}
			return printer.New(cmd.OutOrStdout(), format).List(names)
		},
	}
}

// fetchUserNames returns the sorted names of every user on the server.
func fetchUserNames(ctx context.Context, c *cinc.Client) ([]string, error) {
	index, _, err := c.Users.List(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(index))
	for name := range index {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, nil
}
