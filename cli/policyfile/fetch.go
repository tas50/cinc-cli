package policyfile

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cinc "github.com/tas50/cinc-api"
	sm "github.com/tas50/cinc-supermarket"
)

// Fetcher locates the cookbooks a Policyfile lock pins, fetching them from the
// source each lock records into a local cache and returning the on-disk
// directory that holds the cookbook root. It is safe to call repeatedly: a
// cached cookbook is returned without re-fetching.
type Fetcher struct {
	// CacheRoot is the directory holding one subdirectory per cache_key for
	// fetched (artifactserver/git) cookbooks.
	CacheRoot string
	// LockDir is the directory of the lock file, used to resolve relative
	// path: sources.
	LockDir string
	// Chef is used for chef_server-sourced cookbooks. It may be nil when no
	// lock uses a chef_server source.
	Chef *cinc.Client

	// newSupermarket builds a Supermarket client for a base URL. Overridable
	// in tests; defaults to a cinc-supermarket anonymous client.
	newSupermarket func(base string) (*sm.Client, error)
}

// DefaultCacheRoot is ~/.cinc/cache/cookbooks.
func DefaultCacheRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("policyfile: locate home directory: %w", err)
	}
	return filepath.Join(home, ".cinc", "cache", "cookbooks"), nil
}

func (f *Fetcher) supermarketClient(base string) (*sm.Client, error) {
	if f.newSupermarket != nil {
		return f.newSupermarket(base)
	}
	return sm.NewClient(sm.Config{BaseURL: base}, sm.WithUserAgent("cinc-cli"))
}

// EnsureCookbook returns the directory containing the cookbook for one lock,
// fetching and caching it first if necessary. path: cookbooks are read in
// place; artifactserver/chef_server cookbooks are cached under CacheRoot keyed
// by the lock's cache_key (a cache hit is a directory-exists check).
func (f *Fetcher) EnsureCookbook(ctx context.Context, name string, lock cinc.CookbookLock) (string, error) {
	kind, value, err := classifySource(lock.SourceOptions)
	if err != nil {
		return "", fmt.Errorf("cookbook %q: %w", name, err)
	}

	if kind == sourcePath {
		dir := value
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(f.LockDir, dir)
		}
		if _, err := os.Stat(dir); err != nil {
			return "", fmt.Errorf("cookbook %q: path source %s: %w", name, dir, err)
		}
		return dir, nil
	}

	if lock.CacheKey == "" {
		return "", fmt.Errorf("cookbook %q: %s source has no cache_key to cache under", name, kind)
	}
	dest := filepath.Join(f.CacheRoot, lock.CacheKey)
	if isDir(dest) {
		return dest, nil // cache hit
	}

	staging := dest + ".partial"
	if err := os.RemoveAll(staging); err != nil {
		return "", err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return "", err
	}
	switch kind {
	case sourceArtifactserver:
		err = f.fetchArtifactserver(ctx, name, value, sourceVersion(lock), staging)
	case sourceChefServer:
		err = f.fetchChefServer(ctx, name, sourceVersion(lock), staging)
	case sourceGit:
		err = f.fetchGit(ctx, lock, value, staging)
	}
	if err != nil {
		_ = os.RemoveAll(staging)
		return "", fmt.Errorf("cookbook %q: %w", name, err)
	}
	if err := os.Rename(staging, dest); err != nil {
		_ = os.RemoveAll(staging)
		return "", fmt.Errorf("cookbook %q: finalize cache entry: %w", name, err)
	}
	return dest, nil
}

// fetchArtifactserver downloads a cookbook tarball from a Supermarket-style
// artifactserver and extracts it into dest. The lock records the full download
// URL; the cinc-supermarket client is pointed at its scheme://host base and
// asked for the cookbook by name and version, reconstructing the same request.
func (f *Fetcher) fetchArtifactserver(ctx context.Context, name, artifactURL, version, dest string) error {
	if version == "" {
		return fmt.Errorf("artifactserver source has no version")
	}
	base, err := baseURL(artifactURL)
	if err != nil {
		return err
	}
	client, err := f.supermarketClient(base)
	if err != nil {
		return err
	}
	body, _, err := client.Cookbooks.Download(ctx, name, version)
	if err != nil {
		return fmt.Errorf("download from %s: %w", base, err)
	}
	defer body.Close()
	return extractCookbookTarball(body, dest)
}

