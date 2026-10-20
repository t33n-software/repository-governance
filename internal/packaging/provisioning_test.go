package packaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/t33n-software/repository-governance/internal/canonical"
)

// TestProvisionedTenantPassesTheConformanceVerifier proves the identity of
// the two exposures of one core truth: the tenant surfaces provisioned by
// the home's provisioning CLI pass the conformance verifier byte for byte
// against the same pinned home tree. The write exposure never re-implements
// a proof the verify exposure owns; this contract test binds the identity.
func TestProvisionedTenantPassesTheConformanceVerifier(t *testing.T) {
	home := repoRoot(t)
	manifest := readArtifact(t, "repo-bindings.json")
	bindings, err := canonical.DecodeBindings([]byte(manifest))
	if err != nil {
		t.Fatalf("the home's own binding manifest must decode: %v", err)
	}

	tenant := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tenant, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Tenant-authored data the provisioning CLI never writes: the binding
	// manifest, the config seam, and the module declarations.
	tenantData := map[string]string{
		"repo-bindings.json":             manifest,
		"git-governance.quality.json":    readArtifact(t, "git-governance.quality.json"),
		"go.mod":                         "module example.test/tenant\n\ngo 1.26.6\n\ntoolchain go1.26.6\n",
		filepath.Join("tools", "go.mod"): "module example.test/tenant/tools\n\ngo 1.26.6\n",
	}
	for path, contents := range tenantData {
		if err := os.WriteFile(filepath.Join(tenant, path), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	provisioner := canonical.NewProvisioner(tenant, home, t.TempDir())
	materials, err := provisioner.Apply(bindings)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(materials) != len(bindings.Callers)+5 {
		t.Fatalf("materials = %d, want %d", len(materials), len(bindings.Callers)+5)
	}

	// The verifier runs with the module seams stubbed: the fixture declares
	// no capability packs and no tool pins, so no module resolution runs.
	verifier := canonical.Verifier{
		TenantRoot: tenant,
		ReadTenant: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(tenant, filepath.FromSlash(path)))
		},
		ReadHome: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(home, filepath.FromSlash(path)))
		},
		ReadModule: func(dir, path string) ([]byte, error) {
			return nil, errors.New("module seams are stubbed")
		},
		ListTenant: func(path string) ([]fs.DirEntry, error) {
			return os.ReadDir(filepath.Join(tenant, filepath.FromSlash(path)))
		},
		ListModule: func(dir, path string) ([]fs.DirEntry, error) {
			return nil, errors.New("module seams are stubbed")
		},
		ResolveModule: func(context.Context, string, string) (string, error) {
			return "", errors.New("module seams are stubbed")
		},
		RunTool: func(context.Context, string, ...string) (string, error) {
			return "", errors.New("module seams are stubbed")
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	findings := verifier.Verify(context.Background(), bindings)
	if len(findings) > 0 {
		t.Fatalf("the provisioned tenant must pass the conformance verifier:\n%v", findings)
	}
}

