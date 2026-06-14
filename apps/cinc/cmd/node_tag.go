package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/printer"
)

// newNodeTagCmd builds the `cinc node tag` sub-group. Chef stores a node's
// tags as a string array under its normal attributes (`normal.tags`); add,
// remove, and set mutate that array and PUT the node back, while list reads it
// without modifying the server. knife exposes add/remove/list; set rounds out
// the sub-noun with a wholesale replace.
func newNodeTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Add, remove, set, or list a node's tags",
	}
	cmd.AddCommand(newNodeTagChangeCmd("add"))
	cmd.AddCommand(newNodeTagChangeCmd("remove"))
	cmd.AddCommand(newNodeTagChangeCmd("set"))
	cmd.AddCommand(newNodeTagListCmd())
	return cmd
}

func newNodeTagChangeCmd(verb string) *cobra.Command {
	short := map[string]string{
		"add":    "Add tags to a node",
		"remove": "Remove tags from a node",
		"set":    "Replace a node's tags",
	}[verb]
	return &cobra.Command{
		Use:   verb + " <node> <tag>...",
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
			tags := gatherCSVArgs(args[1:])

			node, _, err := c.Nodes.Get(cmd.Context(), name)
			if err != nil {
				return err
			}
			switch verb {
			case "add":
				setNodeTags(node, appendNew(nodeTags(node), tags))
			case "remove":
				setNodeTags(node, removeItems(nodeTags(node), tags))
			case "set":
				setNodeTags(node, tags)
			}
			node.Name = name
			if node.RunList == nil {
				node.RunList = []string{}
			}
			if _, _, err := c.Nodes.Update(cmd.Context(), node); err != nil {
				return err
			}
			return emitNodeTags(cmd, format, name, nodeTags(node))
		},
	}
}

func newNodeTagListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <node>",
		Short: "List a node's tags",
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
			node, _, err := c.Nodes.Get(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			tags := nodeTags(node)
			if format == printer.FormatJSON {
				return printer.New(cmd.OutOrStdout(), format).Value(tags)
			}
			return printer.New(cmd.OutOrStdout(), format).List(tags)
		},
	}
}

// emitNodeTags reports a node's tags after a change: the raw slice under
// --format json, or a human line.
func emitNodeTags(cmd *cobra.Command, format printer.Format, name string, tags []string) error {
	if format == printer.FormatJSON {
		return printer.New(cmd.OutOrStdout(), format).Value(tags)
	}
	if len(tags) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Node %q now has no tags\n", name)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Node %q tags are now: %s\n", name, strings.Join(tags, ", "))
	return nil
}

// nodeTags reads a node's tags from its normal attributes, tolerating the
// JSON-decoded shapes the value can take ([]string or []any of strings) and
// returning them as a sorted-by-original-order string slice.
func nodeTags(node *cinc.Node) []string {
	raw, ok := node.Normal["tags"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return slices.Clone(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// setNodeTags writes tags back to a node's normal attributes, allocating the
// normal map if the node had no normal attributes yet.
func setNodeTags(node *cinc.Node, tags []string) {
	if node.Normal == nil {
		node.Normal = cinc.Attributes{}
	}
	node.Normal["tags"] = tags
}
