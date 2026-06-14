package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"sort"
	"time"

	cinc "github.com/tas50/cinc-api"
	"golang.org/x/term"

	cliclient "github.com/tas50/cinc-cli/cli/client"
	"github.com/tas50/cinc-cli/cli/config"
	"github.com/tas50/cinc-cli/cli/supermarket"
)

// resolveHost looks up a hostname; it is a package variable so the DNS check is
// testable without real network access.
var resolveHost = net.DefaultResolver.LookupHost

// checkResult is the outcome of one named pre-flight check.
type checkResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"` // failure reason, shown on a failed check
}

// profileResult collects one profile's check results and whether they all
// passed.
type profileResult struct {
	Name   string        `json:"name"`
	Valid  bool          `json:"valid"`
	Checks []checkResult `json:"checks"`
}

// configValidationResult is the full report: file-level (top-level) checks plus
// a per-profile breakdown.
type configValidationResult struct {
	Path     string          `json:"path"`
	Valid    bool            `json:"valid"`
	TopLevel []checkResult   `json:"top_level_checks"`
	Profiles []profileResult `json:"profiles,omitempty"`
}

// profileCheck is one named per-profile pre-flight check. applies (when set)
// gates whether the check runs for a given profile; inapplicable checks are
// omitted from the report rather than shown as passing or failing.
type profileCheck struct {
	name    string
	applies func(config.Profile) bool
	run     func(ctx context.Context, p config.Profile) (passed bool, detail string)
}

// profileChecks is the ordered registry of per-profile pre-flight checks, each
// with a friendly name. Structural checks come first (instant), then the
// network checks.
var profileChecks = []profileCheck{
	{
		name: "Client name is set",
		run: func(_ context.Context, p config.Profile) (bool, string) {
			return p.ClientName != "", "client_name is required"
		},
	},
	{
		name: "Client key is configured",
		run: func(_ context.Context, p config.Profile) (bool, string) {
			return p.KeyPath != "", "client_key is required"
		},
	},
	{
		name: "An endpoint is configured",
		run: func(_ context.Context, p config.Profile) (bool, string) {
			if p.ServerURL == "" && p.Org == "" && p.SupermarketSite == "" {
				return false, "configure cinc_server_url, chef_server_url, or supermarket_site"
			}
			return true, ""
		},
	},
	{
		name:    "Server URL includes /organizations/<org>",
		applies: func(p config.Profile) bool { return p.ServerURL != "" || p.Org != "" },
		run: func(_ context.Context, p config.Profile) (bool, string) {
			if (p.ServerURL == "") != (p.Org == "") {
				return false, "server URL must include the /organizations/<org> segment"
			}
			return true, ""
		},
	},
	{
		name:    "Server URL is valid",
		applies: func(p config.Profile) bool { return p.ServerURL != "" },
		run: func(_ context.Context, p config.Profile) (bool, string) {
			if _, err := parseServerHost(p.ServerURL); err != nil {
				return false, err.Error()
			}
			return true, ""
		},
	},
	{
		name:    "Supermarket site URL is valid",
		applies: func(p config.Profile) bool { return p.SupermarketSite != "" },
		run: func(_ context.Context, p config.Profile) (bool, string) {
			if err := config.ValidateSiteURL(p.SupermarketSite); err != nil {
				return false, err.Error()
			}
			return true, ""
		},
	},
	{
		name:    "ssl_verify_mode is valid",
		applies: func(p config.Profile) bool { return p.SSLVerifyMode != "" },
		run: func(_ context.Context, p config.Profile) (bool, string) {
			if p.SSLVerifyMode != ":verify_peer" && p.SSLVerifyMode != ":verify_none" {
				return false, "ssl_verify_mode must be :verify_peer or :verify_none"
			}
			return true, ""
		},
	},
	{
		name:    "Client key file is readable",
		applies: func(p config.Profile) bool { return p.KeyPath != "" },
		run: func(_ context.Context, p config.Profile) (bool, string) {
			if _, err := cinc.LoadKeyFile(p.KeyPath); err != nil {
				return false, err.Error()
			}
			return true, ""
		},
	},
	{
		name:    "Server is reachable",
		applies: func(p config.Profile) bool { return p.ServerURL != "" && p.Org != "" },
		run: func(ctx context.Context, p config.Profile) (bool, string) {
			// Resolve DNS first so a name-resolution failure is reported
			// distinctly from a connection failure, then try to connect.
			if host, err := parseServerHost(p.ServerURL); err == nil {
				if _, err := resolveHost(ctx, host); err != nil {
					return false, fmt.Sprintf("cannot resolve host %q: %v", host, err)
				}
			}
			c, err := cliclient.New(p)
			if err != nil {
				return false, err.Error()
			}
			if _, _, err := c.Clients.List(ctx); err != nil {
				return false, err.Error()
			}
			return true, ""
		},
	},
	{
		name:    "Supermarket is reachable",
		applies: func(p config.Profile) bool { return p.SupermarketSite != "" },
		run: func(ctx context.Context, p config.Profile) (bool, string) {
			client, err := supermarket.NewAnonymous(p.SupermarketSite)
			if err != nil {
				return false, err.Error()
			}
			if err := client.Reachable(ctx); err != nil {
				return false, err.Error()
			}
			return true, ""
		},
	},
}

