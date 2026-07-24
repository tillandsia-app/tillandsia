package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureKey(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	path, err := EnsureKey()
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(tmpHome, ".config", "tillandsia", "ssh", "id_ed25519")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("private key not created: %v", err)
	}

	pubPath := filepath.Join(filepath.Dir(path), "id_ed25519.pub")
	if _, err := os.Stat(pubPath); err != nil {
		t.Fatalf("public key not created: %v", err)
	}

	pub, err := PublicKey()
	if err != nil {
		t.Fatalf("PublicKey: %v", err)
	}
	if len(pub) == 0 {
		t.Fatal("public key is empty")
	}

	_, err = parsePrivateKey(path)
	if err != nil {
		t.Fatalf("parsing generated private key: %v", err)
	}
}

func TestEnsureKeyIdempotent(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	p1, err := EnsureKey()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := EnsureKey()
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Fatal("EnsureKey should return the same path on subsequent calls")
	}
}

func TestConfigDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(tmpHome, ".config", "tillandsia") {
		t.Fatalf("unexpected config dir: %s", dir)
	}
}