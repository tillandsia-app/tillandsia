package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

const keyDir = "ssh"

type Client struct {
	client *ssh.Client
	host   string
	user   string
	port   int
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type ExitError struct {
	ExitCode int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("remote command exited with code %d", e.ExitCode)
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tillandsia"), nil
}

func KeyDir() (string, error) {
	cfgDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, keyDir), nil
}

func EnsureKey() (string, error) {
	keyDirPath, err := KeyDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(keyDirPath, 0700); err != nil {
		return "", fmt.Errorf("creating key directory: %w", err)
	}

	privPath := filepath.Join(keyDirPath, "id_ed25519")
	if _, err := os.Stat(privPath); err == nil {
		return privPath, nil
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generating key: %w", err)
	}

	sshPriv, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return "", fmt.Errorf("creating signer from key: %w", err)
	}

	privPEM, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return "", fmt.Errorf("marshaling private key: %w", err)
	}

	if err := os.WriteFile(privPath, pem.EncodeToMemory(privPEM), 0600); err != nil {
		return "", fmt.Errorf("writing private key: %w", err)
	}

	pubBytes := ssh.MarshalAuthorizedKey(sshPriv.PublicKey())
	pubPath := filepath.Join(keyDirPath, "id_ed25519.pub")
	if err := os.WriteFile(pubPath, pubBytes, 0644); err != nil {
		return "", fmt.Errorf("writing public key: %w", err)
	}

	return privPath, nil
}

func PublicKey() (string, error) {
	keyDirPath, err := KeyDir()
	if err != nil {
		return "", err
	}
	pubPath := filepath.Join(keyDirPath, "id_ed25519.pub")
	data, err := os.ReadFile(pubPath)
	if err != nil {
		return "", fmt.Errorf("reading public key: %w", err)
	}
	return string(data), nil
}

func Connect(host, user, keyPath string, port int) (*Client, error) {
	if port == 0 {
		port = 22
	}
	if user == "" {
		user = "root"
	}

	config := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: hostKeyCallback(),
		Timeout:         10 * time.Second,
	}

	auths, err := collectAuthMethods(keyPath, host, user)
	if err != nil {
		return nil, err
	}
	config.Auth = auths

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH connect to %s: %w", addr, err)
	}

	return &Client{
		client: conn,
		host:   host,
		user:   user,
		port:   port,
	}, nil
}

func collectAuthMethods(keyPath, host, user string) ([]ssh.AuthMethod, error) {
	var auths []ssh.AuthMethod

	if keyPath != "" {
		key, err := parsePrivateKey(keyPath)
		if err != nil {
			return nil, fmt.Errorf("reading SSH key %s: %w", keyPath, err)
		}
		auths = append(auths, key)
		return auths, nil
	}

	// Search tillandsia-managed keys first
	keyDirPath, err := KeyDir()
	if err == nil {
		tillandsiaKeys := []string{
			filepath.Join(keyDirPath, "id_ed25519"),
			filepath.Join(keyDirPath, "id_rsa"),
		}
		for _, kp := range tillandsiaKeys {
			if _, err := os.Stat(kp); err == nil {
				key, err := parsePrivateKey(kp)
				if err == nil {
					auths = append(auths, key)
					break
				}
			}
		}
	}

	// Fall back to user's standard SSH keys
	if len(auths) == 0 {
		home, err := os.UserHomeDir()
		if err == nil {
			userKeys := []string{
				filepath.Join(home, ".ssh", "id_ed25519"),
				filepath.Join(home, ".ssh", "id_rsa"),
				filepath.Join(home, ".ssh", "id_ecdsa"),
				filepath.Join(home, ".ssh", "id_xmss"),
			}
			for _, kp := range userKeys {
				if _, err := os.Stat(kp); err == nil {
					key, err := parsePrivateKey(kp)
					if err == nil {
						auths = append(auths, key)
						break
					}
				}
			}
		}
	}

	if len(auths) > 0 {
		return auths, nil
	}

	fmt.Fprintf(os.Stderr, "Password for %s@%s: ", user, host)
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("reading password: %w", err)
	}
	auths = append(auths, ssh.Password(string(bytePassword)))
	return auths, nil
}

func parsePrivateKey(path string) (ssh.AuthMethod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(key), nil
}

func hostKeyCallback() ssh.HostKeyCallback {
	home, err := os.UserHomeDir()
	if err != nil {
		return ssh.InsecureIgnoreHostKey()
	}
	knownHostsFile := filepath.Join(home, ".ssh", "known_hosts")
	cb, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return ssh.InsecureIgnoreHostKey()
	}
	return cb
}

func (c *Client) Run(ctx context.Context, cmd string) (*Result, error) {
	sess, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	defer sess.Close()

	ctxDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			sess.Close()
		case <-ctxDone:
		}
	}()
	defer close(ctxDone)

	stdout, err := sess.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := sess.Start(cmd); err != nil {
		return nil, fmt.Errorf("starting command: %w", err)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(&stdoutBuf, stdout)
	}()
	go func() {
		defer wg.Done()
		io.Copy(&stderrBuf, stderr)
	}()

	waitErr := sess.Wait()
	wg.Wait()

	result := &Result{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}
	if exitErr, ok := waitErr.(*ssh.ExitError); ok {
		result.ExitCode = exitErr.ExitStatus()
	} else if waitErr != nil {
		return result, fmt.Errorf("command failed: %w", waitErr)
	}

	return result, nil
}

func (c *Client) RunStream(ctx context.Context, cmd string, stdout, stderr io.Writer) error {
	sess, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	ctxDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			sess.Close()
		case <-ctxDone:
		}
	}()
	defer close(ctxDone)

	sess.Stdout = stdout
	sess.Stderr = stderr

	err = sess.Run(cmd)
	sess.Close()
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return &ExitError{ExitCode: exitErr.ExitStatus()}
		}
		return fmt.Errorf("running command: %w", err)
	}
	return nil
}

func (c *Client) Transfer(ctx context.Context, reader io.Reader, dest string) error {
	return c.PipeToCommand(ctx, reader, fmt.Sprintf("mkdir -p %s && cat > %s", filepath.Dir(dest), dest))
}

func (c *Client) PipeToCommand(ctx context.Context, reader io.Reader, remoteCmd string) error {
	sess, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	defer sess.Close()

	ctxDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			sess.Close()
		case <-ctxDone:
		}
	}()
	defer close(ctxDone)

	w, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	sess.Stdout = io.Discard
	sess.Stderr = io.Discard

	if err := sess.Start(remoteCmd); err != nil {
		w.Close()
		return fmt.Errorf("starting remote command: %w", err)
	}

	written, err := io.Copy(w, reader)
	if err != nil {
		w.Close()
		return fmt.Errorf("writing data: %w", err)
	}
	if written == 0 {
		w.Close()
		return fmt.Errorf("no data transferred")
	}
	w.Close()

	return sess.Wait()
}

func (c *Client) Test(ctx context.Context) error {
	result, err := c.Run(ctx, "docker info --format '{{.ServerVersion}}'")
	if err != nil {
		return fmt.Errorf("SSH connectivity check failed: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("Docker is not installed or not running on the server:\n%s", result.Stderr)
	}
	return nil
}

func (c *Client) Close() error {
	return c.client.Close()
}