// runConfigChecks evaluates the top-level and per-profile checks for a loaded
// config. Network checks share a 10s budget.
func runConfigChecks(ctx context.Context, path string, cfg *config.Config) configValidationResult {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	hasProfiles := len(cfg.Profiles) > 0
	result := configValidationResult{
		Path: path,
		TopLevel: []checkResult{
			{Name: "Credentials file is valid TOML", Passed: true},
			{
				Name:   "At least one profile is defined",
				Passed: hasProfiles,
				Detail: cond(hasProfiles, "", "the credentials file defines no profiles"),
			},
		},
	}

	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := cfg.Profiles[name]
		pr := profileResult{Name: name, Valid: true}
		for _, def := range profileChecks {
			if def.applies != nil && !def.applies(p) {
				continue
			}
			passed, detail := def.run(ctx, p)
			if passed {
				detail = "" // the detail is a failure reason; never attach it to a pass
			}
			pr.Checks = append(pr.Checks, checkResult{Name: def.name, Passed: passed, Detail: detail})
			if !passed {
				pr.Valid = false
			}
		}
		result.Profiles = append(result.Profiles, pr)
	}

	result.Valid = allPassed(result.TopLevel)
	for _, pr := range result.Profiles {
		if !pr.Valid {
			result.Valid = false
		}
	}
	return result
}

// parseServerHost validates that serverURL is a well-formed http(s) URL and
// returns its hostname (without port).
func parseServerHost(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", fmt.Errorf("not a valid http(s) server URL: %q", serverURL)
	}
	return u.Hostname(), nil
}

func allPassed(checks []checkResult) bool {
	for _, c := range checks {
		if !c.Passed {
			return false
		}
	}
	return true
}

func cond(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}

// renderConfigChecks writes the human-readable check report: an overall line,
// the top-level checks, a blank line, then each profile tagged [VALID]/[INVALID]
// with its checks. ✓/✗ and the tags are colorized when the writer is a terminal
// and NO_COLOR is unset.
func renderConfigChecks(w io.Writer, result configValidationResult) {
	color := useColor(w)
	state := "valid"
	if !result.Valid {
		state = "invalid"
	}
	fmt.Fprintf(w, "Config %s is %s\n", result.Path, state)
	for _, c := range result.TopLevel {
		writeCheckLine(w, color, "", c)
	}
	for _, pr := range result.Profiles {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s profile %s\n", pr.Name, validTag(color, pr.Valid))
		for _, c := range pr.Checks {
			writeCheckLine(w, color, "  ", c)
		}
	}
}

func writeCheckLine(w io.Writer, color bool, indent string, c checkResult) {
	fmt.Fprintf(w, "%s%s %s", indent, mark(color, c.Passed), c.Name)
	if !c.Passed && c.Detail != "" {
		fmt.Fprintf(w, ": %s", c.Detail)
	}
	fmt.Fprintln(w)
}

const (
	ansiGreen = "\x1b[32m"
	ansiRed   = "\x1b[31m"
	ansiReset = "\x1b[0m"
)

func colorize(on bool, color, s string) string {
	if !on {
		return s
	}
	return color + s + ansiReset
}

func mark(on, passed bool) string {
	if passed {
		return colorize(on, ansiGreen, "✓") // ✓
	}
	return colorize(on, ansiRed, "✗") // ✗
}

func validTag(on, valid bool) string {
	if valid {
		return colorize(on, ansiGreen, "[VALID]")
	}
	return colorize(on, ansiRed, "[INVALID]")
}

// useColor reports whether ANSI color should be emitted: only when NO_COLOR is
// unset and the writer is a terminal (so piped output and tests stay plain).
func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