// fetchChefServer downloads a cookbook from the Cinc/Chef server via cinc-api,
// which writes the cookbook's files into dest.
func (f *Fetcher) fetchChefServer(ctx context.Context, name, version, dest string) error {
	if f.Chef == nil {
		return fmt.Errorf("chef_server source requires a configured server connection")
	}
	if version == "" {
		version = "_latest"
	}
	return f.Chef.Cookbooks.Download(ctx, name, version, dest)
}

// fetchGit clones the lock's git repository, checks out the pinned revision,
// and copies the cookbook (the repo root, or the source_options "path"
// subdirectory) into dest, omitting the .git directory. It shells out to the
// `git` binary, which must be installed.
func (f *Fetcher) fetchGit(ctx context.Context, lock cinc.CookbookLock, repoURL, dest string) error {
	revision := stringOption(lock.SourceOptions, "revision")
	if revision == "" {
		revision = stringOption(lock.SourceOptions, "branch")
	}
	if revision == "" {
		return fmt.Errorf("git source has no revision or branch to check out")
	}
	// repoURL and revision come from the (potentially untrusted) lock. Reject
	// values that begin with "-" so they cannot be smuggled in as git flags
	// (argv injection). clone additionally gets a "--" separator; checkout
	// cannot use "--" (it would treat the revision as a pathspec), so the
	// leading-dash rejection is its guard.
	if strings.HasPrefix(repoURL, "-") {
		return fmt.Errorf("refusing git source URL beginning with '-': %q", repoURL)
	}
	if strings.HasPrefix(revision, "-") {
		return fmt.Errorf("refusing git revision beginning with '-': %q", revision)
	}

	clone, err := os.MkdirTemp("", "cinc-git-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(clone)

	if out, err := runGit(ctx, "", "clone", "--quiet", "--", repoURL, clone); err != nil {
		return fmt.Errorf("git clone %s: %w: %s", repoURL, err, out)
	}
	if out, err := runGit(ctx, clone, "checkout", "--quiet", revision); err != nil {
		return fmt.Errorf("git checkout %s: %w: %s", revision, err, out)
	}

	root := clone
	if sub := stringOption(lock.SourceOptions, "path"); sub != "" {
		root = filepath.Join(clone, sub)
	}
	if !hasCookbookMetadata(root) {
		return fmt.Errorf("no cookbook (metadata.rb or metadata.json) found at %q in the git repository", root)
	}
	return copyTree(root, dest)
}

// runGit runs `git args...` (optionally in dir) and returns its combined
// output, so failures can be reported with git's own message.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func hasCookbookMetadata(dir string) bool {
	for _, name := range []string{"metadata.rb", "metadata.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func stringOption(opts map[string]any, key string) string {
	if v, ok := opts[key].(string); ok {
		return v
	}
	return ""
}

const (
	sourcePath           = "path"
	sourceArtifactserver = "artifactserver"
	sourceGit            = "git"
	sourceChefServer     = "chef_server"
)

// classifySource inspects a cookbook lock's source_options and returns the
// source kind and its string value (the path or URL). It checks path first
// (local development cookbooks), then the remote sources.
func classifySource(opts map[string]any) (kind, value string, err error) {
	for _, k := range []string{sourcePath, sourceArtifactserver, sourceGit, sourceChefServer} {
		if v, ok := opts[k]; ok {
			s, ok := v.(string)
			if !ok {
				return "", "", fmt.Errorf("source_options.%s is not a string", k)
			}
			return k, s, nil
		}
	}
	return "", "", fmt.Errorf("unsupported or missing cookbook source in %v", keysOf(opts))
}

// sourceVersion returns the version a lock pins: the source_options "version"
// if present, otherwise the lock's top-level version.
func sourceVersion(lock cinc.CookbookLock) string {
	if v, ok := lock.SourceOptions["version"].(string); ok && v != "" {
		return v
	}
	return lock.Version
}

func baseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid artifactserver URL %q", raw)
	}
	return u.Scheme + "://" + u.Host, nil
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
