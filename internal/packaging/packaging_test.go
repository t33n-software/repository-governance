// Package packaging binds the canonical artifacts of the repository-governance
// home — the workflow payloads, the composite actions, the canonical callers,
// the file family, the schemas, and the conformance vectors — to the contract
// through the one canonical contract-test set. The drift watcher itself never
// drifts: this is the only contract-test set of the home.
package packaging

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot resolves the repository root from the packaging test package.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

// readArtifact reads a canonical artifact relative to the repository root.
func readArtifact(t *testing.T, relative string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return string(contents)
}

// listVectors returns the sorted vector file names in a conformance lane.
func listVectors(t *testing.T, lane string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), "conformance", lane))
	if err != nil {
		t.Fatalf("list %s vectors: %v", lane, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		t.Fatalf("the %s lane carries no vectors", lane)
	}
	return names
}

var payloads = []string{
	".github/workflows/reusable-ci-go.yml",
	".github/workflows/reusable-codeql-go.yml",
	".github/workflows/reusable-dependency-review.yml",
	".github/workflows/reusable-release-config.yml",
	".github/workflows/reusable-canonical-conformance.yml",
}

var callers = []string{
	"hosting-platforms/github/workflows/callers/go/ci.yml",
	"hosting-platforms/github/workflows/callers/go/ci-full.yml",
	"hosting-platforms/github/workflows/callers/go/codeql.yml",
	"hosting-platforms/github/workflows/callers/go/dependency-review.yml",
	"hosting-platforms/github/workflows/callers/go/release-config.yml",
	"hosting-platforms/github/workflows/callers/go/canonical-conformance.yml",
}

// compositeActions lists the home's composite actions.
var compositeActions = []string{
	".github/actions/setup-controlled-go/action.yml",
	".github/actions/verify-canonical-files/action.yml",
}

// homeWorkflows lists the home's own workflow files beyond the payloads: the
// three dogfooding callers and the seven lifecycle callers.
var homeWorkflows = []string{
	".github/workflows/ci.yml",
	".github/workflows/codeql.yml",
	".github/workflows/dependency-review.yml",
	".github/workflows/execute-protected-line-request.yml",
	".github/workflows/hotfix-delivery.yml",
	".github/workflows/hotfix-propagation.yml",
	".github/workflows/publish-release-artifacts.yml",
	".github/workflows/release-control.yml",
	".github/workflows/release-reconciliation.yml",
	".github/workflows/tag-promoted-release.yml",
}

var actionSHA = regexp.MustCompile(`@[0-9a-f]{40}\s*(#.*)?$`)
