package packaging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/t33n-software/repository-governance/internal/canonical"
)

// gitignoreGoldenSets binds every registered fragment set to its golden
// render under conformance/gitignore/.
var gitignoreGoldenSets = map[string][]string{
	"core":                              {"core"},
	"core-go":                           {"core", "go/core"},
	"core-node":                         {"core", "node/core"},
	"core-opentofu":                     {"core", "opentofu/core"},
	"core-opentofu-lockfiles-committed": {"core", "opentofu/core", "opentofu/lockfiles-committed"},
	"core-python":                       {"core", "python/core"},
}

// gitignoreGoldenPin is the fixed home pin the golden renders carry.
const gitignoreGoldenPin = "0123456789abcdef0123456789abcdef01234567"

// readHomeArtifact binds the render's home-read seam to the real home tree.
func readHomeArtifact(t *testing.T) func(string) ([]byte, error) {
	t.Helper()
	return func(path string) ([]byte, error) {
		return os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(path)))
	}
}

// TestGitignoreFragmentTree proves the fragment tree of the canonical file
// family: the registered fragments exist, the superseded single-core master
// is gone, no fragment carries the project-block mark, and the org core
// carries the canonical secret-artifact families.
func TestGitignoreFragmentTree(t *testing.T) {
	fragments := []string{"core.gitignore", "go/core.gitignore", "node/core.gitignore", "opentofu/core.gitignore", "opentofu/lockfiles-committed.gitignore", "python/core.gitignore"}
	for _, fragment := range fragments {
		content := readArtifact(t, "hosting-platforms/github/files/gitignore/"+fragment)
		if strings.Contains(content, "# -- project additions below this line --") {
			t.Fatalf("the fragment %s must not carry the project-block mark", fragment)
		}
	}
	if _, err := os.Stat(filepath.Join(repoRoot(t), "hosting-platforms", "github", "files", "gitignore", ".gitignore")); !os.IsNotExist(err) {
		t.Fatal("the superseded single-core gitignore master must not exist")
	}

	core := readArtifact(t, "hosting-platforms/github/files/gitignore/core.gitignore")
	coreLines := make(map[string]struct{})
	for _, line := range strings.Split(core, "\n") {
		coreLines[strings.TrimSpace(line)] = struct{}{}
	}
	for _, family := range []string{"/.build/", "/dist/", "/coverage/", "/.cache/", "*.out", ".env", ".env.*", ".envrc", ".env*.local", "credentials", "credentials.*", "*.pem", "*.key", "*.p12", "*.pfx", "*.jks", "*.keystore", "*.kdbx", "*.ppk", "*.gpg"} {
		if _, found := coreLines[family]; !found {
			t.Fatalf("the org core must carry the family %q", family)
		}
	}
	for _, moved := range []string{"*.coverprofile", "*.test", "*.cov"} {
		if _, found := coreLines[moved]; found {
			t.Fatalf("the Go toolchain artifact %q belongs to the go area, not the org core", moved)
		}
	}
}

// TestGitignoreGoldenRenders proves the golden renders of every registered
// fragment set against the real home tree: changing any layer changes every
// golden render of every bound set, so an invalid composition can never leave
// the home.
func TestGitignoreGoldenRenders(t *testing.T) {
	for name, fragments := range gitignoreGoldenSets {
		t.Run(name, func(t *testing.T) {
			rendered, err := canonical.RenderGitignoreGovernedRegion(readHomeArtifact(t), fragments, gitignoreGoldenPin)
			if err != nil {
				t.Fatalf("RenderGitignoreGovernedRegion: %v", err)
			}
			if golden := readArtifact(t, "conformance/gitignore/"+name+".golden.gitignore"); string(rendered) != golden {
				t.Fatalf("the render of %v diverges from the golden", fragments)
			}
		})
	}
}

// TestGitignoreCompositionInvariants proves the composition invariants on the
// real tree: no pattern line repeats across fragments, and the committed
// lockfile policy fragment carries no pattern.
func TestGitignoreCompositionInvariants(t *testing.T) {
	seen := make(map[string]string)
	for _, fragment := range []string{"core.gitignore", "go/core.gitignore", "node/core.gitignore", "opentofu/core.gitignore", "opentofu/lockfiles-committed.gitignore", "python/core.gitignore"} {
		for _, line := range strings.Split(readArtifact(t, "hosting-platforms/github/files/gitignore/"+fragment), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if owner, found := seen[trimmed]; found {
				t.Fatalf("the pattern %q is restated: %s and %s", trimmed, owner, fragment)
			}
			seen[trimmed] = fragment
		}
	}

	lockfiles := readArtifact(t, "hosting-platforms/github/files/gitignore/opentofu/lockfiles-committed.gitignore")
	for _, line := range strings.Split(lockfiles, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		t.Fatalf("the committed lockfile policy fragment must not carry a pattern: %q", trimmed)
	}
}

// TestHomeGitignoreIsTheRenderedComposition proves the home's own tenant
// file is the rendered composition of its bound fragment list at its bound
// home pin, and that the bound hash matches the rendered governed region.
func TestHomeGitignoreIsTheRenderedComposition(t *testing.T) {
	manifest := readArtifact(t, "repo-bindings.json")
	bindings, err := canonical.DecodeBindings([]byte(manifest))
	if err != nil {
		t.Fatalf("the home's own binding manifest must decode: %v", err)
	}
	rendered, err := canonical.RenderGitignoreGovernedRegion(readHomeArtifact(t), bindings.Files.Gitignore.Fragments, bindings.Home.SHA)
	if err != nil {
		t.Fatalf("the home's bound fragments must render: %v", err)
	}
	if hash := canonical.Sum256Hex(rendered); hash != bindings.Files.Gitignore.SHA256 {
		t.Fatalf("the home's bound gitignore hash diverges from the rendered region: %s != %s", hash, bindings.Files.Gitignore.SHA256)
	}
	if own := readArtifact(t, ".gitignore"); own != string(rendered) {
		t.Fatal("the home's own .gitignore is not the rendered composition of its bound fragments")
	}
}
