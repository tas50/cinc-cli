//go:build acceptance

package acceptance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// cincPackage is the import path of the cinc binary's main package.
const cincPackage = "github.com/tas50/cinc-cli/apps/cinc"

// cincZeroVersion is the pinned cinc-zero release the acceptance suite runs
// against. cinc-zero is a single-binary, in-memory Chef Infra Server published
// at https://github.com/tas50/cinc-zero/releases.
const cincZeroVersion = "v0.4.0"

// cincZeroPlatforms lists the GOOS_GOARCH targets cinc-zero publishes a binary
// for. The suite skips on anything else rather than failing.
var cincZeroPlatforms = map[string]bool{
	"darwin_amd64": true,
	"darwin_arm64": true,
	"linux_amd64":  true,
	"linux_arm64":  true,
}

// freePort reserves and returns an unused TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// cincZeroBinary returns the path to the cinc-zero binary, downloading and
// caching the pinned release once per machine. Set CINC_ZERO_BIN to use a local
// build instead (handy offline or when testing an unreleased cinc-zero). The
// suite skips on platforms cinc-zero does not publish a binary for.
var (
	cincZeroOnce sync.Once
	cincZeroPath string
	cincZeroErr  error
	cincZeroSkip string
)

func cincZeroBinary(t *testing.T) string {
	t.Helper()
	cincZeroOnce.Do(func() {
		if local := os.Getenv("CINC_ZERO_BIN"); local != "" {
			cincZeroPath = local
			return
		}
		platform := runtime.GOOS + "_" + runtime.GOARCH
		if !cincZeroPlatforms[platform] {
			cincZeroSkip = fmt.Sprintf("cinc-zero publishes no %s binary", platform)
			return
		}
		cincZeroPath, cincZeroErr = ensureCincZero(platform)
	})
	if cincZeroSkip != "" {
		t.Skip(cincZeroSkip)
	}
	if cincZeroErr != nil {
		t.Fatalf("obtaining cinc-zero %s: %v", cincZeroVersion, cincZeroErr)
	}
	return cincZeroPath
}

// ensureCincZero returns a cached cinc-zero binary for platform, downloading
// and verifying the release archive if it is not already cached.
func ensureCincZero(platform string) (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cacheRoot, "cinc-cli-acceptance", cincZeroVersion, platform)
	binary := filepath.Join(dir, "cinc-zero")
	if _, err := os.Stat(binary); err == nil {
		return binary, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	asset := fmt.Sprintf("cinc-zero_%s_%s.tar.gz", cincZeroVersion, platform)
	base := "https://github.com/tas50/cinc-zero/releases/download/" + cincZeroVersion
	archive, err := httpGet(base + "/" + asset)
	if err != nil {
		return "", err
	}
	sums, err := httpGet(base + "/SHA256SUMS")
	if err != nil {
		return "", err
	}
	if err := verifyChecksum(archive, sums, asset); err != nil {
		return "", err
	}
	if err := extractCincZero(archive, binary); err != nil {
		return "", err
	}
	return binary, nil
}

// httpGet fetches url, following redirects, and returns the body or an error on
// any non-200 status.
func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// verifyChecksum confirms archive's SHA-256 matches the entry for asset in a
// SHA256SUMS file ("<hex>  <filename>" lines).
func verifyChecksum(archive, sums []byte, asset string) error {
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum for %s in SHA256SUMS", asset)
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch for %s: got %x, want %s", asset, got, want)
	}
	return nil
}

// extractCincZero pulls the cinc-zero executable out of a .tar.gz archive and
// writes it to dest with the executable bit set.
func extractCincZero(archive []byte, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("cinc-zero binary not found in archive")
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != "cinc-zero" {
			continue
		}
		tmp := dest + ".tmp"
		f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		return os.Rename(tmp, dest)
	}
}

// seedDir returns the chef-repo fixture directory shipped alongside these
// tests, which cinc-zero preloads via --repo.
func seedDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the acceptance test source directory")
	}
	return filepath.Join(filepath.Dir(thisFile), "seed")
}

// cincZeroOptions configures how startCincZeroWith launches the server.
// The zero value means: org "acme", signature verification ON, ACLs
// permissive, no admin key emitted.
type cincZeroOptions struct {
	orgs        string // comma-separated; defaults to "acme"
	adminKeyOut string // if set, --key-out writes the admin key here
	noAuth      bool   // if true, pass --no-auth (disables signing checks)
	enforceACLs bool   // if true, pass --enforce-acls (real 403s)
}

