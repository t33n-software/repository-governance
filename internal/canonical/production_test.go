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
	tenantEntries, err := verifier.ListTenant("")
	if err != nil {
		t.Fatalf("ListTenant: %v", err)
	}
	if len(tenantEntries) != 1 || tenantEntries[0].Name() != "tenant.txt" {
		t.Fatalf("tenant entries = %v", tenantEntries)
	}
	moduleEntries, err := verifier.ListModule(root, "")
	if err != nil {
		t.Fatalf("ListModule: %v", err)
	}
	if len(moduleEntries) != 1 || moduleEntries[0].Name() != "tenant.txt" {
		t.Fatalf("module entries = %v", moduleEntries)
	}
	if _, err := verifier.ListTenant("missing"); err == nil {
		t.Fatal("expected the missing directory error")
	}
	if verifier.ResolveModule == nil {
		t.Fatal("the module resolver must be bound")
	}
	// The tool execution seam is exercised with a harmless toolchain query.
	toolOutput, err := verifier.RunTool(context.Background(), root, "version")
	if err != nil {
		t.Fatalf("RunTool: %v", err)
	}
	if !strings.Contains(toolOutput, "go version") {
		t.Fatalf("tool output = %q", toolOutput)
	}
}

// TestListEntries proves the production seam surfaces files and directories
// with their honest directory bits: the consumers skip non-directory entries,
// so the seam must report both kinds faithfully.
func TestListEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := listEntries(dir)
	if err != nil {
		t.Fatalf("listEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %v", entries)
	}
	if entries[0].Name() != "a" || !entries[0].IsDir() {
		t.Fatalf("the directory entry = %v", entries[0])
	}
	if entries[1].Name() != "b.txt" || entries[1].IsDir() {
		t.Fatalf("the file entry = %v", entries[1])
	}
	if _, err := listEntries(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected the missing directory error")
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

func TestResolveModuleDirDownloadsBeforeResolving(t *testing.T) {
	defer func() { execOutput = commandOutput }()

	type call struct {
		name string
		args []string
	}
	var calls []call
	execOutput = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if dir != "tools" {
			t.Fatalf("dir = %q", dir)
		}
		calls = append(calls, call{name: name, args: append([]string(nil), args...)})
		return []byte("/cache/module\n"), nil
	}

	dir, err := ResolveModuleDir(context.Background(), "tools", "example.com/mod")
	if err != nil {
		t.Fatalf("ResolveModuleDir: %v", err)
	}
	if dir != "/cache/module" {
		t.Fatalf("dir = %q", dir)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d", len(calls))
	}
	if got := strings.Join(calls[0].args, " "); calls[0].name != "go" || got != "mod download example.com/mod" {
		t.Fatalf("first call = %s %s", calls[0].name, got)
	}
	if got := strings.Join(calls[1].args, " "); calls[1].name != "go" || got != "list -m -f {{.Dir}} example.com/mod" {
		t.Fatalf("second call = %s %s", calls[1].name, got)
	}
}

func TestResolveModuleDirDownloadFailureFailsClosed(t *testing.T) {
	defer func() { execOutput = commandOutput }()

	var listCalled bool
	execOutput = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "list" {
			listCalled = true
		}
		return []byte("network unreachable"), errors.New("exit 1")
	}

	_, err := ResolveModuleDir(context.Background(), "tools", "example.com/mod")
	if err == nil || !strings.Contains(err.Error(), "go mod download") || !strings.Contains(err.Error(), "network unreachable") {
		t.Fatalf("err = %v", err)
	}
	if listCalled {
		t.Fatal("the directory query must not run after a failed download")
	}
}

func TestResolveModuleDirListFailureAfterDownload(t *testing.T) {
	defer func() { execOutput = commandOutput }()

	execOutput = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "list" {
			return []byte("module lookup failed"), errors.New("exit 1")
		}
		return []byte(""), nil
	}

	_, err := ResolveModuleDir(context.Background(), "tools", "example.com/mod")
	if err == nil || !strings.Contains(err.Error(), "go list -m") || !strings.Contains(err.Error(), "module lookup failed") {
		t.Fatalf("err = %v", err)
	}
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
