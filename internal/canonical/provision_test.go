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
	homeContents   map[string][]byte
	tenantContents map[string][]byte
	written        map[string][]byte
	writeErr       error
	homeErr        error
	tenantErr      error
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
	_, err := NewProvisioner(tenant, home).Apply(provisionTestBindings(t, nil))
	if err == nil || !strings.Contains(err.Error(), "write .github/workflows/ci.yml") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewProvisionerBindsProductionSeams(t *testing.T) {
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

	materials, err := NewProvisioner(tenant, home).Apply(provisionTestBindings(t, nil))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(materials) != 6 {
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
}
