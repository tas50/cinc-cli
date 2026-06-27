package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tas50/cinc-cli/cli/policyfile"
	"github.com/tas50/cinc-cli/cli/policyfile/rubyeval"
	"github.com/tas50/cinc-cli/cli/printer"
)

// newPolicyInstallCmd builds `cinc policy install [Policyfile.rb]`. It
// evaluates a Policyfile with the embedded CRuby engine (real Ruby, so any
// dynamic Policyfile works) and writes the evaluation-only portion of a
// Policyfile.lock.json.
func newPolicyInstallCmd() *cobra.Command {
	var outFile string
	cmd := &cobra.Command{
		Use:   "install [Policyfile.rb]",
		Short: "Evaluate a Policyfile.rb and write the evaluated lock",
		Long: "Evaluate a Policyfile.rb and write the evaluated lock.\n\n" +
			"cinc runs your Policyfile through an embedded CRuby engine (CRuby\n" +
			"compiled to WebAssembly, run with no system Ruby and no CGo), so any\n" +
			"valid Ruby works: loops, conditionals, helper methods, ENV, string\n" +
			"interpolation, and require_relative of sibling files all behave just\n" +
			"as they do with chef. The first run downloads a pinned ruby.wasm and\n" +
			"caches it; later runs are offline.\n\n" +
			"What it resolves: this command performs evaluation only. It captures\n" +
			"your name, run_list, named run lists, attributes, and each cookbook's\n" +
			"declared source, and writes them to Policyfile.lock.json. It does NOT\n" +
			"yet solve cookbook versions, fetch cookbooks, or compute cookbook\n" +
			"identifiers — so the lock is not a fully-resolved, push-ready lock.\n" +
			"Those resolution steps are a separate, larger feature.",
		Example: `Evaluate ./Policyfile.rb and write ./Policyfile.lock.json.
cinc policy install

Evaluate a specific Policyfile and print the evaluation as JSON.
cinc policy install path/to/Policyfile.rb --format json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}

			pfPath := "Policyfile.rb"
			if len(args) == 1 {
				pfPath = args[0]
			}
			if _, err := os.Stat(pfPath); err != nil {
				return fmt.Errorf("we couldn't find a Policyfile at %s. Pass its path, or run `cinc policy create <name>` to scaffold one", pfPath)
			}

			eval, err := rubyeval.NewEngine().EvaluateFile(cmd.Context(), pfPath)
			if rubyeval.IsUnavailable(err) {
				return fmt.Errorf("cinc needs the embedded Ruby engine (ruby.wasm) to evaluate a Policyfile, but we couldn't download it: %w.\nOnce you have network access the first run will cache it and later runs work offline", err)
			}
			if err != nil {
				// Evaluation problems (syntax errors, a raise in the file, an
				// empty run_list, …) come back as a normal error carrying the
				// chef-style messages the engine collected.
				return fmt.Errorf("we couldn't evaluate %s:\n%w", pfPath, err)
			}

			lock := policyfile.EvaluationLock(eval)
			lockPath := outFile
			if lockPath == "" {
				lockPath = filepath.Join(filepath.Dir(pfPath), "Policyfile.lock.json")
			}
			lockJSON, err := json.MarshalIndent(lock, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(lockPath, append(lockJSON, '\n'), 0o644); err != nil {
				return fmt.Errorf("cinc: write %s: %w", lockPath, err)
			}

			if format == printer.FormatJSON {
				return printer.New(cmd.OutOrStdout(), format).Value(eval)
			}
			renderInstall(cmd, pfPath, lockPath, eval)
			return nil
		},
	}
	cmd.Flags().StringVar(&outFile, "output", "", "write the lock to this path instead of ./Policyfile.lock.json")
	return cmd
}

// renderInstall prints the human summary of an evaluated Policyfile, leading
// with what was captured and ending with an honest note about the
// evaluation-vs-resolution boundary.
func renderInstall(cmd *cobra.Command, pfPath, lockPath string, eval *rubyeval.EvaluatedPolicy) {
	out := cmd.OutOrStdout()
	name := eval.Name
	if name == "" {
		name = "(unnamed)"
	}
	fmt.Fprintf(out, "Evaluated %s (policy %q)\n\n", pfPath, name)

	if len(eval.RunList) > 0 {
		fmt.Fprintln(out, "  run_list:")
		for _, item := range eval.RunList {
			fmt.Fprintf(out, "    %s\n", item)
		}
	}
	if len(eval.NamedRunLists) > 0 {
		fmt.Fprintln(out, "  named run lists:")
		for _, n := range sortedKeys(eval.NamedRunLists) {
			fmt.Fprintf(out, "    %s: %s\n", n, strings.Join(eval.NamedRunLists[n], ", "))
		}
	}
	if len(eval.Cookbooks) > 0 {
		fmt.Fprintln(out, "  cookbooks:")
		for _, n := range sortedCookbookNames(eval.Cookbooks) {
			spec := eval.Cookbooks[n]
			fmt.Fprintf(out, "    %-20s %-12s %s\n", n, spec.VersionConstraint, cookbookSource(spec.SourceOptions))
		}
	}
	if len(eval.DefaultAttributes) > 0 {
		fmt.Fprintf(out, "  default attributes:  %d top-level key(s)\n", len(eval.DefaultAttributes))
	}
	if len(eval.OverrideAttributes) > 0 {
		fmt.Fprintf(out, "  override attributes: %d top-level key(s)\n", len(eval.OverrideAttributes))
	}

	fmt.Fprintf(out, "\nWrote %s.\n", lockPath)
	if unresolved := policyfile.UnresolvedCookbooks(eval); len(unresolved) > 0 {
		slices.Sort(unresolved)
		fmt.Fprintf(out, "\nHeads up: this is an evaluation-only lock. cinc captured your run_list,\n"+
			"attributes, and cookbook sources, but it has not resolved cookbook\n"+
			"versions or identifiers or fetched any cookbooks (%s), so the lock\n"+
			"isn't ready to `cinc policy push` yet. Full dependency resolution is a\n"+
			"separate feature still in progress.\n", strings.Join(unresolved, ", "))
	}
}

// cookbookSource describes where a cookbook comes from, for the human summary.
func cookbookSource(opts map[string]any) string {
	if opts == nil {
		return "(default source)"
	}
	if p, ok := opts["path"].(string); ok {
		return "path " + p
	}
	if g, ok := opts["git"].(string); ok {
		if ref := firstString(opts, "ref", "branch", "tag"); ref != "" {
			return "git " + g + " @ " + ref
		}
		return "git " + g
	}
	if len(opts) == 0 {
		return "(default source)"
	}
	return "(custom source)"
}

func firstString(opts map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := opts[k].(string); ok {
			return v
		}
	}
	return ""
}

func sortedKeys(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedCookbookNames(m map[string]rubyeval.CookbookSpec) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
