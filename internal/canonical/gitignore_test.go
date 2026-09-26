package canonical

import (
	"errors"
	"strings"
	"testing"
)

// gitignoreFixtureHome carries a minimal valid fragment tree: the org core
// with every canonical secret family and one Go area fragment.
func gitignoreFixtureHome() map[string][]byte {
	return map[string][]byte{
		"hosting-platforms/github/files/gitignore/core.gitignore":    []byte("# Local build and test outputs.\n/.build/\n\n# Never-commit secret-artifact guard.\n.env\n.env.*\n.envrc\n.env*.local\ncredentials\ncredentials.*\n*.pem\n*.key\n*.p12\n*.pfx\n*.jks\n*.keystore\n*.kdbx\n*.ppk\n*.gpg\n"),
		"hosting-platforms/github/files/gitignore/go/core.gitignore": []byte("# Go toolchain artifacts.\n*.coverprofile\n*.test\n*.cov\n"),
	}
}

// readGitignoreFixture reads the fixture tree fail-closed on unknown paths.
func readGitignoreFixture(home map[string][]byte) func(path string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		contents, found := home[path]
		if !found {
			return nil, errors.New("no such home file: " + path)
		}
		return contents, nil
	}
}

func TestValidateGitignoreFragments(t *testing.T) {
	t.Run("valid forms", func(t *testing.T) {
		for _, fragments := range [][]string{
			{"core"},
			{"core", "go/core"},
			{"core", "opentofu/core", "opentofu/lockfiles-committed"},
		} {
			if err := ValidateGitignoreFragments(fragments); err != nil {
				t.Fatalf("ValidateGitignoreFragments(%v): %v", fragments, err)
			}
		}
	})

	tests := []struct {
		name      string
		fragments []string
		message   string
	}{
		{name: "empty", fragments: nil, message: "must contain at least the core fragment"},
		{name: "core not first", fragments: []string{"go/core", "core"}, message: "must begin with the core fragment"},
		{name: "invalid name", fragments: []string{"core", "Go/Core"}, message: "must be a canonical fragment name"},
		{name: "parent traversal", fragments: []string{"core", "../evil"}, message: "must be a canonical fragment name"},
		{name: "bare area without a fragment", fragments: []string{"core", "golang"}, message: "must name an area or concern below the org core"},
		{name: "duplicate", fragments: []string{"core", "go/core", "go/core"}, message: "must not repeat a fragment"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateGitignoreFragments(test.fragments)
			if err == nil {
				t.Fatalf("expected a rejection containing %q", test.message)
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error %q does not contain %q", err.Error(), test.message)
			}
		})
	}
}

