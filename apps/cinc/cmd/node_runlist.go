package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tas50/cinc-cli/cli/printer"
)

// newNodeRunListCmd builds the `cinc node run-list` sub-group, which mutates a
// node's run list in place: add appends new entries, remove drops matching
// ones, and set replaces the whole list. Each verb fetches the node, applies
// the change, and PUTs it back, mirroring knife's `node run_list` verbs.
func newNodeRunListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run-list",
		Short: "Add, remove, or set a node's run list",
	}
	cmd.AddCommand(newNodeRunListChangeCmd("add"))
	cmd.AddCommand(newNodeRunListChangeCmd("remove"))
	cmd.AddCommand(newNodeRunListChangeCmd("set"))
	return cmd
}

// newNodeRunListChangeCmd builds one of the run-list verbs. Items may be given
// as separate args or comma-separated within an arg (e.g. `recipe[a],role[b]`),
// matching how the other run-list-bearing commands accept them.
func newNodeRunListChangeCmd(verb string) *cobra.Command {
	short := map[string]string{
		"add":    "Append entries to a node's run list",
		"remove": "Remove entries from a node's run list",
		"set":    "Replace a node's run list",
	}[verb]
	return &cobra.Command{
		Use:   verb + " <node> <entry>...",
		Short: short,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			name := args[0]
			items := gatherCSVArgs(args[1:])

			node, _, err := c.Nodes.Get(cmd.Context(), name)
			if err != nil {
				return err
			}
			switch verb {
			case "add":
				node.RunList = appendNew(node.RunList, items)
			case "remove":
				node.RunList = removeItems(node.RunList, items)
			case "set":
				node.RunList = items
			}
			if node.RunList == nil {
				node.RunList = []string{}
			}
			node.Name = name
			if _, _, err := c.Nodes.Update(cmd.Context(), node); err != nil {
				return err
			}
			return emitRunList(cmd, format, name, node.RunList)
		},
	}
}

// emitRunList reports a node's run list after a change: the raw slice under
// --format json, or a human line listing the entries.
func emitRunList(cmd *cobra.Command, format printer.Format, name string, runList []string) error {
	if format == printer.FormatJSON {
		return printer.New(cmd.OutOrStdout(), format).Value(runList)
	}
	if len(runList) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Run list for node %q is now empty\n", name)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Run list for node %q is now: %s\n", name, strings.Join(runList, ", "))
	return nil
}

// appendNew returns base with every item that is not already present appended
// in order, leaving existing entries (and their positions) untouched.
func appendNew(base, items []string) []string {
	out := slices.Clone(base)
	for _, item := range items {
		if !slices.Contains(out, item) {
			out = append(out, item)
		}
	}
	return out
}

// removeItems returns base with every entry equal to one of items dropped.
func removeItems(base, items []string) []string {
	out := make([]string, 0, len(base))
	for _, entry := range base {
		if !slices.Contains(items, entry) {
			out = append(out, entry)
		}
	}
	return out
}

// gatherCSVArgs flattens args that may each be a single entry or a
// comma-separated list of entries into one ordered slice.
func gatherCSVArgs(args []string) []string {
	var out []string
	for _, arg := range args {
		out = append(out, splitCSV(arg)...)
	}
	return out
}
