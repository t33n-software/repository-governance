package canonical

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// provisionFixture binds the seams for the provisioning tests.
type provisionFixture struct {
	homeContents      map[string][]byte
	territoryContents map[string][]byte
	tenantContents    map[string][]byte
	written           map[string][]byte
	writeErr          error
	homeErr           error
	territoryErr      error
	tenantErr         error
}

// provisioner binds the fixture's seams; a tenant file that does not exist
// in the fixture reads as fs.ErrNotExist, mirroring the production seam.
func (fixture *provisionFixture) provisioner() Provisioner {
	return Provisioner{
		ReadHome: func(path string) ([]byte, error) {
			if fixture.homeErr != nil {
				return nil, fixture.homeErr
			}
			contents, found := fixture.homeContents[path]
			if !found {
				return nil, errors.New("no such home file: " + path)
			}
			return contents, nil
		},
		ReadTerritory: func(path string) ([]byte, error) {
			if fixture.territoryErr != nil {
				return nil, fixture.territoryErr
			}
			contents, found := fixture.territoryContents[path]
			if !found {
				return nil, errors.New("no such territory file: " + path)
			}
			return contents, nil
		},
		ReadTenant: func(path string) ([]byte, error) {
			if fixture.tenantErr != nil {
				return nil, fixture.tenantErr
			}
			contents, found := fixture.tenantContents[path]
			if !found {
				return nil, fs.ErrNotExist
			}
			return contents, nil
		},
		WriteTenant: func(path string, contents []byte) error {
			if fixture.writeErr != nil {
				return fixture.writeErr
			}
			if fixture.written == nil {
				fixture.written = make(map[string][]byte)
			}
			fixture.written[path] = contents
			return nil
		},
	}
}

// provisionTestBindings builds a complete binding set whose hashes match the
// fixture home tree, with the optional conventions binding.
func provisionTestBindings(t *testing.T, conventions *ConventionsBinding) Bindings {
	t.Helper()
	return Bindings{
		Home:  HomePin{Repository: "t33n-software/repository-governance", SHA: testHomeSHA},
		Class: Class{QualityGates: "linux-only"},
		Callers: []CallerBinding{{
			File:   ".github/workflows/ci.yml",
			Master: "hosting-platforms/github/workflows/callers/go/ci.yml",
			SHA256: Sum256Hex([]byte("caller-master")),
		}},
		Files: FileBindings{
			Lefthook:      FileBinding{Path: "lefthook.yml", SHA256: Sum256Hex([]byte("lefthook-core"))},
			Gitattributes: FileBinding{Path: ".gitattributes", SHA256: Sum256Hex([]byte("gitattributes-core"))},
			Gitignore: GitignoreBinding{
				Path:      ".gitignore",
				Fragments: []string{"core", "go/core"},
				SHA256:    Sum256Hex(gitignoreRenderedFixture(t)),
			},
			Dependabot: FileBinding{Path: ".github/dependabot.yml", SHA256: Sum256Hex([]byte("dependabot-core"))},
		},
		Codeowners:  CodeownersBinding{Path: ".github/CODEOWNERS", DefaultOwner: "@CyberT33N"},
		Conventions: conventions,
		Quality:     QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4},
		Tools:       ToolsBinding{Module: "tools/go.mod", CatalogVersion: 1},
	}
}

// passingProvisionFixture carries a fixture home whose every master hash
// matches the binding set of provisionTestBindings.
func passingProvisionFixture(t *testing.T) *provisionFixture {
	t.Helper()
	homeContents := map[string][]byte{
		"hosting-platforms/github/workflows/callers/go/ci.yml":        []byte("caller-master"),
		"hosting-platforms/github/files/lefthook/lefthook.yml":        []byte("lefthook-core"),
		"hosting-platforms/github/files/gitattributes/.gitattributes": []byte("gitattributes-core"),
		"hosting-platforms/github/files/dependabot/dependabot-go.yml": []byte("dependabot-core"),
		"hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl":   []byte("# contract\n\n* {{defaultOwner}}\n"),
	}
	for path, contents := range gitignoreFixtureHome() {
		homeContents[path] = contents
	}
	return &provisionFixture{homeContents: homeContents}
}

