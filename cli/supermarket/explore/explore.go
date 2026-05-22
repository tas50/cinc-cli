package explore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	sm "github.com/tas50/cinc-supermarket"
)

// Options configures Run.
type Options struct {
	Site   string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Run launches the explore TUI against the configured Supermarket
// site. It is the single entry point used by the cobra command.
func Run(ctx context.Context, opts Options) error {
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if !stdoutIsTTY(opts.Stdout) {
		return errors.New("cinc supermarket explore needs an interactive terminal — run it from a normal shell session")
	}
	if opts.Site == "" {
		opts.Site = sm.DefaultBaseURL
	}
	client, err := newRealClient(opts.Site)
	if err != nil {
		return fmt.Errorf("supermarket explore: %w", err)
	}
	m := initialModel(ctx, client, opts.Site, openBrowser)
	p := tea.NewProgram(
		m,
		tea.WithContext(ctx),
		tea.WithInput(opts.Stdin),
		tea.WithOutput(opts.Stdout),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("supermarket explore: %w", err)
	}
	return nil
}

// stdoutIsTTY checks whether the output is an interactive terminal. We
// only treat *os.File-shaped outputs as candidates so tests passing
// bytes.Buffer just say "no, not a TTY".
func stdoutIsTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// openBrowser shells out to the platform-native URL opener. Failures
// are returned so the TUI can surface them in the footer.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	// Reap the child so we don't leave a zombie behind. We don't care
	// about the exit status — if the opener fails, the user will see
	// nothing happen, but that's a UX issue beyond our reach.
	go func() { _ = cmd.Wait() }()
	return nil
}
