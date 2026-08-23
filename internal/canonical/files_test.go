package canonical

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func canonicalFileBindings() FileBindings {
	return FileBindings{
		Lefthook:      FileBinding{Path: "lefthook.yml", SHA256: Sum256Hex([]byte("lefthook-core"))},
		Gitattributes: FileBinding{Path: ".gitattributes", SHA256: Sum256Hex([]byte("gitattributes-core"))},
		Gitignore:     FileBinding{Path: ".gitignore", SHA256: Sum256Hex([]byte("gitignore-core\n# -- project additions below this line --\n"))},
		Dependabot:    FileBinding{Path: ".github/dependabot.yml", SHA256: Sum256Hex([]byte("dependabot-core"))},
	}
}

func passingFilesFixture() filesFixture {
	return filesFixture{
		homeContents: map[string][]byte{
			"hosting-platforms/github/files/lefthook/lefthook.yml":        []byte("lefthook-core"),
			"hosting-platforms/github/files/gitattributes/.gitattributes": []byte("gitattributes-core"),
			"hosting-platforms/github/files/gitignore/.gitignore":         []byte("gitignore-core\n# -- project additions below this line --\n"),
			"hosting-platforms/github/files/dependabot/dependabot-go.yml": []byte("dependabot-core"),
			"hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl":   []byte("# contract\n\n* {{defaultOwner}}\n"),
		},
		tenantContents: map[string][]byte{
			"lefthook.yml":           []byte("lefthook-core"),
			".gitattributes":         []byte("gitattributes-core"),
			".gitignore":             []byte("gitignore-core\n# -- project additions below this line --\n\n/dist-custom/\n"),
			".github/dependabot.yml": []byte("dependabot-core"),
			".github/CODEOWNERS":     []byte("# contract\n\n* @CyberT33N\n"),
		},
	}
}

func TestVerifyFilesPass(t *testing.T) {
	fixture := passingFilesFixture()
	if findings := fixture.verifier().verifyFiles(Bindings{Files: canonicalFileBindings()}); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyFilesHomeReadError(t *testing.T) {
	fixture := passingFilesFixture()
	fixture.homeErr = errors.New("boom")
	findings := fixture.verifier().verifyFiles(Bindings{Files: canonicalFileBindings()})
	if len(findings) != len(fileTopics) {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyFilesHomeHashMismatch(t *testing.T) {
	fixture := passingFilesFixture()
	fixture.homeContents["hosting-platforms/github/files/lefthook/lefthook.yml"] = []byte("drifted")
	findings := fixture.verifier().verifyFiles(Bindings{Files: canonicalFileBindings()})
	assertFindingContains(t, findings, "the canonical lefthook hash")
}

func TestVerifyFilesTenantReadError(t *testing.T) {
	fixture := passingFilesFixture()
	delete(fixture.tenantContents, "lefthook.yml")
	findings := fixture.verifier().verifyFiles(Bindings{Files: canonicalFileBindings()})
	assertFindingContains(t, findings, "no such tenant file")
}

func TestVerifyFilesTenantHashMismatch(t *testing.T) {
	fixture := passingFilesFixture()
	fixture.tenantContents[".gitattributes"] = []byte("drifted")
	findings := fixture.verifier().verifyFiles(Bindings{Files: canonicalFileBindings()})
	assertFindingContains(t, findings, "the tenant .gitattributes hash")
}

func TestVerifyFilesGitignorePrefix(t *testing.T) {
	bindings := Bindings{Files: canonicalFileBindings()}

	t.Run("exact core without project block", func(t *testing.T) {
		fixture := passingFilesFixture()
		fixture.tenantContents[".gitignore"] = []byte("gitignore-core\n# -- project additions below this line --\n")
		if findings := fixture.verifier().verifyFiles(bindings); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("modified core breaks the prefix", func(t *testing.T) {
		fixture := passingFilesFixture()
		fixture.tenantContents[".gitignore"] = []byte("# drifted core\n# -- project additions below this line --\n")
		findings := fixture.verifier().verifyFiles(bindings)
		assertFindingContains(t, findings, "verbatim prefix")
	})
}

func TestVerifyCodeowners(t *testing.T) {
	bindings := Bindings{Codeowners: CodeownersBinding{Path: ".github/CODEOWNERS", DefaultOwner: "@CyberT33N"}}

	t.Run("pass", func(t *testing.T) {
		fixture := passingFilesFixture()
		if findings := fixture.verifier().verifyCodeowners(bindings); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("template read error", func(t *testing.T) {
		fixture := passingFilesFixture()
		fixture.homeErr = errors.New("boom")
		findings := fixture.verifier().verifyCodeowners(bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "boom") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("template without token", func(t *testing.T) {
		fixture := passingFilesFixture()
		fixture.homeContents["hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl"] = []byte("* @someone\n")
		findings := fixture.verifier().verifyCodeowners(bindings)
		assertFindingContains(t, findings, "carries no")
	})

	t.Run("tenant read error", func(t *testing.T) {
		fixture := passingFilesFixture()
		delete(fixture.tenantContents, ".github/CODEOWNERS")
		findings := fixture.verifier().verifyCodeowners(bindings)
		assertFindingContains(t, findings, "no such tenant file")
	})

	t.Run("render mismatch", func(t *testing.T) {
		fixture := passingFilesFixture()
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
	for _, path := range []string{codeownersTemplatePath, callerHashesPath} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			t.Fatalf("the verifier home path %q does not exist in the home layout: %v", path, err)
		}
	}
}