// startCincZeroWith launches cinc-zero on port with the given options,
// blocks until it serves requests (and, if adminKeyOut is set, until the
// admin key file exists), and returns a shutdown func.
func startCincZeroWith(t *testing.T, port int, opts cincZeroOptions) func() {
	t.Helper()
	binary := cincZeroBinary(t)

	orgs := opts.orgs
	if orgs == "" {
		orgs = "acme"
	}
	args := []string{
		"--addr", fmt.Sprintf("127.0.0.1:%d", port),
		"--orgs", orgs,
		"--repo", seedDir(t),
	}
	if opts.adminKeyOut != "" {
		args = append(args, "--key-out", opts.adminKeyOut)
	}
	if opts.noAuth {
		args = append(args, "--no-auth")
	}
	if opts.enforceACLs {
		args = append(args, "--enforce-acls")
	}

	cmd := exec.Command(binary, args...)
	var log bytes.Buffer
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting cinc-zero: %v", err)
	}
	stop := func() {
		_ = cmd.Process.Signal(os.Interrupt)
		_ = cmd.Wait()
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/organizations/acme/nodes", port)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			// With auth on, an unsigned probe gets 401 (not 200); either
			// status means the server is up and serving.
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
				if opts.adminKeyOut == "" || fileExists(opts.adminKeyOut) {
					return stop
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	stop()
	t.Fatalf("cinc-zero did not become ready on port %d within 30s\noutput:\n%s", port, log.String())
	return nil
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// startCincZero launches a permissive, no-auth, single-org cinc-zero.
// Retained for setup_test.go, which signs with an unregistered key to
// exercise credential migration rather than authentication.
func startCincZero(t *testing.T, port int) func() {
	t.Helper()
	return startCincZeroWith(t, port, cincZeroOptions{noAuth: true})
}

// buildCinc compiles the cinc binary once per test run and returns its
// path. Subsequent calls return the cached binary, which keeps the
// acceptance suite fast as it grows.
var (
	cincBinaryOnce sync.Once
	cincBinaryPath string
	cincBinaryErr  error
)

func buildCinc(t *testing.T) string {
	t.Helper()
	cincBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cinc-acceptance-*")
		if err != nil {
			cincBinaryErr = err
			return
		}
		binary := filepath.Join(dir, "cinc")
		if out, err := exec.Command("go", "build", "-o", binary, cincPackage).CombinedOutput(); err != nil {
			cincBinaryErr = fmt.Errorf("building cinc: %v\n%s", err, out)
			return
		}
		cincBinaryPath = binary
	})
	if cincBinaryErr != nil {
		t.Fatal(cincBinaryErr)
	}
	return cincBinaryPath
}

// writeAcceptanceConfig writes a credentials file for org on the given
// port, signing as the bootstrap admin "pivotal" with the key at
// adminKey, and returns the config path.
func writeAcceptanceConfig(t *testing.T, port int, org, adminKey string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "credentials")
	cfg := fmt.Sprintf(`[default]
cinc_server_url = "http://127.0.0.1:%d/organizations/%s"
client_name     = "pivotal"
client_key      = %q
`, port, org, adminKey)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

// seedGlobalActors creates the two objects the chef-repo loader can't express:
// the global users anna and ben (the /users endpoint is global, not org-scoped)
// and the mutable "devs" authz group (the loader has no groups/ directory).
// Both are seeded through the cinc CLI against the running server.
func seedGlobalActors(t *testing.T, binary, cfgPath string) {
	t.Helper()
	users := []struct{ name, display, first, last string }{
		{"anna", "Anna Admin", "Anna", "Admin"},
		{"ben", "Ben Viewer", "Ben", "Viewer"},
	}
	for _, u := range users {
		runCinc(t, binary, "user", "create", u.name,
			"--email", u.name+"@example.test",
			"--display-name", u.display,
			"--first-name", u.first,
			"--last-name", u.last,
			"--config", cfgPath)
	}
	runCinc(t, binary, "group", "create", "devs", "--config", cfgPath)
}

// acceptanceEnv bundles everything an acceptance test needs to drive
// the real cinc binary against a freshly seeded cinc-zero server.
type acceptanceEnv struct {
	binary   string
	cfgPath  string
	port     int    // the port cinc-zero is listening on
	adminKey string // path to the emitted pivotal admin private key
}

// acceptanceOptions configures startAcceptanceWith.
type acceptanceOptions struct {
	orgs        string // comma-separated; defaults to "acme"
	enforceACLs bool
}

// startAcceptance does the standard per-test setup with signature
// verification ON and a single "acme" org. The returned stop function
// tears cinc-zero down; the caller should `defer stop()`.
func startAcceptance(t *testing.T) (acceptanceEnv, func()) {
	return startAcceptanceWith(t, acceptanceOptions{})
}

// startAcceptanceWith starts cinc-zero with signature verification on,
// emits the pivotal admin key, builds the cinc binary, writes a config
// for org "acme" signing as pivotal, and seeds the global users and
// "devs" group. enforceACLs and a multi-org list are opt-in.
func startAcceptanceWith(t *testing.T, opts acceptanceOptions) (acceptanceEnv, func()) {
	t.Helper()
	port := freePort(t)
	adminKey := filepath.Join(t.TempDir(), "pivotal.pem")
	stop := startCincZeroWith(t, port, cincZeroOptions{
		orgs:        opts.orgs,
		adminKeyOut: adminKey,
		enforceACLs: opts.enforceACLs,
	})
	env := acceptanceEnv{
		binary:   buildCinc(t),
		cfgPath:  writeAcceptanceConfig(t, port, "acme", adminKey),
		port:     port,
		adminKey: adminKey,
	}
	seedGlobalActors(t, env.binary, env.cfgPath)
	return env, stop
}

// runCinc executes the cinc binary, fails the test on a non-zero exit,
// and returns its standard output.
func runCinc(t *testing.T, binary string, args ...string) string {
	t.Helper()
	stdout, stderr, err := runCincRaw(binary, args...)
	if err != nil {
		t.Fatalf("cinc %s failed: %v\nstderr: %s", strings.Join(args, " "), err, stderr)
	}
	return stdout
}

// runCincRaw runs the cinc binary without failing the test on a
// non-zero exit, so tests can assert on the error path.
func runCincRaw(binary string, args ...string) (stdout, stderr string, err error) {
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(binary, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// runCincRawEnv runs the cinc binary with extra environment variables
// (appended to the current environment), without failing on non-zero
// exit, so tests can assert on profile/env resolution.
func runCincRawEnv(t *testing.T, extraEnv []string, binary string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// itoa is a tiny strconv.Itoa wrapper kept local so acceptance tests can
// build URLs without each importing strconv.
func itoa(n int) string { return strconv.Itoa(n) }
