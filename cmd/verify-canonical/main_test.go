package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/t33n-software/repository-governance/internal/canonical"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run --version = %d", code)
	}
	if !strings.Contains(stdout.String(), "verify-canonical") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunUsageError(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run --bogus = %d, want 2", code)
	}
}

func TestRunAcceptsTheSpaceSeparatedFlagForm(t *testing.T) {
	// The composite action invokes the space-separated form; the read error
	// against an empty directory proves the flag was accepted.
	var stdout, stderr strings.Builder
	code := run(context.Background(), []string{"--repo", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run with the space-separated flag form = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "repo-bindings.json") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsAMissingFlagValue(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run with a missing flag value = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsAMissingTerritoryFlagValue(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--territory-home"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run with a missing territory flag value = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunAcceptsTheEqualsTerritoryFlagForm(t *testing.T) {
	defer func() { verify = verifyTenant }()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo-bindings.json"), []byte(minimalBindings()), 0o600); err != nil {
		t.Fatal(err)
	}
	verify = func(ctx context.Context, verifier canonical.Verifier, bindings canonical.Bindings) []canonical.Finding {
		return nil
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir(), "--territory-home=" + t.TempDir()}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d, want 0 (stderr: %s)", code, stderr.String())
	}
}

func TestRunSuccessWithTheSpaceSeparatedForm(t *testing.T) {
	defer func() { verify = verifyTenant }()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo-bindings.json"), []byte(minimalBindings()), 0o600); err != nil {
		t.Fatal(err)
	}
	verify = func(ctx context.Context, verifier canonical.Verifier, bindings canonical.Bindings) []canonical.Finding {
		return nil
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo", dir, "--home", t.TempDir()}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunManifestReadError(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run(context.Background(), []string{"--repo=" + t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run with a missing manifest = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "repo-bindings.json") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunManifestDecodeError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo-bindings.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir}, &stdout, &stderr); code != 1 {
		t.Fatalf("run with an invalid manifest = %d, want 1", code)
	}
}

