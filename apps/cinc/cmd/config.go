package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	cliclient "github.com/tas50/cinc-cli/cli/client"
	"github.com/tas50/cinc-cli/cli/config"
	"github.com/tas50/cinc-cli/cli/printer"
)

// newConfigCmd builds the `cinc config` command group.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage local Cinc configuration",
	}
	cmd.AddCommand(newConfigureCmd())
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
		Short: "Validate local Cinc TOML configuration and endpoint reachability",
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
			result.Issues = append(result.Issues, preflightConfig(cmd.Context(), cfg, result.Issues)...)
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

func preflightConfig(ctx context.Context, cfg *config.Config, localIssues []config.ValidationIssue) []config.ValidationIssue {
	skip := profilesWithValidationIssues(localIssues)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var issues []config.ValidationIssue
	for name, profile := range cfg.Profiles {
		if skip[name] {
			continue
		}
		issues = append(issues, preflightProfile(ctx, name, profile)...)
	}
	return issues
}

func profilesWithValidationIssues(issues []config.ValidationIssue) map[string]bool {
	profiles := make(map[string]bool)
	for _, issue := range issues {
		if issue.Profile != "" {
			profiles[issue.Profile] = true
		}
	}
	return profiles
}

func preflightProfile(ctx context.Context, name string, profile config.Profile) []config.ValidationIssue {
	var issues []config.ValidationIssue
	if profile.ServerURL != "" && profile.Org != "" {
		c, err := cliclient.New(profile)
		if err != nil {
			issues = append(issues, config.ValidationIssue{
				Profile: name,
				Field:   "cinc_server_url",
				Message: "preflight failed: " + err.Error(),
			})
		} else if _, _, err := c.Clients.List(ctx); err != nil {
			issues = append(issues, config.ValidationIssue{
				Profile: name,
				Field:   "cinc_server_url",
				Message: "preflight failed: " + err.Error(),
			})
		}
	}
	if profile.SupermarketSite != "" {
		if err := preflightSupermarket(ctx, profile.SupermarketSite); err != nil {
			issues = append(issues, config.ValidationIssue{
				Profile: name,
				Field:   "supermarket_site",
				Message: "preflight failed: " + err.Error(),
			})
		}
	}
	return issues
}

func preflightSupermarket(ctx context.Context, site string) error {
	endpoint := strings.TrimRight(site, "/") + "/api/v1/cookbooks"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("GET %s: %s", endpoint, resp.Status)
	}
	return nil
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
