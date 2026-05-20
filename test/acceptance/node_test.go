//go:build acceptance

package acceptance

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh"
)

// TestNodeListAgainstChefZero verifies that `cinc node list` against a
// real chef-zero server returns the seeded nodes in both human and
// JSON output formats.
func TestNodeListAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	human := runCinc(t, env.binary, "node", "list", "--config", env.cfgPath)
	if human != "db01\nweb01\nweb02\n" {
		t.Errorf("node list (human) = %q, want sorted node names", human)
	}

	jsonOut := runCinc(t, env.binary, "node", "list", "--config", env.cfgPath, "--format", "json")
	for _, name := range []string{"db01", "web01", "web02"} {
		if !strings.Contains(jsonOut, name) {
			t.Errorf("node list (json) missing %q\ngot: %s", name, jsonOut)
		}
	}
}

// TestNodeDeleteAgainstChefZero deletes one of the seeded nodes and
// verifies a follow-up list no longer includes it.
func TestNodeDeleteAgainstChefZero(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()

	out := runCinc(t, env.binary, "node", "delete", "web02", "--config", env.cfgPath)
	if out != "Deleted node \"web02\"\n" {
		t.Errorf("node delete output = %q", out)
	}

	after := runCinc(t, env.binary, "node", "list", "--config", env.cfgPath)
	if after != "db01\nweb01\n" {
		t.Errorf("node list after delete = %q, want web02 absent", after)
	}
}

func TestNodeSSHManualListAgainstSSHServer(t *testing.T) {
	server := startAcceptanceSSHServer(t, "hello from ssh\n")

	out := runCinc(t, buildCinc(t),
		"node", "ssh", "127.0.0.1", "echo hello",
		"--manual-list",
		"--ssh-user", "tester",
		"--ssh-password", "secret",
		"--ssh-port", fmt.Sprint(server.port),
		"--no-host-key-verify",
	)
	if out != "127.0.0.1\thello from ssh\n" {
		t.Fatalf("node ssh output = %q", out)
	}
	if got := server.lastCommand(); got != "echo hello" {
		t.Fatalf("ssh command = %q", got)
	}
}

func TestNodeBootstrapAgainstChefZeroAndSSHServer(t *testing.T) {
	env, stop := startAcceptance(t)
	defer stop()
	server := startAcceptanceSSHServer(t, "bootstrap ok\n")

	out := runCinc(t, env.binary,
		"node", "bootstrap", "127.0.0.1",
		"--node-name", "boot01",
		"--ssh-user", "tester",
		"--ssh-password", "secret",
		"--ssh-port", fmt.Sprint(server.port),
		"--no-host-key-verify",
		"--config", env.cfgPath,
	)
	if out != "Bootstrapped node \"boot01\" on 127.0.0.1\n" {
		t.Fatalf("node bootstrap output = %q", out)
	}
	command := server.lastCommand()
	for _, want := range []string{
		"curl -L 'https://omnitruck.cinc.sh/install.sh'",
		"node_name \"boot01\"",
		"cinc-client -j /etc/cinc/first-boot.json",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("bootstrap command missing %q:\n%s", want, command)
		}
	}
}

type acceptanceSSHServer struct {
	port    int
	output  string
	mu      sync.Mutex
	command string
}

func (s *acceptanceSSHServer) lastCommand() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.command
}

func startAcceptanceSSHServer(t *testing.T, output string) *acceptanceSSHServer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == "tester" && string(password) == "secret" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &acceptanceSSHServer{
		port:   listener.Addr().(*net.TCPAddr).Port,
		output: output,
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go server.handleConn(conn, config)
		}
	}()
	return server
}

func (s *acceptanceSSHServer) handleConn(conn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)
	for ch := range chans {
		if ch.ChannelType() != "session" {
			_ = ch.Reject(ssh.UnknownChannelType, "session required")
			continue
		}
		channel, requests, err := ch.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(channel, requests)
	}
}

func (s *acceptanceSSHServer) handleSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer channel.Close()
	for req := range requests {
		if req.Type != "exec" {
			_ = req.Reply(false, nil)
			continue
		}
		command := parseExecCommand(req.Payload)
		s.mu.Lock()
		s.command = command
		s.mu.Unlock()
		_ = req.Reply(true, nil)
		_, _ = io.WriteString(channel, s.output)
		_, _ = channel.SendRequest("exit-status", false, exitStatusPayload(0))
		return
	}
}

func parseExecCommand(payload []byte) string {
	if len(payload) < 4 {
		return ""
	}
	n := binary.BigEndian.Uint32(payload[:4])
	if int(n) > len(payload)-4 {
		return ""
	}
	return string(payload[4 : 4+n])
}

func exitStatusPayload(status uint32) []byte {
	var payload [4]byte
	binary.BigEndian.PutUint32(payload[:], status)
	return payload[:]
}
