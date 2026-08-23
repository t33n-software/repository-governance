package canonical

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func catalogJSON() string {
	return catalogJSONWithSchema("https://raw.githubusercontent.com/t33n-software/go-quality-authority/main/catalog/tools.schema.json")
}

func catalogJSONWithSchema(schema string) string {
	return `{
  "$schema": "` + schema + `",
  "schemaVersion": 1,
  "tools": [
    { "name": "staticcheck", "module": "honnef.co/go/tools", "package": "honnef.co/go/tools/cmd/staticcheck", "purpose": "static analysis" },
    { "name": "quality-gate", "module": "github.com/t33n-software/go-quality-authority", "package": "github.com/t33n-software/go-quality-authority/cmd/quality-gate", "purpose": "quality orchestration" }
  ]
}`
}

func TestDecodeToolCatalog(t *testing.T) {
	catalog, err := DecodeToolCatalog([]byte(catalogJSON()))
	if err != nil {
		t.Fatalf("DecodeToolCatalog: %v", err)
	}
	if catalog.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d", catalog.SchemaVersion)
	}
	if catalog.Schema != "https://raw.githubusercontent.com/t33n-software/go-quality-authority/main/catalog/tools.schema.json" {
		t.Fatalf("Schema = %q", catalog.Schema)
	}
	if len(catalog.Packages) != 2 {
		t.Fatalf("Packages = %v", catalog.Packages)
	}
}