func TestRunHomeResolutionError(t *testing.T) {
	defer func() { resolveHome = canonical.ResolveModuleDir }()
	resolveHome = func(context.Context, string, string) (string, error) {
		return "", errors.New("no module")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo-bindings.json"), []byte(minimalBindings()), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir}, &stdout, &stderr); code != 1 {
		t.Fatalf("run with a home resolution error = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no module") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunVerificationFailure(t *testing.T) {
	defer func() { verify = verifyTenant }()
	verify = func(context.Context, canonical.Verifier, canonical.Bindings) []canonical.Finding {
		return []canonical.Finding{{Check: "c", Detail: "d"}}
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo-bindings.json"), []byte(minimalBindings()), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir()}, &stdout, &stderr); code != 1 {
		t.Fatalf("run with findings = %d, want 1", code)
	}
}

func TestRunSuccess(t *testing.T) {
	defer func() { verify = verifyTenant }()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo-bindings.json"), []byte(minimalBindings()), 0o600); err != nil {
		t.Fatal(err)
	}
	verify = func(ctx context.Context, verifier canonical.Verifier, bindings canonical.Bindings) []canonical.Finding {
		if bindings.Home.Repository != "t33n-software/repository-governance" {
			t.Fatalf("Home.Repository = %q", bindings.Home.Repository)
		}
		return nil
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir()}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunTerritoryHomeFlagIsAccepted(t *testing.T) {
	// The composite action invokes the space-separated form; the acceptance
	// of all three flags in that form is proven through the passing run.
	defer func() { verify = verifyTenant }()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo-bindings.json"), []byte(minimalBindings()), 0o600); err != nil {
		t.Fatal(err)
	}
	verify = func(ctx context.Context, verifier canonical.Verifier, bindings canonical.Bindings) []canonical.Finding {
		return nil
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo", dir, "--home", t.TempDir(), "--territory-home", t.TempDir()}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunTerritoryResolutionFailsClosedWithoutTheFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repo-bindings.json"), []byte(minimalBindingsWithToolchain()), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir()}, &stdout, &stderr); code != 1 {
		t.Fatalf("run = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--territory-home") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestResolveTerritoryRoot(t *testing.T) {
	t.Run("the flag wins", func(t *testing.T) {
		root, err := resolveTerritoryRoot("territory", canonical.Bindings{})
		if err != nil || root != "territory" {
			t.Fatalf("root = %q, err = %v", root, err)
		}
	})
	t.Run("a toolchain binding without the flag fails closed", func(t *testing.T) {
		bindings := canonical.Bindings{Toolchain: &canonical.ToolchainBindings{Category: "monorepo"}}
		if _, err := resolveTerritoryRoot("", bindings); err == nil || !strings.Contains(err.Error(), "--territory-home") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("no binding and no flag resolves empty", func(t *testing.T) {
		root, err := resolveTerritoryRoot("", canonical.Bindings{})
		if err != nil || root != "" {
			t.Fatalf("root = %q, err = %v", root, err)
		}
	})
}

func TestVerifyTenantDelegation(t *testing.T) {
	// The delegation invokes the verifier's Verify; the failing read seams
	// surface their findings through the delegation.
	verifier := canonical.Verifier{
		ReadTenant: func(path string) ([]byte, error) { return nil, errors.New("boom") },
		ReadHome:   func(path string) ([]byte, error) { return nil, errors.New("boom") },
		Stdout:     io.Discard,
		Stderr:     io.Discard,
	}
	findings := verifyTenant(context.Background(), verifier, canonical.Bindings{})
	if len(findings) == 0 {
		t.Fatal("expected the delegated findings")
	}
}

func TestMain(t *testing.T) {
	defer func() { exitProcess = os.Exit }()
	defer func() { commandArgs = os.Args }()
	var code int
	exitProcess = func(c int) { code = c }
	commandArgs = []string{"verify-canonical", "--version"}
	main()
	if code != 0 {
		t.Fatalf("main exit = %d", code)
	}
}

// minimalBindings is the smallest manifest that decodes.
func minimalBindings() string {
	return `{
  "schemaVersion": 2,
  "home": { "repository": "t33n-software/repository-governance", "sha": "89be739ee8a1d1ed6ebbe97dd1556a253477d242" },
  "class": { "qualityGates": "linux-only", "codeScanning": true, "licenseHub": false },
  "callers": [
    {
      "file": ".github/workflows/ci.yml",
      "master": "hosting-platforms/github/workflows/callers/go/ci.yml",
      "sha256": "f29a65bd73fe575b159123a9d4bebed86ab4eebe3c5dc5dac31c96e7fb7c4c4a"
    }
  ],
  "files": {
    "lefthook": { "path": "lefthook.yml", "sha256": "` + strings.Repeat("a", 64) + `" },
    "gitattributes": { "path": ".gitattributes", "sha256": "` + strings.Repeat("b", 64) + `" },
    "gitignore": { "path": ".gitignore", "fragments": ["core"], "sha256": "` + strings.Repeat("c", 64) + `" },
    "dependabot": { "path": ".github/dependabot.yml", "sha256": "` + strings.Repeat("d", 64) + `" }
  },
  "codeowners": { "path": ".github/CODEOWNERS", "defaultOwner": "@CyberT33N" },
  "quality": { "config": "git-governance.quality.json", "schemaVersion": 4 },
  "tools": { "module": "tools/go.mod", "catalogVersion": 1 }
}`
}

// minimalBindingsWithToolchain extends the smallest manifest with the
// territory config-topic binding.
func minimalBindingsWithToolchain() string {
	return `{
  "schemaVersion": 2,
  "home": { "repository": "t33n-software/repository-governance", "sha": "89be739ee8a1d1ed6ebbe97dd1556a253477d242" },
  "class": { "qualityGates": "linux-only", "codeScanning": true, "licenseHub": false },
  "callers": [
    {
      "file": ".github/workflows/ci.yml",
      "master": "hosting-platforms/github/workflows/callers/go/ci.yml",
      "sha256": "f29a65bd73fe575b159123a9d4bebed86ab4eebe3c5dc5dac31c96e7fb7c4c4a"
    }
  ],
  "files": {
    "lefthook": { "path": "lefthook.yml", "sha256": "` + strings.Repeat("a", 64) + `" },
    "gitattributes": { "path": ".gitattributes", "sha256": "` + strings.Repeat("b", 64) + `" },
    "gitignore": { "path": ".gitignore", "fragments": ["core"], "sha256": "` + strings.Repeat("c", 64) + `" },
    "dependabot": { "path": ".github/dependabot.yml", "sha256": "` + strings.Repeat("d", 64) + `" }
  },
  "codeowners": { "path": ".github/CODEOWNERS", "defaultOwner": "@CyberT33N" },
  "quality": { "config": "git-governance.quality.json", "schemaVersion": 4 },
  "tools": { "module": "tools/go.mod", "catalogVersion": 1 },
  "toolchain": {
    "territory": { "repository": "t33n-software/node-quality-authority", "sha": "1234567890123456789012345678901234567890" },
    "registry": { "path": "configs/registry.json", "sha256": "` + strings.Repeat("1", 64) + `" },
    "category": "single-project/bundler-owned",
    "artifacts": [
      { "family": "tsconfig", "path": "tsconfig.base.json", "sha256": "` + strings.Repeat("2", 64) + `" }
    ],
    "pnpmWorkspace": { "path": "pnpm-workspace.yaml" },
    "sourceRoots": ["src"]
  }
}`
}
