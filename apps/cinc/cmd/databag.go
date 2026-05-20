package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/printer"
)

// newDataBagCmd builds the `cinc data-bag` command group.
func newDataBagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data-bag",
		Short: "Manage data bags on the Cinc/Chef Server",
	}
	cmd.AddCommand(newDataBagListCmd())
	cmd.AddCommand(newDataBagDeleteCmd())
	cmd.AddCommand(newDataBagItemCmd())
	return cmd
}

// newDataBagItemCmd builds the `cinc data-bag item` command group.
// Data bag items are arbitrary JSON documents (always carrying an
// "id" key) so the item-level verbs live under their own subgroup.
func newDataBagItemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "item",
		Short: "Manage items within a data bag",
	}
	cmd.AddCommand(newDataBagItemEditCmd())
	return cmd
}

// newDataBagItemEditCmd builds `cinc data-bag item edit <bag> <id>`.
// It fetches the item, opens its JSON in the built-in editor (same
// engine as `cinc client edit`), validates on save, and PUTs the
// result back. `--file` reads the updated JSON from disk for
// scripted use. The path arg's id pins the item identifier so an
// edit can never accidentally rename the item out from under itself.
func newDataBagItemEditCmd() *cobra.Command {
	var inputFile string
	cmd := &cobra.Command{
		Use:   "edit <bag> <id>",
		Short: "Edit a data bag item on the server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			bag, id := args[0], args[1]
			items := c.DataBags.Items(bag)

			var updated cinc.DataBagItem
			if inputFile != "" {
				data, err := os.ReadFile(inputFile)
				if err != nil {
					return fmt.Errorf("cinc: read %s: %w", inputFile, err)
				}
				if err := json.Unmarshal(data, &updated); err != nil {
					return fmt.Errorf("cinc: parse %s: %w", inputFile, err)
				}
				if err := validateDataBagItem(data); err != nil {
					return err
				}
			} else {
				current, _, err := items.Get(cmd.Context(), id)
				if err != nil {
					return err
				}
				edited, err := editDataBagItem(current)
				if err != nil {
					return err
				}
				if reflect.DeepEqual(current, edited) {
					fmt.Fprintf(cmd.OutOrStdout(), "Item %q in bag %q unchanged\n", id, bag)
					return nil
				}
				updated = edited
			}
			updated["id"] = id

			if _, _, err := items.Update(cmd.Context(), updated); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated item %q in bag %q\n", id, bag)
			return nil
		},
	}
	cmd.Flags().StringVar(&inputFile, "file", "", "read the updated item JSON from this file instead of launching the editor")
	return cmd
}

// validateDataBagItem checks that b is valid JSON and contains a
// non-empty string "id" key. It is exported via the editor seam.
func validateDataBagItem(b []byte) error {
	var item cinc.DataBagItem
	if err := json.Unmarshal(b, &item); err != nil {
		return err
	}
	if item.ID() == "" {
		return errors.New("data bag item is missing a non-empty \"id\" field")
	}
	return nil
}

// newDataBagDeleteCmd builds the `cinc data-bag delete <name>` command.
func newDataBagDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a data bag from the server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			if _, err := c.DataBags.Delete(cmd.Context(), name); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted data bag %q\n", name)
			return nil
		},
	}
}

// newDataBagListCmd builds the `cinc data-bag list` command.
func newDataBagListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List data bags on the server",
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
			names, err := fetchDataBagNames(cmd.Context(), c)
			if err != nil {
				return err
			}
			return printer.New(cmd.OutOrStdout(), format).List(names)
		},
	}
}

// fetchDataBagNames returns the sorted names of every data bag on the
// server.
func fetchDataBagNames(ctx context.Context, c *cinc.Client) ([]string, error) {
	index, _, err := c.DataBags.List(ctx)
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
