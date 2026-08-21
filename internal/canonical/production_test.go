package canonical

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewVerifierProductionSeams(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "tenant.txt"), []byte("tenant"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "home.txt"), []byte("home"), 0o600); err != nil {
		t.Fatal(err)
	}

	verifier := NewVerifier(root, home, nil, nil)
	if verifier.Stdout == nil || verifier.Stderr == nil {
		t.Fatal("the default writers must be bound")
	}
	tenant, err := verifier.ReadTenant("tenant.txt")
	if err != nil {
		t.Fatalf("ReadTenant: %v", err)
	}
	if string(tenant) != "tenant" {
		t.Fatalf("tenant = %q", tenant)
	}
	homeContent, err := verifier.ReadHome("home.txt")
	if err != nil {
		t.Fatalf("ReadHome: %v", err)
	}
	if string(homeContent) != "home" {
		t.Fatalf("home = %q", homeContent)
	}
	module, err := verifier.ReadModule(root, "tenant.txt")
	if err != nil {
		t.Fatalf("ReadModule: %v", err)
	}
	if string(module) != "tenant" {
		t.Fatalf("module = %q", module)
	}
	if _, err := verifier.ReadTenant("missing.txt"); err == nil {
		t.Fatal("expected the missing file error")
	}
	if verifier.ResolveModule == nil {
		t.Fatal("the module resolver must be bound")
	}
}

func TestResolveModuleDir(t *testing.T) {
	defer func() { execOutput = commandOutput }()

	t.Run("success", func(t *testing.T) {
		execOutput = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
			return []byte("/cache/module\n"), nil
		}
		dir, err := ResolveModuleDir(context.Background(), "tools", "example.com/mod")
		if err != nil {
			t.Fatalf("ResolveModuleDir: %v", err)
		}
		if dir != "/cache/module" {
			t.Fatalf("dir = %q", dir)
		}
	})

	t.Run("nil context", func(t *testing.T) {
		execOutput = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
			if ctx == nil {
				t.Fatal("the context must be normalized")
			}
			return []byte("/cache/module\n"), nil
		}
		if _, err := ResolveModuleDir(testNilContext(), "tools", "example.com/mod"); err != nil {
			t.Fatalf("ResolveModuleDir: %v", err)
		}
	})

	t.Run("exec error", func(t *testing.T) {
		execOutput = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
			return []byte("not in graph"), errors.New("exit 1")
		}
		_, err := ResolveModuleDir(context.Background(), "tools", "example.com/mod")
		if err == nil || !strings.Contains(err.Error(), "not in graph") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("empty output", func(t *testing.T) {
		execOutput = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
			return []byte("\n"), nil
		}
		if _, err := ResolveModuleDir(context.Background(), "tools", "example.com/mod"); err == nil {
			t.Fatal("expected the empty output rejection")
		}
	})
}

func TestCommandOutputRealCommand(t *testing.T) {
	// The real process seam is exercised with a harmless toolchain query.
	output, err := commandOutput(context.Background(), ".", "go", "version")
	if err != nil {
		t.Fatalf("exec go version: %v", err)
	}
	if !strings.Contains(string(output), "go version") {
		t.Fatalf("output = %q", output)
	}
}