func TestRenderGitignoreGovernedRegion(t *testing.T) {
	const pin = "0123456789abcdef0123456789abcdef01234567"

	t.Run("byte exact composition", func(t *testing.T) {
		rendered, err := RenderGitignoreGovernedRegion(readGitignoreFixture(gitignoreFixtureHome()), []string{"core", "go/core"}, pin)
		if err != nil {
			t.Fatalf("RenderGitignoreGovernedRegion: %v", err)
		}
		want := "# canonical: gitignore core + go/core @ " + pin + " — governed region, do not edit\n" +
			"# Local build and test outputs.\n/.build/\n\n# Never-commit secret-artifact guard.\n.env\n.env.*\n.envrc\n.env*.local\ncredentials\ncredentials.*\n*.pem\n*.key\n*.p12\n*.pfx\n*.jks\n*.keystore\n*.kdbx\n*.ppk\n*.gpg" +
			"\n\n# go/core\n# Go toolchain artifacts.\n*.coverprofile\n*.test\n*.cov" +
			"\n\n# -- project additions below this line --\n"
		if string(rendered) != want {
			t.Fatalf("rendered = %q", string(rendered))
		}
	})

	t.Run("invalid fragment list", func(t *testing.T) {
		_, err := RenderGitignoreGovernedRegion(readGitignoreFixture(gitignoreFixtureHome()), []string{"go/core"}, pin)
		if err == nil || !strings.Contains(err.Error(), "must begin with the core fragment") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("unreadable fragment", func(t *testing.T) {
		_, err := RenderGitignoreGovernedRegion(readGitignoreFixture(gitignoreFixtureHome()), []string{"core", "unknown/area"}, pin)
		if err == nil || !strings.Contains(err.Error(), "read the fragment unknown/area") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("fragment carrying the mark", func(t *testing.T) {
		home := gitignoreFixtureHome()
		home["hosting-platforms/github/files/gitignore/go/core.gitignore"] = []byte("*.test\n# -- project additions below this line --\n")
		_, err := RenderGitignoreGovernedRegion(readGitignoreFixture(home), []string{"core", "go/core"}, pin)
		if err == nil || !strings.Contains(err.Error(), "carries the project-block mark") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("cross fragment restatement", func(t *testing.T) {
		home := gitignoreFixtureHome()
		home["hosting-platforms/github/files/gitignore/go/core.gitignore"] = []byte("/.build/\n*.test\n")
		_, err := RenderGitignoreGovernedRegion(readGitignoreFixture(home), []string{"core", "go/core"}, pin)
		if err == nil || !strings.Contains(err.Error(), "restates the pattern \"/.build/\"") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("negation in the org core", func(t *testing.T) {
		home := gitignoreFixtureHome()
		home["hosting-platforms/github/files/gitignore/core.gitignore"] = append(home["hosting-platforms/github/files/gitignore/core.gitignore"], []byte("!/.build/\n")...)
		_, err := RenderGitignoreGovernedRegion(readGitignoreFixture(home), []string{"core"}, pin)
		if err == nil || !strings.Contains(err.Error(), "negation \"!/.build/\"") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("missing secret family", func(t *testing.T) {
		home := gitignoreFixtureHome()
		home["hosting-platforms/github/files/gitignore/core.gitignore"] = []byte("# Local build and test outputs.\n/.build/\n")
		_, err := RenderGitignoreGovernedRegion(readGitignoreFixture(home), []string{"core"}, pin)
		if err == nil || !strings.Contains(err.Error(), "misses the secret-artifact family \".env\"") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("license family match in the governed region", func(t *testing.T) {
		home := gitignoreFixtureHome()
		home["hosting-platforms/github/files/gitignore/go/core.gitignore"] = []byte("LICENSE\n*.test\n")
		_, err := RenderGitignoreGovernedRegion(readGitignoreFixture(home), []string{"core", "go/core"}, pin)
		if err == nil || !strings.Contains(err.Error(), "ignores the protected license family: LICENSE") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestGitignoreLicenseViolations(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{name: "clean", content: "/dist/\n/.cache/\n", want: nil},
		{name: "comments and blanks are skipped", content: "# LICENSE\n\n/dist/\n", want: nil},
		{name: "unrelated extension pattern", content: "*.txt\n", want: nil},
		{name: "license file", content: "LICENSE\n", want: []string{"LICENSE"}},
		{name: "license prefix pattern", content: "LICENSE*\n", want: []string{"LICENSE", "LICENSES/", "LICENSES/<content>"}},
		{name: "notice file", content: "NOTICE\n", want: []string{"NOTICE"}},
		{name: "licenses directory", content: "LICENSES/\n", want: []string{"LICENSES/", "LICENSES/<content>"}},
		{name: "licenses content", content: "LICENSES/*\n", want: []string{"LICENSES/<content>"}},
		{name: "unanchored directory name", content: "LICENSES\n", want: []string{"LICENSES/", "LICENSES/<content>"}},
		{name: "negation re-includes in order", content: "LICENSE\n!LICENSE\n", want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			violations := GitignoreLicenseViolations([]byte(test.content))
			if strings.Join(violations, ",") != strings.Join(test.want, ",") {
				t.Fatalf("violations = %v, want %v", violations, test.want)
			}
		})
	}
}

func TestGitignorePatternLines(t *testing.T) {
	patterns := gitignorePatternLines("# comment\n\n/dist/\n*.out \n")
	if len(patterns) != 2 || patterns[0] != "/dist/" || patterns[1] != "*.out " {
		t.Fatalf("patterns = %q", patterns)
	}
	if patterns := gitignorePatternLines(""); len(patterns) != 0 {
		t.Fatalf("patterns = %q", patterns)
	}
}