// TestProvisionedToolchainTenantPassesTheConformanceVerifier extends the
// identity proof to the toolchain family: the provisioned territory config
// artifacts and the composed pnpm workspace pass the conformance verifier
// including the category proofs — the pinned registry membership, the byte
// identity against both trees, the delivery-lane guards, and the pnpm
// policy invariants.
func TestProvisionedToolchainTenantPassesTheConformanceVerifier(t *testing.T) {
	home := repoRoot(t)
	const callerHashesRecord = "hosting-platforms/github/workflows/callers/go/caller-hashes.json"
	var published struct {
		Home struct {
			SHA string `json:"sha"`
		} `json:"home"`
	}
	if err := json.Unmarshal([]byte(readArtifact(t, callerHashesRecord)), &published); err != nil {
		t.Fatalf("the caller-hashes record is not valid JSON: %v", err)
	}
	homeSHA := published.Home.SHA

	category := "single-project/direct-node"
	tsconfig := "{\n  \"compilerOptions\": {\n    \"module\": \"Node20\",\n    \"moduleResolution\": \"Node16\",\n    \"noEmit\": false\n  },\n  \"include\": [\n    \"./src\"\n  ]\n}\n"
	vitest := "export const vitest = 'fixture'\n"
	tsdown := "export const tsdown = 'fixture'\n"
	registry := fmt.Sprintf(`{
  "schemaVersion": 1,
  "categories": [
    {
      "id": %q,
      "title": "Direct Node single project",
      "artifacts": {
        "tsconfig": "configs/tsconfig/%s/",
        "vitest": "configs/vitest/%s/",
        "tsdown": "configs/tsdown/%s/"
      },
      "proof": { "baseByteIdentity": true, "leafInvariants": [], "behaviorGate": [] }
    }
  ]
}`, category, category, category, category)
	baseline := "catalogMode: strict\n"

	territory := t.TempDir()
	territoryFiles := map[string]string{
		"configs/registry.json":                                registry,
		"configs/tsconfig/" + category + "/tsconfig.node.json": tsconfig,
		"configs/vitest/" + category + "/vitest.config.ts":     vitest,
		"configs/tsdown/" + category + "/tsdown.config.ts":     tsdown,
		"configs/pnpm/pnpm-workspace.base.yaml":                baseline,
	}
	for path, contents := range territoryFiles {
		target := filepath.Join(territory, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	rendered, err := canonical.RenderGitignoreGovernedRegion(readHomeArtifact(t), []string{"core"}, homeSHA)
	if err != nil {
		t.Fatalf("the core fragments must render at the bound pin: %v", err)
	}
	hash := func(contents string) string {
		return canonical.Sum256Hex([]byte(contents))
	}
	manifest := fmt.Sprintf(`{
  "schemaVersion": 2,
  "home": { "repository": "t33n-software/repository-governance", "sha": %q },
  "class": { "qualityGates": "linux-only", "codeScanning": false, "licenseHub": false },
  "callers": [
    { "file": ".github/workflows/ci.yml", "master": "hosting-platforms/github/workflows/callers/go/ci.yml", "sha256": %q }
  ],
  "files": {
    "lefthook": { "path": "lefthook.yml", "sha256": %q },
    "gitattributes": { "path": ".gitattributes", "sha256": %q },
    "gitignore": { "path": ".gitignore", "fragments": ["core"], "sha256": %q },
    "dependabot": { "path": ".github/dependabot.yml", "sha256": %q }
  },
  "codeowners": { "path": ".github/CODEOWNERS", "defaultOwner": "@CyberT33N" },
  "quality": { "config": "git-governance.quality.json", "schemaVersion": 4 },
  "tools": { "module": "tools/go.mod", "catalogVersion": 1 },
  "toolchain": {
    "territory": { "repository": "t33n-software/go-quality-authority", "sha": "0123456789abcdef0123456789abcdef01234567" },
    "registry": { "path": "configs/registry.json", "sha256": %q },
    "category": %q,
    "artifacts": [
      { "family": "tsconfig", "path": "tsconfig.node.json", "sha256": %q },
      { "family": "vitest", "path": "vitest.config.ts", "sha256": %q },
      { "family": "tsdown", "path": "tsdown.config.ts", "sha256": %q }
    ],
    "pnpmWorkspace": { "path": "pnpm-workspace.yaml" },
    "sourceRoots": ["src"]
  }
}`,
		homeSHA,
		hash(readArtifact(t, "hosting-platforms/github/workflows/callers/go/ci.yml")),
		hash(readArtifact(t, "hosting-platforms/github/files/lefthook/lefthook.yml")),
		hash(readArtifact(t, "hosting-platforms/github/files/gitattributes/.gitattributes")),
		hash(string(rendered)),
		hash(readArtifact(t, "hosting-platforms/github/files/dependabot/dependabot-go.yml")),
		hash(registry),
		category,
		hash(tsconfig),
		hash(vitest),
		hash(tsdown),
	)
	bindings, err := canonical.DecodeBindings([]byte(manifest))
	if err != nil {
		t.Fatalf("the synthetic toolchain manifest must decode: %v", err)
	}

	tenant := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tenant, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Tenant-authored data the provisioning CLI never writes: the binding
	// manifest, the config seam with the toolchain identity, and the module
	// declarations.
	quality := `{
  "schemaVersion": 4,
  "toolchain": { "language": "node-typescript", "category": "single-project/direct-node" }
}`
	tenantData := map[string]string{
		"repo-bindings.json":             manifest,
		"git-governance.quality.json":    quality,
		"go.mod":                         "module example.test/tenant\n\ngo 1.26.6\n\ntoolchain go1.26.6\n",
		filepath.Join("tools", "go.mod"): "module example.test/tenant/tools\n\ngo 1.26.6\n",
	}
	for path, contents := range tenantData {
		if err := os.WriteFile(filepath.Join(tenant, path), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	provisioner := canonical.NewProvisioner(tenant, home, territory)
	materials, err := provisioner.Apply(bindings)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(materials) != len(bindings.Callers)+9 {
		t.Fatalf("materials = %d, want %d", len(materials), len(bindings.Callers)+9)
	}

	// The verifier runs with the module seams stubbed: the fixture declares
	// no capability packs and no tool pins, so no module resolution runs.
	verifier := canonical.Verifier{
		TenantRoot: tenant,
		ReadTenant: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(tenant, filepath.FromSlash(path)))
		},
		ReadHome: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(home, filepath.FromSlash(path)))
		},
		ReadTerritory: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(territory, filepath.FromSlash(path)))
		},
		ReadModule: func(dir, path string) ([]byte, error) {
			return nil, errors.New("module seams are stubbed")
		},
		ListTenant: func(path string) ([]fs.DirEntry, error) {
			return os.ReadDir(filepath.Join(tenant, filepath.FromSlash(path)))
		},
		ListModule: func(dir, path string) ([]fs.DirEntry, error) {
			return nil, errors.New("module seams are stubbed")
		},
		ResolveModule: func(context.Context, string, string) (string, error) {
			return "", errors.New("module seams are stubbed")
		},
		RunTool: func(context.Context, string, ...string) (string, error) {
			return "", errors.New("module seams are stubbed")
		},
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	if findings := verifier.Verify(context.Background(), bindings); len(findings) > 0 {
		t.Fatalf("the provisioned toolchain tenant must pass the conformance verifier:\n%v", findings)
	}
}
