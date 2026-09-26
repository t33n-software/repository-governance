package canonical

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gitignoreRenderedFixture renders the fixture tree's governed region at the
// canonical test home pin.

// filesFixture binds the seams for the file-family proof tests.
type filesFixture struct {
	homeContents   map[string][]byte
	tenantContents map[string][]byte
	tenantErr      error
	homeErr        error
}

func (fixture filesFixture) verifier() Verifier {
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

// gitignoreRenderedFixture renders the fixture tree's governed region.
func gitignoreRenderedFixture(t *testing.T) []byte {
	t.Helper()
	rendered, err := RenderGitignoreGovernedRegion(readGitignoreFixture(gitignoreFixtureHome()), []string{"core", "go/core"}, testHomeSHA)
	if err != nil {
		t.Fatalf("RenderGitignoreGovernedRegion: %v", err)
	}
	return rendered
}

func canonicalFileBindings(t *testing.T) FileBindings {
	t.Helper()
	return FileBindings{
		Lefthook:      FileBinding{Path: "lefthook.yml", SHA256: Sum256Hex([]byte("lefthook-core"))},
		Gitattributes: FileBinding{Path: ".gitattributes", SHA256: Sum256Hex([]byte("gitattributes-core"))},
		Gitignore:     GitignoreBinding{Path: ".gitignore", Fragments: []string{"core", "go/core"}, SHA256: Sum256Hex(gitignoreRenderedFixture(t))},
		Dependabot:    FileBinding{Path: ".github/dependabot.yml", SHA256: Sum256Hex([]byte("dependabot-core"))},
	}
}

// fileTestBindings carries the file bindings with the home pin the fixtures
// render against.
func fileTestBindings(t *testing.T) Bindings {
	t.Helper()
	return Bindings{
		Home:  HomePin{SHA: testHomeSHA},
		Files: canonicalFileBindings(t),
	}
}

func passingFilesFixture(t *testing.T) filesFixture {
	t.Helper()
	homeContents := map[string][]byte{
		"hosting-platforms/github/files/lefthook/lefthook.yml":        []byte("lefthook-core"),
		"hosting-platforms/github/files/gitattributes/.gitattributes": []byte("gitattributes-core"),
		"hosting-platforms/github/files/dependabot/dependabot-go.yml": []byte("dependabot-core"),
		"hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl":   []byte("# contract\n\n* {{defaultOwner}}\n"),
	}
	for path, contents := range gitignoreFixtureHome() {
		homeContents[path] = contents
	}
	return filesFixture{
		homeContents: homeContents,
		tenantContents: map[string][]byte{
			"lefthook.yml":           []byte("lefthook-core"),
			".gitattributes":         []byte("gitattributes-core"),
			".gitignore":             append(gitignoreRenderedFixture(t), []byte("\n/dist-custom/\n")...),
			".github/dependabot.yml": []byte("dependabot-core"),
			".github/CODEOWNERS":     []byte("# contract\n\n* @CyberT33N\n"),
		},
	}
}

func TestVerifyFilesPass(t *testing.T) {
	fixture := passingFilesFixture(t)
	if findings := fixture.verifier().verifyFiles(fileTestBindings(t)); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyFilesHomeReadError(t *testing.T) {
	fixture := passingFilesFixture(t)
	fixture.homeErr = errors.New("boom")
	findings := fixture.verifier().verifyFiles(fileTestBindings(t))
	if len(findings) != len(fileTopics)+1 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyFilesHomeHashMismatch(t *testing.T) {
	fixture := passingFilesFixture(t)
	fixture.homeContents["hosting-platforms/github/files/lefthook/lefthook.yml"] = []byte("drifted")
	findings := fixture.verifier().verifyFiles(fileTestBindings(t))
	assertFindingContains(t, findings, "the canonical lefthook hash")
}

func TestVerifyFilesTenantReadError(t *testing.T) {
	fixture := passingFilesFixture(t)
	delete(fixture.tenantContents, "lefthook.yml")
	findings := fixture.verifier().verifyFiles(fileTestBindings(t))
	assertFindingContains(t, findings, "no such tenant file")
}

func TestVerifyFilesTenantHashMismatch(t *testing.T) {
	fixture := passingFilesFixture(t)
	fixture.tenantContents[".gitattributes"] = []byte("drifted")
	findings := fixture.verifier().verifyFiles(fileTestBindings(t))
	assertFindingContains(t, findings, "the tenant .gitattributes hash")
}

func TestVerifyFilesGitignore(t *testing.T) {
	t.Run("pass on the exact governed region without a project block", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		fixture.tenantContents[".gitignore"] = gitignoreRenderedFixture(t)
		if findings := fixture.verifier().verifyFiles(fileTestBindings(t)); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("render failure", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		bindings := fileTestBindings(t)
		bindings.Files.Gitignore.Fragments = []string{"core", "unknown/area"}
		findings := fixture.verifier().verifyFiles(bindings)
		assertFindingContains(t, findings, "the bound fragments do not render")
	})

	t.Run("hash mismatch", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		bindings := fileTestBindings(t)
		bindings.Files.Gitignore.SHA256 = strings.Repeat("0", 64)
		findings := fixture.verifier().verifyFiles(bindings)
		assertFindingContains(t, findings, "the rendered governed region hash")
	})

	t.Run("tenant read error", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		delete(fixture.tenantContents, ".gitignore")
		findings := fixture.verifier().verifyFiles(fileTestBindings(t))
		assertFindingContains(t, findings, "no such tenant file")
	})

	t.Run("prefix mismatch", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		fixture.tenantContents[".gitignore"] = []byte("# drifted region\n# -- project additions below this line --\n")
		findings := fixture.verifier().verifyFiles(fileTestBindings(t))
		assertFindingContains(t, findings, "verbatim prefix")
	})

	t.Run("license guard skips when the class is not bound", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		fixture.tenantContents[".gitignore"] = append(gitignoreRenderedFixture(t), []byte("\nLICENSE\n")...)
		if findings := fixture.verifier().verifyFiles(fileTestBindings(t)); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("license guard fails closed when bound", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		fixture.tenantContents[".gitignore"] = append(gitignoreRenderedFixture(t), []byte("\nLICENSE\n")...)
		bindings := fileTestBindings(t)
		bindings.Class.LicenseHub = true
		findings := fixture.verifier().verifyFiles(bindings)
		assertFindingContains(t, findings, "ignores the protected license family: LICENSE")
	})

	t.Run("license guard passes when bound and clean", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		bindings := fileTestBindings(t)
		bindings.Class.LicenseHub = true
		if findings := fixture.verifier().verifyFiles(bindings); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})
}