// materialByPath returns the materialization of one tenant path.
func materialByPath(t *testing.T, materials []Materialization, path string) Materialization {
	t.Helper()
	for _, material := range materials {
		if material.Path == path {
			return material
		}
	}
	t.Fatalf("no materialization for %s", path)
	return Materialization{}
}

func TestPlanMaterializesEveryBoundSurface(t *testing.T) {
	fixture := passingProvisionFixture(t)
	materials, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	rendered := gitignoreRenderedFixture(t)
	want := []Materialization{
		{Path: ".github/workflows/ci.yml", Contents: []byte("caller-master"), Source: "master hosting-platforms/github/workflows/callers/go/ci.yml"},
		{Path: "lefthook.yml", Contents: []byte("lefthook-core"), Source: "master hosting-platforms/github/files/lefthook/lefthook.yml"},
		{Path: ".gitattributes", Contents: []byte("gitattributes-core"), Source: "master hosting-platforms/github/files/gitattributes/.gitattributes"},
		{Path: ".github/dependabot.yml", Contents: []byte("dependabot-core"), Source: "master hosting-platforms/github/files/dependabot/dependabot-go.yml"},
		{Path: ".gitignore", Contents: rendered, Source: "render gitignore core + go/core"},
		{Path: ".github/CODEOWNERS", Contents: []byte("# contract\n\n* @CyberT33N\n"), Source: "render hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl"},
	}
	if len(materials) != len(want) {
		t.Fatalf("materials = %d, want %d", len(materials), len(want))
	}
	for index, material := range materials {
		expected := want[index]
		if material.Path != expected.Path || string(material.Contents) != string(expected.Contents) || material.Source != expected.Source {
			t.Fatalf("material %d = %+v, want %+v", index, material, expected)
		}
	}
}

