package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

// testSSHServer is a minimal in-process SSH server for testing.
// It supports keepalive requests and direct-tcpip channel forwarding.
type testSSHServer struct {
	listener    net.Listener
	config      *gossh.ServerConfig
	hostKey     gossh.Signer
	tempKeyFile string // path to a temporary client private key for auth
	wg          sync.WaitGroup

	mu     sync.Mutex
	closed bool
	conns  []net.Conn // tracked for cleanup on Close

	// Hooks for controlling server behavior in tests.
	onKeepalive   func() bool // return false to reject keepalive
	onDirectTCPIP func(destHost string, destPort uint32) (net.Conn, error)
}

// newTestSSHServer creates and starts a test SSH server on a random local port.
func newTestSSHServer(t *testing.T) *testSSHServer {
	t.Helper()

	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}
	hostSigner, err := gossh.NewSignerFromKey(privKey)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	config := &gossh.ServerConfig{
		NoClientAuth: true,
	}
	config.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	// Generate a temporary client key file so Connect() has an auth method.
	keyFile := writeTempKey(t)

	s := &testSSHServer{
		listener:    ln,
		config:      config,
		hostKey:     hostSigner,
		tempKeyFile: keyFile,
	}

	s.wg.Add(1)
	go s.serve()

	t.Cleanup(func() { s.Close() })
	return s
}

// Addr returns the server's listen address (e.g. "127.0.0.1:12345").
func (s *testSSHServer) Addr() string {
	return s.listener.Addr().String()
}

// HostPort returns the host and port separately.
func (s *testSSHServer) HostPort() (string, int) {
	addr := s.listener.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

// Close shuts down the server and all active connections.
func (s *testSSHServer) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	// Close all tracked connections so handleConn goroutines can exit.
	for _, c := range s.conns {
		c.Close()
	}
	s.conns = nil
	s.mu.Unlock()

	s.listener.Close()
	s.wg.Wait()
}

func (s *testSSHServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns = append(s.conns, conn)
		s.mu.Unlock()
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

func (s *testSSHServer) handleConn(netConn net.Conn) {
	sshConn, chans, reqs, err := gossh.NewServerConn(netConn, s.config)
	if err != nil {
		netConn.Close()
		return
	}
	defer sshConn.Close()

	// Handle global requests (keepalive, etc.)
	go s.handleGlobalRequests(reqs)

	// Handle channel requests (direct-tcpip for tunneling)
	for newChan := range chans {
		if newChan.ChannelType() == "direct-tcpip" {
			s.handleDirectTCPIP(newChan)
		} else {
			newChan.Reject(gossh.UnknownChannelType, "unsupported channel type")
		}
	}
}

func (s *testSSHServer) handleGlobalRequests(reqs <-chan *gossh.Request) {
	for req := range reqs {
		switch req.Type {
		case "keepalive@openssh.com":
			s.mu.Lock()
			hook := s.onKeepalive
			s.mu.Unlock()
			if hook != nil {
				req.Reply(hook(), nil)
			} else {
				req.Reply(true, nil)
			}
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// directTCPIPPayload is the SSH protocol payload for direct-tcpip channels.
type directTCPIPPayload struct {
	DestHost   string
	DestPort   uint32
	OriginHost string
	OriginPort uint32
}

func (s *testSSHServer) handleDirectTCPIP(newChan gossh.NewChannel) {
	var payload directTCPIPPayload
	if err := gossh.Unmarshal(newChan.ExtraData(), &payload); err != nil {
		newChan.Reject(gossh.ConnectionFailed, "invalid payload")
		return
	}

	s.mu.Lock()
	hook := s.onDirectTCPIP
	s.mu.Unlock()

	if hook == nil {
		newChan.Reject(gossh.ConnectionFailed, "no direct-tcpip handler")
		return
	}

	target, err := hook(payload.DestHost, payload.DestPort)
	if err != nil {
		newChan.Reject(gossh.ConnectionFailed, err.Error())
		return
	}

	ch, _, err := newChan.Accept()
	if err != nil {
		target.Close()
		return
	}

	go func() {
		defer ch.Close()
		defer target.Close()
		go io.Copy(ch, target)
		io.Copy(target, ch)
	}()
}

// startEchoServer starts a simple TCP echo server for testing direct-tcpip.
func startEchoServer(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start echo server: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				io.Copy(conn, conn)
			}()
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln
}

// connectToTestServer creates client Options for connecting to a testSSHServer.
// It generates a temporary ed25519 private key file so that Connect() has an
// auth method available even in CI environments without ~/.ssh keys.
func connectToTestServer(s *testSSHServer) Options {
	host, port := s.HostPort()
	return Options{
		Host:                host,
		Port:                port,
		IdentityFile:        s.tempKeyFile,
		SkipKnownHostsCheck: true,
		KeepaliveInterval:   -1, // disabled by default, tests enable as needed
	}
}

// parseHostPort splits an address string into host and port.
func parseHostPort(addr string) (string, int) {
	host, portStr, _ := net.SplitHostPort(addr)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	return host, port
}

// writeTempKey generates an ed25519 private key, writes it to a temp file in
// PEM format, and registers cleanup via t.Cleanup. Returns the file path.
func writeTempKey(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}

	f, err := os.CreateTemp(t.TempDir(), "id_ed25519_test_*")
	if err != nil {
		t.Fatalf("create temp key file: %v", err)
	}
	if err := pem.Encode(f, block); err != nil {
		f.Close()
		t.Fatalf("write key: %v", err)
	}
	f.Close()
	return f.Name()
}
