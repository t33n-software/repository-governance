package main

import (
	"context"
	"errors"
	"io/fs"
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
	if !strings.Contains(stdout.String(), "provision-canonical") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunUsageError(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--bogus"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run --bogus = %d", code)
	}
}

func TestRunAcceptsTheSpaceSeparatedFlagForm(t *testing.T) {
	// The tenant invocation uses the space-separated form; the read error
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
	writeManifest(t, dir, "not json")
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
	writeManifest(t, dir, minimalBindings())
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir}, &stdout, &stderr); code != 1 {
		t.Fatalf("run with a home resolution error = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no module") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunDryRunPlan(t *testing.T) {
	defer func() { planTenant = planTenantMaterials }()
	dir := t.TempDir()
	writeManifest(t, dir, minimalBindings())
	planTenant = func(canonical.Provisioner, canonical.Bindings) ([]canonical.Materialization, error) {
		return []canonical.Materialization{{Path: ".gitignore", Contents: []byte("core"), Source: "render gitignore core"}}, nil
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home", t.TempDir(), "--dry-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run --dry-run = %d (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "Canonical provisioning plan: no files written (1)") ||
		!strings.Contains(out, "would write .gitignore (from render gitignore core)") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestRunDryRunPlanError(t *testing.T) {
	defer func() { planTenant = planTenantMaterials }()
	dir := t.TempDir()
	writeManifest(t, dir, minimalBindings())
	planTenant = func(canonical.Provisioner, canonical.Bindings) ([]canonical.Materialization, error) {
		return nil, errors.New("boom")
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir(), "--dry-run"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run --dry-run with a plan error = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunConfirmationRequiredInANonInteractiveContext(t *testing.T) {
	defer func() { stdinIsTerminal = detectTerminal }()
	dir := t.TempDir()
	writeManifest(t, dir, minimalBindings())
	stdinIsTerminal = func() bool { return false }
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir()}, &stdout, &stderr); code != 2 {
		t.Fatalf("run without a confirmation = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "requires an explicit confirmation") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunInteractiveConfirmationAccepts(t *testing.T) {
	defer func() {
		planTenant = planTenantMaterials
		applyTenant = applyTenantMaterials
		stdinIsTerminal = detectTerminal
		stdinReader = os.Stdin
	}()
	dir := t.TempDir()
	writeManifest(t, dir, minimalBindings())
	stdinIsTerminal = func() bool { return true }
	stdinReader = strings.NewReader("y\n")
	planTenant = func(canonical.Provisioner, canonical.Bindings) ([]canonical.Materialization, error) {
		return []canonical.Materialization{{Path: ".gitignore", Source: "render gitignore core"}}, nil
	}
	applied := false
	applyTenant = func(canonical.Provisioner, canonical.Bindings) ([]canonical.Materialization, error) {
		applied = true
		return []canonical.Materialization{{Path: ".gitignore"}}, nil
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir()}, &stdout, &stderr); code != 0 {
		t.Fatalf("run with an accepted confirmation = %d (stderr: %s)", code, stderr.String())
	}
	if !applied {
		t.Fatal("the apply seam was not invoked")
	}
	out := stdout.String()
	if !strings.Contains(out, "Apply the provisioning? [y/N] ") || !strings.Contains(out, "Canonical provisioning: PASS (1 files written)") {
		t.Fatalf("stdout = %q", out)
	}
}

func TestRunInteractiveConfirmationDeclines(t *testing.T) {
	defer func() {
		planTenant = planTenantMaterials
		applyTenant = applyTenantMaterials
		stdinIsTerminal = detectTerminal
		stdinReader = os.Stdin
	}()
	dir := t.TempDir()
	writeManifest(t, dir, minimalBindings())
	stdinIsTerminal = func() bool { return true }
	stdinReader = strings.NewReader("n\n")
	planTenant = func(canonical.Provisioner, canonical.Bindings) ([]canonical.Materialization, error) {
		return []canonical.Materialization{{Path: ".gitignore"}}, nil
	}
	applyTenant = func(canonical.Provisioner, canonical.Bindings) ([]canonical.Materialization, error) {
		return nil, errors.New("must not run")
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir()}, &stdout, &stderr); code != 2 {
		t.Fatalf("run with a declined confirmation = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "the confirmation was declined") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunInteractivePlanError(t *testing.T) {
	defer func() { planTenant = planTenantMaterials; stdinIsTerminal = detectTerminal }()
	dir := t.TempDir()
	writeManifest(t, dir, minimalBindings())
	stdinIsTerminal = func() bool { return true }
	planTenant = func(canonical.Provisioner, canonical.Bindings) ([]canonical.Materialization, error) {
		return nil, errors.New("boom")
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir()}, &stdout, &stderr); code != 1 {
		t.Fatalf("run with an interactive plan error = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunApplyError(t *testing.T) {
	defer func() { applyTenant = applyTenantMaterials }()
	dir := t.TempDir()
	writeManifest(t, dir, minimalBindings())
	applyTenant = func(canonical.Provisioner, canonical.Bindings) ([]canonical.Materialization, error) {
		return nil, errors.New("boom")
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir(), "--yes"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run with an apply error = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunApplySuccess(t *testing.T) {
	defer func() { applyTenant = applyTenantMaterials }()
	dir := t.TempDir()
	writeManifest(t, dir, minimalBindings())
	applyTenant = func(_ canonical.Provisioner, bindings canonical.Bindings) ([]canonical.Materialization, error) {
		if bindings.Home.Repository != "t33n-software/repository-governance" {
			t.Fatalf("Home.Repository = %q", bindings.Home.Repository)
		}
		return []canonical.Materialization{{Path: ".gitignore"}, {Path: ".github/CODEOWNERS"}}, nil
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir(), "--yes"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote .gitignore") || !strings.Contains(stdout.String(), "Canonical provisioning: PASS (2 files written)") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestPlanTenantDelegation(t *testing.T) {
	// The default planning seam invokes the provisioner's Plan; the failing
	// read seam surfaces its error through the delegation.
	provisioner := canonical.Provisioner{
		ReadHome:    func(string) ([]byte, error) { return nil, errors.New("boom") },
		ReadTenant:  func(string) ([]byte, error) { return nil, fs.ErrNotExist },
		WriteTenant: func(string, []byte) error { return errors.New("boom") },
	}
	if _, err := planTenantMaterials(provisioner, canonical.Bindings{}); err == nil {
		t.Fatal("expected the delegated plan error")
	}
}

func TestApplyTenantDelegation(t *testing.T) {
	provisioner := canonical.Provisioner{
		ReadHome:    func(string) ([]byte, error) { return nil, errors.New("boom") },
		ReadTenant:  func(string) ([]byte, error) { return nil, fs.ErrNotExist },
		WriteTenant: func(string, []byte) error { return errors.New("boom") },
	}
	if _, err := applyTenantMaterials(provisioner, canonical.Bindings{}); err == nil {
		t.Fatal("expected the delegated apply error")
	}
}

func TestMain(t *testing.T) {
	defer func() { exitProcess = os.Exit; commandArgs = os.Args }()
	var code int
	exitProcess = func(c int) { code = c }
	commandArgs = []string{"provision-canonical", "--version"}
	main()
	if code != 0 {
		t.Fatalf("main exit = %d", code)
	}
}

func TestDetectTerminal(t *testing.T) {
	// The production terminal detection runs against the test runner's real
	// standard input: it reports a terminal exactly when the mode marks a
	// character device and the stat call succeeds.
	info, err := os.Stdin.Stat()
	if err != nil {
		t.Skipf("standard input is not statable: %v", err)
	}
	want := info.Mode()&os.ModeCharDevice != 0
	if detectTerminal() != want {
		t.Fatalf("detectTerminal = %v, want %v", !want, want)
	}
}

// writeManifest writes the manifest fixture into a tenant directory.
func writeManifest(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "repo-bindings.json"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
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

// toolchainBindingsManifest extends the smallest manifest with a toolchain
// section; the hash forms satisfy the decoder while the plan and apply seams
// stay overridden in the tests that use it.
func toolchainBindingsManifest() string {
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
    "territory": { "repository": "t33n-software/go-quality-authority", "sha": "89be739ee8a1d1ed6ebbe97dd1556a253477d242" },
    "registry": { "path": "configs/registry.json", "sha256": "` + strings.Repeat("1", 64) + `" },
    "category": "single-project/direct-node",
    "artifacts": [
      { "family": "tsconfig", "path": "tsconfig.node.json", "sha256": "` + strings.Repeat("2", 64) + `" }
    ],
    "pnpmWorkspace": { "path": "pnpm-workspace.yaml" },
    "sourceRoots": ["src"]
  }
}`
}

func TestRunTerritoryResolutionError(t *testing.T) {
	// A toolchain binding without the territory flag is a fail-closed
	// resolution error: the config topics would claim writes no seam carries.
	dir := t.TempDir()
	writeManifest(t, dir, toolchainBindingsManifest())
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir()}, &stdout, &stderr); code != 1 {
		t.Fatalf("run without the territory flag = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "the binding manifest binds a toolchain section; pass --territory-home <path>") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunTerritoryFlagWins(t *testing.T) {
	defer func() { planTenant = planTenantMaterials }()
	territory := t.TempDir()
	if err := os.WriteFile(filepath.Join(territory, "probe.txt"), []byte("territory"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeManifest(t, dir, toolchainBindingsManifest())
	var probed string
	planTenant = func(provisioner canonical.Provisioner, bindings canonical.Bindings) ([]canonical.Materialization, error) {
		contents, err := provisioner.ReadTerritory("probe.txt")
		if err != nil {
			return nil, err
		}
		probed = string(contents)
		return nil, nil
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir(), "--territory-home", territory, "--dry-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run = %d (stderr: %s)", code, stderr.String())
	}
	if probed != "territory" {
		t.Fatalf("the territory seam read %q", probed)
	}
}

func TestRunTerritoryAssignmentForm(t *testing.T) {
	defer func() { planTenant = planTenantMaterials }()
	territory := t.TempDir()
	if err := os.WriteFile(filepath.Join(territory, "probe.txt"), []byte("territory"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeManifest(t, dir, toolchainBindingsManifest())
	var probed string
	planTenant = func(provisioner canonical.Provisioner, bindings canonical.Bindings) ([]canonical.Materialization, error) {
		contents, err := provisioner.ReadTerritory("probe.txt")
		if err != nil {
			return nil, err
		}
		probed = string(contents)
		return nil, nil
	}
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--repo=" + dir, "--home=" + t.TempDir(), "--territory-home=" + territory, "--dry-run"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run with the assignment form = %d (stderr: %s)", code, stderr.String())
	}
	if probed != "territory" {
		t.Fatalf("the territory seam read %q", probed)
	}
}

func TestRunTerritoryFlagMissingValue(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := run(context.Background(), []string{"--territory-home"}, &stdout, &stderr); code != 2 {
		t.Fatalf("run with a missing territory value = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
