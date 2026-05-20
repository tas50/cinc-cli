package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tas50/cinc-cli/cli/config"
	"github.com/tas50/cinc-cli/cli/printer"
)

// newConfigCmd builds the `cinc config` command group.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage local Cinc configuration",
	}
	cmd.AddCommand(newConfigValidateCmd())
	return cmd
}

type configValidationResult struct {
	Path     string                   `json:"path"`
	Valid    bool                     `json:"valid"`
	Profiles int                      `json:"profiles"`
	Issues   []config.ValidationIssue `json:"issues,omitempty"`
}

func newConfigValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a local Cinc TOML configuration file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configValidatePath(cmd, args)
			if err != nil {
				return err
			}
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.Load(path)
			if err != nil {
				result := configValidationResult{
					Path:  path,
					Valid: false,
					Issues: []config.ValidationIssue{{
						Field:   "file",
						Message: err.Error(),
					}},
				}
				if format == printer.FormatJSON {
					_ = printer.New(cmd.OutOrStdout(), format).Value(result)
				}
				return err
			}
			result := configValidationResult{
				Path:     path,
				Valid:    true,
				Profiles: len(cfg.Profiles),
				Issues:   cfg.Validate(),
			}
			result.Valid = len(result.Issues) == 0
			if format == printer.FormatJSON {
				if err := printer.New(cmd.OutOrStdout(), format).Value(result); err != nil {
					return err
				}
			} else {
				printConfigValidation(cmd, result)
			}
			if !result.Valid {
				return fmt.Errorf("config validation failed with %d issue(s)", len(result.Issues))
			}
			return nil
		},
	}
	return cmd
}

func configValidatePath(cmd *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return expandHome(args[0])
	}
	cfgPath, err := configPathForCommand(cmd)
	if err != nil {
		return "", err
	}
	return expandHome(cfgPath)
}

func printConfigValidation(cmd *cobra.Command, result configValidationResult) {
	if result.Valid {
		fmt.Fprintf(cmd.OutOrStdout(), "Config %s is valid (%d profile(s))\n", result.Path, result.Profiles)
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Config %s is invalid (%d issue(s))\n", result.Path, len(result.Issues))
	for _, issue := range result.Issues {
		if issue.Profile != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", issue.Profile, issue.Field, issue.Message)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", issue.Field, issue.Message)
		}
	}
}
