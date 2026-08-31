package canonical

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// conventionsTemplateFixture is the synthetic template every proof test
// starts from; it carries the full token surface.
const conventionsTemplateFixture = "# Rule Sets\n\nOrganization {{organization}} ships {{repository}} ({{class}}).\n{{platforms}}\nRationale: {{rationale}}\n"

// conventionsReadmePath is the tenant-relative path of the rendered README.
const conventionsReadmePath = "docs/conventions/hosting-platforms/github/rule-sets/README.md"

// conventionsFixture binds the seams for the conventions proof tests.
type conventionsFixture struct {
	homeContents   map[string][]byte
	tenantContents map[string][]byte
	homeErr        error
	tenantErr      error
}

func (fixture conventionsFixture) verifier() Verifier {
	return Verifier{
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
				return nil, errors.New("no such tenant file: " + path)
			}
			return contents, nil
		},
	}
}

func passingConventionsFixture() conventionsFixture {
	return conventionsFixture{
		homeContents: map[string][]byte{
			conventionsTemplatePath: []byte(conventionsTemplateFixture),
		},
		tenantContents: map[string][]byte{
			conventionsReadmePath: []byte("# Rule Sets\n\nOrganization acme ships rocket (linux-only).\nThe quality gates run exclusively on **Linux**.\nRationale: portable schemas only.\n"),
		},
	}
}

func conventionsTestBindings() Bindings {
	return Bindings{
		Class: Class{QualityGates: "linux-only"},
		Conventions: &ConventionsBinding{
			Path:         conventionsReadmePath,
			Organization: "acme",
			Repository:   "rocket",
			Rationale:    "portable schemas only.",
		},
	}
}

func TestVerifyConventionsNotBound(t *testing.T) {
	verifier := Verifier{
		ReadHome: func(path string) ([]byte, error) {
			t.Fatalf("no home read expected, got %q", path)
			return nil, nil
		},
		ReadTenant: func(path string) ([]byte, error) {
			t.Fatalf("no tenant read expected, got %q", path)
			return nil, nil
		},
	}
	if findings := verifier.verifyConventions(Bindings{}); findings != nil {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyConventionsPass(t *testing.T) {
	fixture := passingConventionsFixture()
	if findings := fixture.verifier().verifyConventions(conventionsTestBindings()); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyConventionsFullClass(t *testing.T) {
	fixture := passingConventionsFixture()
	fixture.tenantContents[conventionsReadmePath] = []byte("# Rule Sets\n\nOrganization acme ships rocket (full).\nThe quality gates run on **Linux**, **Windows**, and **macOS**.\nRationale: portable schemas only.\n")
	bindings := conventionsTestBindings()
	bindings.Class.QualityGates = "full"
	if findings := fixture.verifier().verifyConventions(bindings); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyConventionsClassWithoutCanonicalRender(t *testing.T) {
	fixture := passingConventionsFixture()
	bindings := conventionsTestBindings()
	bindings.Class.QualityGates = "pending"
	findings := fixture.verifier().verifyConventions(bindings)
	assertFindingContains(t, findings, "carries no canonical conventions render")
}

func TestVerifyConventionsTemplateReadError(t *testing.T) {
	fixture := passingConventionsFixture()
	fixture.homeErr = errors.New("boom")
	findings := fixture.verifier().verifyConventions(conventionsTestBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "boom") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyConventionsTemplateWithoutToken(t *testing.T) {
	fixture := passingConventionsFixture()
	fixture.homeContents[conventionsTemplatePath] = []byte("# no tokens\n")
	findings := fixture.verifier().verifyConventions(conventionsTestBindings())
	assertFindingContains(t, findings, "carries no {{organization}} token")
}

func TestVerifyConventionsTenantReadError(t *testing.T) {
	fixture := passingConventionsFixture()
	delete(fixture.tenantContents, conventionsReadmePath)
	findings := fixture.verifier().verifyConventions(conventionsTestBindings())
	assertFindingContains(t, findings, "no such tenant file")
}

func TestVerifyConventionsRenderMismatch(t *testing.T) {
	fixture := passingConventionsFixture()
	fixture.tenantContents[conventionsReadmePath] = []byte("drifted\n")
	findings := fixture.verifier().verifyConventions(conventionsTestBindings())
	assertFindingContains(t, findings, "not the materialization")
}

// TestConventionsTemplateRendersCompletely reads the real template from the
// home layout and proves that every token resolves for every class with a
// canonical render: no placeholder rest survives the render.
func TestConventionsTemplateRendersCompletely(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(conventionsTemplatePath)))
	if err != nil {
		t.Fatalf("read the canonical template: %v", err)
	}
	for _, class := range []string{"full", "linux-only"} {
		t.Run(class, func(t *testing.T) {
			rendered, err := renderConventionsReadme(string(contents), ConventionsBinding{
				Path:         conventionsReadmePath,
				Organization: "acme",
				Repository:   "rocket",
				Rationale:    "portable schemas only.",
			}, class)
			if err != nil {
				t.Fatalf("renderConventionsReadme: %v", err)
			}
			if strings.Contains(rendered, "{{") {
				t.Fatalf("the render carries an unresolved token:\n%s", rendered)
			}
			for _, want := range []string{"`acme`", "`rocket`", "quality-gates=" + class, "portable schemas only."} {
				if !strings.Contains(rendered, want) {
					t.Fatalf("the render must carry %q", want)
				}
			}
		})
	}
}
