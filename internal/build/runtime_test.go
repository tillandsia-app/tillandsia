package build

import (
	"testing"
)

func TestGenerateDockerfile(t *testing.T) {
	df, err := GenerateDockerfile("node:20", ".", "node server.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(df) < 50 {
		t.Fatalf("dockerfile too short: %q", df)
	}
}

func TestGenerateDockerfileUnsupported(t *testing.T) {
	_, err := GenerateDockerfile("kotlin:1.9", ".", "echo hi")
	if err == nil {
		t.Fatal("expected error for unsupported runtime")
	}
}

func TestGenerateDockerfileVersionFallback(t *testing.T) {
	df, err := GenerateDockerfile("node", ".", "node server.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(df) < 50 {
		t.Fatalf("dockerfile too short: %q", df)
	}
}

func TestGenerateDockerfilePython(t *testing.T) {
	df, err := GenerateDockerfile("python", ".", "python app.py")
	if err != nil {
		t.Fatal(err)
	}
	if len(df) < 50 {
		t.Fatalf("dockerfile too short: %q", df)
	}
}

func TestGenerateDockerfileGo(t *testing.T) {
	df, err := GenerateDockerfile("go", ".", "./app")
	if err != nil {
		t.Fatal(err)
	}
	if len(df) < 50 {
		t.Fatalf("dockerfile too short: %q", df)
	}
}

func TestSupportedRuntimes(t *testing.T) {
	rts := SupportedRuntimes()
	if len(rts) == 0 {
		t.Fatal("expected at least 1 runtime")
	}
	seen := make(map[string]bool)
	for _, r := range rts {
		if seen[r] {
			t.Fatalf("duplicate runtime: %s", r)
		}
		seen[r] = true
	}
}