func TestVerifyCodeowners(t *testing.T) {
	bindings := Bindings{Codeowners: CodeownersBinding{Path: ".github/CODEOWNERS", DefaultOwner: "@CyberT33N"}}

	t.Run("pass", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		if findings := fixture.verifier().verifyCodeowners(bindings); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("template read error", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		fixture.homeErr = errors.New("boom")
		findings := fixture.verifier().verifyCodeowners(bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "boom") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("template without token", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		fixture.homeContents["hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl"] = []byte("* @someone\n")
		findings := fixture.verifier().verifyCodeowners(bindings)
		assertFindingContains(t, findings, "carries no")
	})

	t.Run("tenant read error", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		delete(fixture.tenantContents, ".github/CODEOWNERS")
		findings := fixture.verifier().verifyCodeowners(bindings)
		assertFindingContains(t, findings, "no such tenant file")
	})

	t.Run("render mismatch", func(t *testing.T) {
		fixture := passingFilesFixture(t)
		fixture.tenantContents[".github/CODEOWNERS"] = []byte("* @SomeoneElse\n")
		findings := fixture.verifier().verifyCodeowners(bindings)
		assertFindingContains(t, findings, "not the materialization")
	})
}

// TestVerifierHomePathsExistInTheHomeLayout binds the verifier's home-relative
// paths to the real home layout: a path that drifts from the repository tree
// fails closed instead of silently reading nothing at verification time.
func TestVerifierHomePathsExistInTheHomeLayout(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, topic := range fileTopics {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(topic.homePath))); err != nil {
			t.Fatalf("the verifier home path %q does not exist in the home layout: %v", topic.homePath, err)
		}
	}
	fragmentPaths := []string{
		"core.gitignore",
		"go/core.gitignore",
		"opentofu/core.gitignore",
		"opentofu/lockfiles-committed.gitignore",
	}
	for _, fragment := range fragmentPaths {
		path := gitignoreTreeRoot + "/" + fragment
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("the gitignore fragment %q does not exist in the home layout: %v", path, err)
		}
	}
	for _, path := range []string{codeownersTemplatePath, conventionsTemplatePath, callerHashesPath} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("the verifier home path %q does not exist in the home layout: %v", path, err)
		}
	}
}
