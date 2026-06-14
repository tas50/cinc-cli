package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	cinc "github.com/tas50/cinc-api"

	"github.com/tas50/cinc-cli/cli/policyfile"
	"github.com/tas50/cinc-cli/cli/printer"
)

const defaultLockFile = "Policyfile.lock.json"

// newPolicyPushCmd builds `cinc policy push <group> [lock]`. It deploys an
// existing Policyfile.lock.json: it fetches every cookbook the lock pins (from
// the source the lock records, into the cinc cache), uploads each as a cookbook
// artifact, and associates the revision with the named policy group.
func newPolicyPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push <group> [lock]",
		Short: "Deploy a Policyfile lock to a policy group",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			group := args[0]
			lockPath := defaultLockFile
			if len(args) == 2 {
				lockPath = args[1]
			}

			lock, lockJSON, err := cinc.LoadPolicyfileLock(lockPath)
			if err != nil {
				return err
			}
			fetcher, err := newFetcher(filepath.Dir(lockPath), c)
			if err != nil {
				return err
			}
			cookbooks, err := fetchLockCookbooks(cmd.Context(), fetcher, lock)
			if err != nil {
				return err
			}
			rev, _, err := c.Policies.PushRevision(cmd.Context(), lockJSON, group, cookbooks)
			if err != nil {
				return err
			}
			return emitPushResult(cmd, format, lock.Name, group, rev, len(cookbooks))
		},
	}
	return cmd
}

// newPolicyExportCmd builds `cinc policy export [lock] [dir]`. It assembles a
// standalone bundle (cookbooks + lock + a local-mode client config) for an
// air-gapped `cinc-client -z`. A server connection is only needed when the lock
// has chef_server-sourced cookbooks.
func newPolicyExportCmd() *cobra.Command {
	var archive bool
	cmd := &cobra.Command{
		Use:   "export [lock] [dir]",
		Short: "Assemble a standalone bundle from a Policyfile lock",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			lockPath := defaultLockFile
			if len(args) >= 1 {
				lockPath = args[0]
			}
			lock, lockJSON, err := cinc.LoadPolicyfileLock(lockPath)
			if err != nil {
				return err
			}
			destDir := lock.Name
			if len(args) == 2 {
				destDir = args[1]
			}

			// Export only needs the server for chef_server cookbook sources, so
			// a missing/unusable config is tolerated here (nil Chef client).
			chef, _ := resolveClient(cmd)
			fetcher, err := newFetcher(filepath.Dir(lockPath), chef)
			if err != nil {
				return err
			}
			result, err := policyfile.Export(cmd.Context(), fetcher, lock, lockJSON, destDir, archive)
			if err != nil {
				return err
			}
			if format == printer.FormatJSON {
				return printer.New(cmd.OutOrStdout(), format).Value(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported policy %q to %s\n", result.Policy, result.Dir)
			if result.Archive != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Wrote archive %s\n", result.Archive)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&archive, "archive", "a", false, "also write the bundle as a .tar.gz archive")
	return cmd
}

// newFetcher builds a policyfile.Fetcher rooted at the default cinc cookbook
// cache, resolving path sources relative to lockDir and using chef for
// chef_server sources.
func newFetcher(lockDir string, chef *cinc.Client) (*policyfile.Fetcher, error) {
	cacheRoot, err := policyfile.DefaultCacheRoot()
	if err != nil {
		return nil, err
	}
	return &policyfile.Fetcher{CacheRoot: cacheRoot, LockDir: lockDir, Chef: chef}, nil
}

// fetchLockCookbooks resolves every cookbook the lock pins to an uploadable
// LocalCookbook, fetching+caching as needed.
func fetchLockCookbooks(ctx context.Context, fetcher *policyfile.Fetcher, lock *cinc.PolicyRevision) (map[string]*cinc.LocalCookbook, error) {
	cookbooks := make(map[string]*cinc.LocalCookbook, len(lock.CookbookLocks))
	for name, cl := range lock.CookbookLocks {
		dir, err := fetcher.EnsureCookbook(ctx, name, cl)
		if err != nil {
			return nil, err
		}
		cb, err := cinc.LocalCookbookFromDir(dir, cl.Version)
		if err != nil {
			return nil, fmt.Errorf("cinc: load cookbook %q from %s: %w", name, dir, err)
		}
		cookbooks[name] = cb
	}
	return cookbooks, nil
}

func emitPushResult(cmd *cobra.Command, format printer.Format, policy, group string, rev *cinc.PolicyRevision, uploaded int) error {
	if format == printer.FormatJSON {
		return printer.New(cmd.OutOrStdout(), format).Value(map[string]any{
			"policy":             policy,
			"group":              group,
			"revision_id":        rev.RevisionID,
			"cookbooks_uploaded": uploaded,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Pushed policy %q (revision %s) to group %q with %d cookbook(s)\n",
		policy, rev.RevisionID, group, uploaded)
	return nil
}
