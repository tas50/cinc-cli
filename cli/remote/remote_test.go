package remote

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestApplyOpenSSHConfigUsesIdentityAgent(t *testing.T) {
	old := sshConfigGet
	sshConfigGet = func(host, key string) (string, error) {
		if host != "node-0b0715" || key != "IdentityAgent" {
			return "", fmt.Errorf("unexpected lookup %s %s", host, key)
		}
		return "~/.1password/agent.sock", nil
	}
	t.Cleanup(func() { sshConfigGet = old })

	opts, err := applyOpenSSHConfig("node-0b0715", SSHOptions{UseAgent: true})
	if err != nil {
		t.Fatalf("applyOpenSSHConfig: %v", err)
	}
	if opts.AgentSocket != "~/.1password/agent.sock" {
		t.Fatalf("AgentSocket = %q", opts.AgentSocket)
	}
}

func TestApplyOpenSSHConfigHonorsIdentityAgentNone(t *testing.T) {
	old := sshConfigGet
	sshConfigGet = func(_, _ string) (string, error) { return "none", nil }
	t.Cleanup(func() { sshConfigGet = old })

	opts, err := applyOpenSSHConfig("host", SSHOptions{UseAgent: true})
	if err != nil {
		t.Fatalf("applyOpenSSHConfig: %v", err)
	}
	if opts.UseAgent {
		t.Fatal("UseAgent = true, want disabled by IdentityAgent none")
	}
}

func TestApplyOpenSSHConfigKeepsExplicitAgentSocket(t *testing.T) {
	old := sshConfigGet
	sshConfigGet = func(_, _ string) (string, error) {
		t.Fatal("ssh config should not be read when AgentSocket is explicit")
		return "", nil
	}
	t.Cleanup(func() { sshConfigGet = old })

	opts, err := applyOpenSSHConfig("host", SSHOptions{UseAgent: true, AgentSocket: "/tmp/agent.sock"})
	if err != nil {
		t.Fatalf("applyOpenSSHConfig: %v", err)
	}
	if opts.AgentSocket != "/tmp/agent.sock" {
		t.Fatalf("AgentSocket = %q", opts.AgentSocket)
	}
}

func TestAgentSocketCandidatesIncludesOnePasswordFallbacks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "/tmp/ssh-agent.sock")

	got := agentSocketCandidates("~/explicit-agent.sock")
	want := []string{
		filepath.Join(home, "explicit-agent.sock"),
		filepath.Join(home, ".1password", "agent.sock"),
		filepath.Join(home, "Library", "Group Containers", "2BUA8C4S2C.com.1password", "t", "agent.sock"),
		"/tmp/ssh-agent.sock",
	}
	if len(got) != len(want) {
		t.Fatalf("agent socket candidates = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("agent socket candidate %d = %q, want %q", i, got[i], want[i])
		}
	}
}