func TestDecodeToolCatalogRejections(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{name: "empty", doc: ``},
		{name: "not json", doc: `not json`},
		{name: "trailing document", doc: catalogJSON() + ` {}`},
		{name: "wrong schema version", doc: `{"$schema":"https://raw.githubusercontent.com/t33n-software/go-quality-authority/main/catalog/tools.schema.json","schemaVersion":2,"tools":[]}`},
		{name: "unknown field", doc: `{"$schema":"https://raw.githubusercontent.com/t33n-software/go-quality-authority/main/catalog/tools.schema.json","schemaVersion":1,"tools":[],"bogus":true}`},
		{name: "missing schema identity", doc: `{"schemaVersion":1,"tools":[]}`},
		{name: "empty schema identity", doc: `{"$schema":"  ","schemaVersion":1,"tools":[]}`},
		{name: "incomplete entry", doc: `{"$schema":"https://raw.githubusercontent.com/t33n-software/go-quality-authority/main/catalog/tools.schema.json","schemaVersion":1,"tools":[{"name":"x","module":"","package":"p","purpose":"q"}]}`},
		{name: "duplicate package", doc: `{"$schema":"https://raw.githubusercontent.com/t33n-software/go-quality-authority/main/catalog/tools.schema.json","schemaVersion":1,"tools":[{"name":"a","module":"m","package":"p","purpose":"q"},{"name":"b","module":"m","package":"p","purpose":"q"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeToolCatalog([]byte(test.doc)); err == nil {
				t.Fatal("expected a rejection")
			}
		})
	}
}

func TestToolDirectives(t *testing.T) {
	module := `module example.com/tenant/tools

go 1.26.6

tool (
	honnef.co/go/tools/cmd/staticcheck
	golang.org/x/vuln/cmd/govulncheck // vulnerability analysis

	github.com/evilmartians/lefthook/v2
)

tool github.com/t33n-software/go-quality-authority/cmd/quality-gate

require honnef.co/go/tools v0.7.0 // indirect
`
	tools, err := ToolDirectives([]byte(module))
	if err != nil {
		t.Fatalf("ToolDirectives: %v", err)
	}
	want := []string{
		"honnef.co/go/tools/cmd/staticcheck",
		"golang.org/x/vuln/cmd/govulncheck",
		"github.com/evilmartians/lefthook/v2",
		"github.com/t33n-software/go-quality-authority/cmd/quality-gate",
	}
	if strings.Join(tools, ",") != strings.Join(want, ",") {
		t.Fatalf("tools = %v", tools)
	}
}

func TestToolDirectivesEmptyAndUnterminated(t *testing.T) {
	tools, err := ToolDirectives([]byte("module example.com/m\n"))
	if err != nil {
		t.Fatalf("ToolDirectives: %v", err)
	}
	if len(tools) != 0 {
		t.Fatalf("tools = %v", tools)
	}
	if _, err := ToolDirectives([]byte("tool (\n\texample.com/x\n")); err == nil {
		t.Fatal("expected the unterminated block rejection")
	}
}

func TestUnadmittedTools(t *testing.T) {
	catalog, err := DecodeToolCatalog([]byte(catalogJSON()))
	if err != nil {
		t.Fatalf("DecodeToolCatalog: %v", err)
	}
	unadmitted := UnadmittedTools([]string{
		"honnef.co/go/tools/cmd/staticcheck",
		"example.com/evil/tool",
		"example.com/evil/tool",
		"github.com/t33n-software/repository-governance/cmd/verify-canonical",
	}, catalog, "github.com/t33n-software/repository-governance/cmd/verify-canonical")
	if len(unadmitted) != 1 || unadmitted[0] != "example.com/evil/tool" {
		t.Fatalf("unadmitted = %v", unadmitted)
	}
}

func TestUnadmittedToolsWithoutHomeTool(t *testing.T) {
	catalog, err := DecodeToolCatalog([]byte(catalogJSON()))
	if err != nil {
		t.Fatalf("DecodeToolCatalog: %v", err)
	}
	unadmitted := UnadmittedTools([]string{"github.com/t33n-software/repository-governance/cmd/verify-canonical"}, catalog, "")
	if len(unadmitted) != 1 {
		t.Fatalf("unadmitted = %v", unadmitted)
	}
}

// toolsFixture binds a tenant tools module and the resolution seams for the
// verifyTools tests.
type toolsFixture struct {
	moduleContents  []byte
	moduleErr       error
	resolveDir      string
	resolveErr      error
	catalogContents []byte
	catalogErr      error
	homeGoMod       []byte
	homeErr         error
}

func (fixture toolsFixture) verifier() Verifier {
	return Verifier{
		TenantRoot: "tenant",
		ReadTenant: func(path string) ([]byte, error) {
			return fixture.moduleContents, fixture.moduleErr
		},
		ReadHome: func(path string) ([]byte, error) {
			return fixture.homeGoMod, fixture.homeErr
		},
		ReadModule: func(dir, path string) ([]byte, error) {
			return fixture.catalogContents, fixture.catalogErr
		},
		ResolveModule: func(ctx context.Context, dir, module string) (string, error) {
			return fixture.resolveDir, fixture.resolveErr
		},
	}
}

func verifyToolsBindings() Bindings {
	return Bindings{Tools: ToolsBinding{Module: "tools/go.mod", CatalogVersion: 1}}
}

func TestVerifyToolsWithoutToolsModule(t *testing.T) {
	fixture := toolsFixture{moduleErr: fs.ErrNotExist}
	if findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings()); findings != nil {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsReadError(t *testing.T) {
	fixture := toolsFixture{moduleErr: errors.New("boom")}
	findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "boom") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsParseError(t *testing.T) {
	fixture := toolsFixture{moduleContents: []byte("tool (\n")}
	findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "unterminated") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsWithoutDirectives(t *testing.T) {
	fixture := toolsFixture{moduleContents: []byte("module example.com/m\n")}
	if findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings()); findings != nil {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsResolveError(t *testing.T) {
	fixture := toolsFixture{
		moduleContents: []byte("tool example.com/x\n"),
		resolveErr:     errors.New("no module"),
	}
	findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "resolve the canonical catalog module") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsCatalogReadError(t *testing.T) {
	fixture := toolsFixture{
		moduleContents: []byte("tool example.com/x\n"),
		resolveDir:     "gqa",
		catalogErr:     errors.New("boom"),
	}
	findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "catalog/tools.json") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsCatalogDecodeError(t *testing.T) {
	fixture := toolsFixture{
		moduleContents:  []byte("tool example.com/x\n"),
		resolveDir:      "gqa",
		catalogContents: []byte("not json"),
	}
	findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings())
	if len(findings) != 1 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsHomeModuleError(t *testing.T) {
	fixture := toolsFixture{
		moduleContents:  []byte("tool honnef.co/go/tools/cmd/staticcheck\n"),
		resolveDir:      "gqa",
		catalogContents: []byte(catalogJSON()),
		homeErr:         errors.New("boom"),
	}
	findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "home module") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsSchemaIdentityMismatch(t *testing.T) {
	fixture := toolsFixture{
		moduleContents:  []byte("tool honnef.co/go/tools/cmd/staticcheck\n"),
		resolveDir:      "gqa",
		catalogContents: []byte(catalogJSONWithSchema("https://example.com/drifted/tools.schema.json")),
		homeGoMod:       []byte("module github.com/t33n-software/repository-governance\n"),
	}
	findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "diverges from the canonical") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsUnadmitted(t *testing.T) {
	fixture := toolsFixture{
		moduleContents:  []byte("tool (\n\thonnef.co/go/tools/cmd/staticcheck\n\texample.com/evil/tool\n)\n"),
		resolveDir:      "gqa",
		catalogContents: []byte(catalogJSON()),
		homeGoMod:       []byte("module github.com/t33n-software/repository-governance\n"),
	}
	findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "example.com/evil/tool") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolsPass(t *testing.T) {
	fixture := toolsFixture{
		moduleContents: []byte("tool (\n\thonnef.co/go/tools/cmd/staticcheck\n\tgithub.com/t33n-software/repository-governance/cmd/verify-canonical\n)\n"),
		resolveDir:      "gqa",
		catalogContents: []byte(catalogJSON()),
		homeGoMod:       []byte("module github.com/t33n-software/repository-governance\n"),
	}
	if findings := fixture.verifier().verifyTools(context.Background(), verifyToolsBindings()); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestHomeToolPackage(t *testing.T) {
	t.Run("pass", func(t *testing.T) {
		verifier := Verifier{
			ReadHome: func(path string) ([]byte, error) {
				return []byte("module github.com/t33n-software/repository-governance\n\ngo 1.26\n"), nil
			},
		}
		pkg, err := verifier.homeToolPackage()
		if err != nil {
			t.Fatalf("homeToolPackage: %v", err)
		}
		if pkg != "github.com/t33n-software/repository-governance/cmd/verify-canonical" {
			t.Fatalf("pkg = %q", pkg)
		}
	})

	t.Run("read error", func(t *testing.T) {
		verifier := Verifier{
			ReadHome: func(path string) ([]byte, error) { return nil, errors.New("boom") },
		}
		if _, err := verifier.homeToolPackage(); err == nil {
			t.Fatal("expected the read error")
		}
	})

	t.Run("no module line", func(t *testing.T) {
		verifier := Verifier{
			ReadHome: func(path string) ([]byte, error) { return []byte("go 1.26\n"), nil },
		}
		if _, err := verifier.homeToolPackage(); err == nil {
			t.Fatal("expected the missing module line rejection")
		}
	})
}