func TestPlanRejectsADivergingCallerMasterHash(t *testing.T) {
	fixture := passingProvisionFixture(t)
	fixture.homeContents["hosting-platforms/github/workflows/callers/go/ci.yml"] = []byte("drifted")
	_, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
	if err == nil || !strings.Contains(err.Error(), "the canonical caller master") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanRejectsAnUnreadableCallerMaster(t *testing.T) {
	fixture := passingProvisionFixture(t)
	fixture.homeErr = errors.New("boom")
	_, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
	if err == nil || !strings.Contains(err.Error(), "read the canonical caller master") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanRejectsADivergingFileTopicHash(t *testing.T) {
	fixture := passingProvisionFixture(t)
	fixture.homeContents["hosting-platforms/github/files/lefthook/lefthook.yml"] = []byte("drifted")
	_, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
	if err == nil || !strings.Contains(err.Error(), "the canonical lefthook master") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanRejectsAnUnreadableFileTopicMaster(t *testing.T) {
	fixture := passingProvisionFixture(t)
	delete(fixture.homeContents, "hosting-platforms/github/files/lefthook/lefthook.yml")
	_, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
	if err == nil || !strings.Contains(err.Error(), "read the canonical lefthook master") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlanGitignore(t *testing.T) {
	rendered := gitignoreRenderedFixture(t)

	t.Run("provisions the rendered region alone for a new tenant", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		materials, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		if material := materialByPath(t, materials, ".gitignore"); string(material.Contents) != string(rendered) {
			t.Fatalf("gitignore contents = %q", string(material.Contents))
		}
	})

	t.Run("preserves the project block below the rendered region", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.tenantContents = map[string][]byte{
			".gitignore": append(slices.Clone(rendered), []byte("\n/dist-custom/\n")...),
		}
		materials, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		material := materialByPath(t, materials, ".gitignore")
		if want := string(rendered) + "\n/dist-custom/\n"; string(material.Contents) != want {
			t.Fatalf("gitignore contents = %q, want %q", string(material.Contents), want)
		}
	})

	t.Run("preserves the project block below the mark of a drifted file", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.tenantContents = map[string][]byte{
			".gitignore": []byte("# drifted region\n\n" + projectBlockMark + "\nproject/\n"),
		}
		materials, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		material := materialByPath(t, materials, ".gitignore")
		if want := string(rendered) + "project/\n"; string(material.Contents) != want {
			t.Fatalf("gitignore contents = %q, want %q", string(material.Contents), want)
		}
	})

	t.Run("refuses unmarked drifted content", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.tenantContents = map[string][]byte{".gitignore": []byte("# drifted region without a mark\n")}
		_, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
		if err == nil || !strings.Contains(err.Error(), "unmarked content is never overwritten") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects a diverging rendered region hash", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		bindings := provisionTestBindings(t, nil)
		bindings.Files.Gitignore.SHA256 = strings.Repeat("0", 64)
		_, err := fixture.provisioner().Plan(bindings)
		if err == nil || !strings.Contains(err.Error(), "the rendered gitignore governed region hashes") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects an unknown fragment", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		bindings := provisionTestBindings(t, nil)
		bindings.Files.Gitignore.Fragments = []string{"core", "unknown/area"}
		_, err := fixture.provisioner().Plan(bindings)
		if err == nil || !strings.Contains(err.Error(), "the bound gitignore fragments do not render") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("propagates a tenant read error", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.tenantErr = errors.New("boom")
		_, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
		if err == nil || !strings.Contains(err.Error(), "read the tenant .gitignore") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPlanCodeowners(t *testing.T) {
	t.Run("template read error", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		delete(fixture.homeContents, "hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl")
		_, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
		if err == nil || !strings.Contains(err.Error(), "read the canonical CODEOWNERS template") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("template without the render token", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.homeContents["hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl"] = []byte("* @someone\n")
		_, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
		if err == nil || !strings.Contains(err.Error(), "carries no {{defaultOwner}} token") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPlanConventions(t *testing.T) {
	conventions := &ConventionsBinding{
		Path:         "docs/conventions/hosting-platforms/github/rule-sets/README.md",
		Organization: "example-org",
		Repository:   "example-repository",
		Rationale:    "the fleet convention",
	}

	t.Run("skips the family where unbound", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		materials, err := fixture.provisioner().Plan(provisionTestBindings(t, nil))
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		for _, material := range materials {
			if material.Path == conventions.Path {
				t.Fatal("the conventions family must not be provisioned where unbound")
			}
		}
	})

	t.Run("renders the family where bound", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.homeContents["hosting-platforms/github/files/conventions/rule-sets-readme.md.tmpl"] = []byte("# {{organization}}/{{repository}}\n\n{{class}}\n\n{{platforms}}\n\n{{rationale}}\n")
		materials, err := fixture.provisioner().Plan(provisionTestBindings(t, conventions))
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		material := materialByPath(t, materials, conventions.Path)
		want := "# example-org/example-repository\n\nlinux-only\n\nThe quality gates run exclusively on **Linux**.\n\nthe fleet convention\n"
		if string(material.Contents) != want {
			t.Fatalf("conventions contents = %q, want %q", string(material.Contents), want)
		}
	})

	t.Run("rejects a class without a canonical render", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.homeContents["hosting-platforms/github/files/conventions/rule-sets-readme.md.tmpl"] = []byte("# {{organization}}\n")
		bindings := provisionTestBindings(t, conventions)
		bindings.Class.QualityGates = "pending"
		_, err := fixture.provisioner().Plan(bindings)
		if err == nil || !strings.Contains(err.Error(), "the conventions template does not render") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("template read error", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		_, err := fixture.provisioner().Plan(provisionTestBindings(t, conventions))
		if err == nil || !strings.Contains(err.Error(), "read the canonical conventions template") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestApplyWritesTheProvenMaterializations(t *testing.T) {
	fixture := passingProvisionFixture(t)
	materials, err := fixture.provisioner().Apply(provisionTestBindings(t, nil))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(materials) != 6 || len(fixture.written) != 6 {
		t.Fatalf("materials = %d, written = %d", len(materials), len(fixture.written))
	}
	if string(fixture.written[".github/workflows/ci.yml"]) != "caller-master" {
		t.Fatalf("written caller = %q", string(fixture.written[".github/workflows/ci.yml"]))
	}
	if string(fixture.written[".github/CODEOWNERS"]) != "# contract\n\n* @CyberT33N\n" {
		t.Fatalf("written codeowners = %q", string(fixture.written[".github/CODEOWNERS"]))
	}
}

func TestApplyFailsClosedBeforeAnyWrite(t *testing.T) {
	// A failing proof in the last planned surface leaves the tenant
	// untouched: nothing is written when any proof fails.
	fixture := passingProvisionFixture(t)
	fixture.homeContents["hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl"] = []byte("* @someone\n")
	_, err := fixture.provisioner().Apply(provisionTestBindings(t, nil))
	if err == nil || !strings.Contains(err.Error(), "carries no {{defaultOwner}} token") {
		t.Fatalf("err = %v", err)
	}
	if len(fixture.written) != 0 {
		t.Fatalf("written = %d, want 0", len(fixture.written))
	}
}

func TestApplyReportsWriteFailure(t *testing.T) {
	fixture := passingProvisionFixture(t)
	fixture.writeErr = errors.New("boom")
	_, err := fixture.provisioner().Apply(provisionTestBindings(t, nil))
	if err == nil || !strings.Contains(err.Error(), "write .github/workflows/ci.yml: boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewProvisionerReportsAnUnwritableParent(t *testing.T) {
	home := t.TempDir()
	tenant := t.TempDir()
	for path, contents := range passingProvisionFixture(t).homeContents {
		target := filepath.Join(home, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A regular file where the caller's parent directory belongs makes the
	// write seam fail closed with the exact path.
	if err := os.WriteFile(filepath.Join(tenant, ".github"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewProvisioner(tenant, home, t.TempDir()).Apply(provisionTestBindings(t, nil))
	if err == nil || !strings.Contains(err.Error(), "write .github/workflows/ci.yml") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewProvisionerBindsProductionSeams(t *testing.T) {
	home := t.TempDir()
	territory := t.TempDir()
	tenant := t.TempDir()
	for path, contents := range passingProvisionFixture(t).homeContents {
		target := filepath.Join(home, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for path, contents := range territoryFixtureContents() {
		target := filepath.Join(territory, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	bindings := provisionTestBindings(t, nil)
	bindings.Toolchain = provisionTestToolchain(t)
	materials, err := NewProvisioner(tenant, home, territory).Apply(bindings)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(materials) != 10 {
		t.Fatalf("materials = %d", len(materials))
	}

	// The nested caller path proves the write seam creates parent directories.
	written, err := os.ReadFile(filepath.Join(tenant, ".github", "workflows", "ci.yml"))
	if err != nil || string(written) != "caller-master" {
		t.Fatalf("the nested caller was not written: %v", err)
	}
	if written, err := os.ReadFile(filepath.Join(tenant, ".gitignore")); err != nil || !strings.HasPrefix(string(written), "# canonical: gitignore") {
		t.Fatalf("the gitignore was not written: %v", err)
	}
	// The territory artifact proves the territory read seam is production-bound.
	want := string(territoryFixtureContents()["configs/tsconfig/"+toolchainCategory+"/tsconfig.node.json"])
	artifact, err := os.ReadFile(filepath.Join(tenant, "tsconfig.node.json"))
	if err != nil || string(artifact) != want {
		t.Fatalf("the territory artifact was not written: %v", err)
	}
}

// toolchainCategory is the fixture category the territory tree carries.
const toolchainCategory = "single-project/direct-node"

// territoryFixtureContents builds a territory tree whose pinned registry,
// config artifacts, and pnpm baseline match provisionTestToolchain.
func territoryFixtureContents() map[string][]byte {
	registry := []byte(`{
  "schemaVersion": 1,
  "categories": [
    {
      "id": "single-project/direct-node",
      "title": "Direct Node single project",
      "artifacts": {
        "tsconfig": "configs/tsconfig/single-project/direct-node/",
        "vitest": "configs/vitest/single-project/direct-node/",
        "tsdown": "configs/tsdown/single-project/direct-node/"
      },
      "proof": { "baseByteIdentity": true, "leafInvariants": [], "behaviorGate": [] }
    }
  ]
}`)
	return map[string][]byte{
		"configs/registry.json": registry,
		"configs/tsconfig/" + toolchainCategory + "/tsconfig.node.json": []byte("{\n  \"compilerOptions\": {\n    \"module\": \"Node20\",\n    \"moduleResolution\": \"Node16\",\n    \"noEmit\": false\n  },\n  \"include\": [\n    \"./src\"\n  ]\n}\n"),
		"configs/vitest/" + toolchainCategory + "/vitest.config.ts":     []byte("export const vitest = 'fixture'\n"),
		"configs/tsdown/" + toolchainCategory + "/tsdown.config.ts":     []byte("export const tsdown = 'fixture'\n"),
		"configs/pnpm/pnpm-workspace.base.yaml":                         []byte("catalogMode: strict\n"),
	}
}

// provisionTestToolchain builds the toolchain binding whose hashes match the
// territory fixture tree.
func provisionTestToolchain(t *testing.T) *ToolchainBindings {
	t.Helper()
	territory := territoryFixtureContents()
	hash := func(path string) string {
		return Sum256Hex(territory[path])
	}
	return &ToolchainBindings{
		Territory: TerritoryPin{Repository: "t33n-software/go-quality-authority", SHA: testTerritorySHA},
		Registry:  RegistryPin{Path: "configs/registry.json", SHA256: hash("configs/registry.json")},
		Category:  toolchainCategory,
		Artifacts: []ArtifactBinding{
			{Family: "tsconfig", Path: "tsconfig.node.json", SHA256: hash("configs/tsconfig/" + toolchainCategory + "/tsconfig.node.json")},
			{Family: "vitest", Path: "vitest.config.ts", SHA256: hash("configs/vitest/" + toolchainCategory + "/vitest.config.ts")},
			{Family: "tsdown", Path: "tsdown.config.ts", SHA256: hash("configs/tsdown/" + toolchainCategory + "/tsdown.config.ts")},
		},
		PnpmWorkspace: PnpmWorkspaceBinding{Path: "pnpm-workspace.yaml"},
		SourceRoots:   []string{"src"},
	}
}

// provisionToolchainBindings extends the passing binding set with the
// toolchain binding.
func provisionToolchainBindings(t *testing.T) Bindings {
	t.Helper()
	bindings := provisionTestBindings(t, nil)
	bindings.Toolchain = provisionTestToolchain(t)
	return bindings
}

func TestPlanToolchainArtifacts(t *testing.T) {
	t.Run("materializes the pinned territory artifacts", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		materials, err := fixture.provisioner().Plan(provisionToolchainBindings(t))
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		territory := territoryFixtureContents()
		for _, artifact := range []struct{ path, source string }{
			{"tsconfig.node.json", "territory configs/tsconfig/single-project/direct-node/tsconfig.node.json"},
			{"vitest.config.ts", "territory configs/vitest/single-project/direct-node/vitest.config.ts"},
			{"tsdown.config.ts", "territory configs/tsdown/single-project/direct-node/tsdown.config.ts"},
		} {
			material := materialByPath(t, materials, artifact.path)
			if want := string(territory[strings.TrimPrefix(artifact.source, "territory ")]); string(material.Contents) != want {
				t.Fatalf("%s contents = %q, want the territory bytes %q", artifact.path, string(material.Contents), want)
			}
			if material.Source != artifact.source {
				t.Fatalf("%s source = %q, want %q", artifact.path, material.Source, artifact.source)
			}
		}
	})

	t.Run("rejects an unreadable pinned registry", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		delete(fixture.territoryContents, "configs/registry.json")
		_, err := fixture.provisioner().Plan(provisionToolchainBindings(t))
		if err == nil || !strings.Contains(err.Error(), "read the pinned registry configs/registry.json") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects a diverging pinned registry hash", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		bindings := provisionToolchainBindings(t)
		bindings.Toolchain.Registry.SHA256 = strings.Repeat("0", 64)
		_, err := fixture.provisioner().Plan(bindings)
		if err == nil || !strings.Contains(err.Error(), "the pinned registry hash "+Sum256Hex(territoryFixtureContents()["configs/registry.json"])+" diverges from the bound hash") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects an unregistered category", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		bindings := provisionToolchainBindings(t)
		bindings.Toolchain.Category = "monorepo"
		_, err := fixture.provisioner().Plan(bindings)
		if err == nil || !strings.Contains(err.Error(), "carries no entry for the declared category") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects a family without a registry mapping", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		bindings := provisionToolchainBindings(t)
		bindings.Toolchain.Artifacts = append(bindings.Toolchain.Artifacts,
			ArtifactBinding{Family: "eslint", Path: "eslint.config.js", SHA256: strings.Repeat("e", 64)})
		_, err := fixture.provisioner().Plan(bindings)
		if err == nil || !strings.Contains(err.Error(), `carries no artifact mapping for the family "eslint"`) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects a registry mapping that diverges from the category identity", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		// The registry maps the vitest family to the tsconfig folder while
		// the category identity derives configs/vitest/<category>/.
		diverged := []byte(`{
  "schemaVersion": 1,
  "categories": [
    {
      "id": "single-project/direct-node",
      "title": "Direct Node single project",
      "artifacts": {
        "tsconfig": "configs/tsconfig/single-project/direct-node/",
        "vitest": "configs/tsconfig/single-project/direct-node/",
        "tsdown": "configs/tsdown/single-project/direct-node/"
      },
      "proof": { "baseByteIdentity": true, "leafInvariants": [], "behaviorGate": [] }
    }
  ]
}`)
		fixture.territoryContents = territoryFixtureContents()
		fixture.territoryContents["configs/registry.json"] = diverged
		bindings := provisionToolchainBindings(t)
		bindings.Toolchain.Registry.SHA256 = Sum256Hex(diverged)
		_, err := fixture.provisioner().Plan(bindings)
		if err == nil || !strings.Contains(err.Error(), "but the category identity derives") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects an unreadable territory artifact", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		delete(fixture.territoryContents, "configs/tsconfig/"+toolchainCategory+"/tsconfig.node.json")
		_, err := fixture.provisioner().Plan(provisionToolchainBindings(t))
		if err == nil || !strings.Contains(err.Error(), "read the territory tsconfig artifact") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects a diverging territory artifact hash", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		fixture.territoryContents["configs/tsconfig/"+toolchainCategory+"/tsconfig.node.json"] = []byte("drifted")
		_, err := fixture.provisioner().Plan(provisionToolchainBindings(t))
		if err == nil || !strings.Contains(err.Error(), "not the bound") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPlanPnpmWorkspace(t *testing.T) {
	t.Run("provisions the baseline with an empty catalog for a fresh tenant", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		materials, err := fixture.provisioner().Plan(provisionToolchainBindings(t))
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		material := materialByPath(t, materials, "pnpm-workspace.yaml")
		if want := "catalog: {}\ncatalogMode: strict\n"; string(material.Contents) != want {
			t.Fatalf("pnpm workspace = %q, want %q", string(material.Contents), want)
		}
		if material.Source != "compose configs/pnpm/pnpm-workspace.base.yaml" {
			t.Fatalf("source = %q", material.Source)
		}
	})

	t.Run("preserves the tenant keys and heals the governed keys", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		fixture.tenantContents = map[string][]byte{
			"pnpm-workspace.yaml": []byte("catalog:\n  react: 19.0.0\ncatalogMode: loose\npackages:\n  - apps/*\n"),
		}
		materials, err := fixture.provisioner().Plan(provisionToolchainBindings(t))
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		material := materialByPath(t, materials, "pnpm-workspace.yaml")
		want := "catalog:\n    react: 19.0.0\ncatalogMode: strict\npackages:\n    - apps/*\n"
		if string(material.Contents) != want {
			t.Fatalf("pnpm workspace = %q, want %q", string(material.Contents), want)
		}
	})

	t.Run("rejects an unparseable tenant document", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		fixture.tenantContents = map[string][]byte{
			"pnpm-workspace.yaml": []byte("just a scalar\n"),
		}
		_, err := fixture.provisioner().Plan(provisionToolchainBindings(t))
		if err == nil || !strings.Contains(err.Error(), "must be a valid YAML mapping; unparseable content is never overwritten") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects an unreadable baseline", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		delete(fixture.territoryContents, "configs/pnpm/pnpm-workspace.base.yaml")
		_, err := fixture.provisioner().Plan(provisionToolchainBindings(t))
		if err == nil || !strings.Contains(err.Error(), "read the territory pnpm fortress baseline") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rejects an unparseable baseline", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		fixture.territoryContents["configs/pnpm/pnpm-workspace.base.yaml"] = []byte("just a scalar\n")
		_, err := fixture.provisioner().Plan(provisionToolchainBindings(t))
		if err == nil || !strings.Contains(err.Error(), "the territory pnpm fortress baseline configs/pnpm/pnpm-workspace.base.yaml must be a valid YAML mapping") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("propagates a tenant read error", func(t *testing.T) {
		fixture := passingProvisionFixture(t)
		fixture.territoryContents = territoryFixtureContents()
		provisioner := fixture.provisioner()
		provisioner.ReadTenant = func(path string) ([]byte, error) {
			if path == "pnpm-workspace.yaml" {
				return nil, errors.New("boom")
			}
			return nil, fs.ErrNotExist
		}
		_, err := provisioner.Plan(provisionToolchainBindings(t))
		if err == nil || !strings.Contains(err.Error(), "read the tenant pnpm-workspace.yaml: boom") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestApplyToolchainWritesTheProvenMaterializations(t *testing.T) {
	fixture := passingProvisionFixture(t)
	fixture.territoryContents = territoryFixtureContents()
	bindings := provisionToolchainBindings(t)
	materials, err := fixture.provisioner().Apply(bindings)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(materials) != 10 || len(fixture.written) != 10 {
		t.Fatalf("materials = %d, written = %d", len(materials), len(fixture.written))
	}
	if want := string(territoryFixtureContents()["configs/tsconfig/"+toolchainCategory+"/tsconfig.node.json"]); string(fixture.written["tsconfig.node.json"]) != want {
		t.Fatalf("the written tsconfig is not the territory artifact: %q", string(fixture.written["tsconfig.node.json"]))
	}

	// The composed re-render is idempotent: feeding the written bytes back
	// through the tenant read seam composes the same document again.
	fixture.tenantContents = fixture.written
	fixture.written = nil
	repeated, err := fixture.provisioner().Apply(bindings)
	if err != nil {
		t.Fatalf("repeated Apply: %v", err)
	}
	first := materialByPath(t, materials, "pnpm-workspace.yaml")
	second := materialByPath(t, repeated, "pnpm-workspace.yaml")
	if string(first.Contents) != string(second.Contents) {
		t.Fatalf("the composed pnpm workspace is not idempotent: %q != %q", string(first.Contents), string(second.Contents))
	}
}
