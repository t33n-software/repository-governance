// Package packaging binds the canonical artifacts of the repository-governance
// home — the workflow payloads, the composite actions, the canonical callers,
// the file family, the schemas, and the conformance vectors — to the contract
// through the one canonical contract-test set. The drift watcher itself never
// drifts: this is the only contract-test set of the home.
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
	"regexp"
	"strings"
	"testing"

	"github.com/t33n-software/repository-governance/internal/canonical"
	"go.yaml.in/yaml/v3"
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

func TestPayloadsCarryOnlyWorkflowCall(t *testing.T) {
	for _, payload := range payloads {
		t.Run(filepath.Base(payload), func(t *testing.T) {
			content := readArtifact(t, payload)
			if !strings.Contains(content, "on:\n  workflow_call:") {
				t.Fatalf("%s must carry only on: workflow_call", payload)
			}
			for _, forbidden := range []string{"\n  push:", "\n  pull_request:", "\n  schedule:", "\n  release:", "\n  issues:"} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s carries a self-trigger %q", payload, forbidden)
				}
			}
		})
	}
}

func TestPayloadsPinEveryAction(t *testing.T) {
	for _, payload := range payloads {
		t.Run(filepath.Base(payload), func(t *testing.T) {
			for _, line := range strings.Split(readArtifact(t, payload), "\n") {
				trimmed := strings.TrimSpace(line)
				uses, found := strings.CutPrefix(trimmed, "uses: ")
				if !found {
					continue
				}
				if !actionSHA.MatchString(uses) {
					t.Fatalf("%s carries an unpinned action reference: %q", payload, uses)
				}
				// The home self-reference tracks the canonical home pin (bound by
				// TestConformancePayloadTracksTheCanonicalPin); its version comment
				// lands with the home release the pin belongs to. Third-party
				// references always carry the release version comment.
				if strings.HasPrefix(uses, "t33n-software/repository-governance/") {
					continue
				}
				if !strings.Contains(uses, " # v") {
					t.Fatalf("%s carries no version comment: %q", payload, uses)
				}
			}
		})
	}
}

func TestPayloadsCarryNoForbiddenPatterns(t *testing.T) {
	for _, payload := range payloads {
		t.Run(filepath.Base(payload), func(t *testing.T) {
			content := readArtifact(t, payload)
			if strings.Contains(content, "pull_request_target") {
				t.Fatalf("%s carries pull_request_target", payload)
			}
			if strings.Contains(content, "cache: true") {
				t.Fatalf("%s carries a cache trust authority", payload)
			}
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(line, "  GOFLAGS:") {
					t.Fatalf("%s carries workflow-level GOFLAGS", payload)
				}
			}
		})
	}
}

func TestPayloadsPermissionMatrix(t *testing.T) {
	matrix := map[string][]string{
		".github/workflows/reusable-ci-go.yml":                 {"contents: read"},
		".github/workflows/reusable-codeql-go.yml":             {"actions: read", "contents: read", "security-events: write"},
		".github/workflows/reusable-dependency-review.yml":     {"contents: read"},
		".github/workflows/reusable-release-config.yml":        {"contents: read"},
		".github/workflows/reusable-canonical-conformance.yml": {"contents: read"},
	}
	for payload, permissions := range matrix {
		t.Run(filepath.Base(payload), func(t *testing.T) {
			content := readArtifact(t, payload)
			block := "permissions:\n"
			for index, permission := range permissions {
				block += "  " + permission
				if index < len(permissions)-1 {
					block += "\n"
				}
			}
			if !strings.Contains(content, block) {
				t.Fatalf("%s must declare exactly %v", payload, permissions)
			}
		})
	}
}

func TestPayloadsBoundedExecution(t *testing.T) {
	for _, payload := range payloads {
		t.Run(filepath.Base(payload), func(t *testing.T) {
			content := readArtifact(t, payload)
			if !strings.Contains(content, "concurrency:") || !strings.Contains(content, "cancel-in-progress: true") {
				t.Fatalf("%s must carry a bounded concurrency group", payload)
			}
			if !strings.Contains(content, "timeout-minutes:") {
				t.Fatalf("%s must carry an explicit job timeout", payload)
			}
		})
	}
}

// TestPayloadsAreOrganizationAgnostic proves the payloads carry no
// organization-bound values: no endpoints, no credentials, no organization
// data. The single permitted occurrence of the home coordinate is a
// self-reference in a uses line — the payload's reference to the home's own
// composite action at the canonical pin, which is the artifact's identity,
// never tenant or organization data.
func TestPayloadsAreOrganizationAgnostic(t *testing.T) {
	for _, payload := range payloads {
		t.Run(filepath.Base(payload), func(t *testing.T) {
			content := readArtifact(t, payload)
			for _, forbidden := range []string{"pkg.dev", "googleapis", "gcloud"} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s carries the organization-bound literal %q", payload, forbidden)
				}
			}
			for _, line := range strings.Split(content, "\n") {
				if !strings.Contains(line, "t33n") {
					continue
				}
				uses, found := strings.CutPrefix(strings.TrimSpace(line), "uses: ")
				if !found || !strings.HasPrefix(uses, "t33n-software/repository-governance/") {
					t.Fatalf("%s carries the organization-bound literal %q", payload, strings.TrimSpace(line))
				}
			}
		})
	}
}

// TestPayloadsProvisionTheExactPinnedToolchain proves the exact controlled
// toolchain provisioning: every Go-provisioning artifact carries the
// fail-closed resolution step that extracts the pinned version from the
// toolchain directive of the tenant's go.mod, and setup-go installs exactly
// that version through go-version. The drift-prone go-version-file form
// (which resolves the go directive to the latest patch) and the JSON
// extraction shim are forbidden everywhere.
func TestPayloadsProvisionTheExactPinnedToolchain(t *testing.T) {
	goArtifacts := []string{
		".github/workflows/reusable-ci-go.yml",
		".github/workflows/reusable-codeql-go.yml",
		".github/actions/setup-controlled-go/action.yml",
	}
	for _, artifact := range goArtifacts {
		t.Run(filepath.Base(artifact), func(t *testing.T) {
			content := readArtifact(t, artifact)
			for _, required := range []string{
				"- name: Resolve the pinned toolchain",
				"id: toolchain",
				`- name: Set up Go`,
				`$1 == "toolchain"`,
				"must carry exactly one pinned toolchain directive",
				"go-version: ${{ steps.toolchain.outputs.version }}",
			} {
				if !strings.Contains(content, required) {
					t.Fatalf("%s must carry %q", artifact, required)
				}
			}
			resolution := strings.Index(content, "- name: Resolve the pinned toolchain")
			setup := strings.Index(content, "- name: Set up Go")
			if setup < resolution {
				t.Fatalf("%s must resolve the pinned toolchain before setting up Go", artifact)
			}
			for _, forbidden := range []string{"go-version-file", "jq"} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s must not carry %q", artifact, forbidden)
				}
			}
		})
	}
}

// TestCIPayloadCarriesTheConstantPackProvisioningSeam proves the
// constant-size pack form of the CI payload: exactly one provisioning step
// running the orchestrator's provision mode and exactly one gate step, the
// provisioning step first. The seam is generic — it resolves every pack the
// tenant declares against the capability-pack registry — so it covers exactly
// the registry's known packs by construction: the payload never carries a
// step for a nonexistent pack and never lacks coverage for an existing one.
func TestCIPayloadCarriesTheConstantPackProvisioningSeam(t *testing.T) {
	content := readArtifact(t, ".github/workflows/reusable-ci-go.yml")
	const provisionCommand = "run: go tool -modfile tools/go.mod quality-gate provision"
	const gateCommand = "run: go tool -modfile tools/go.mod quality-gate"

	invocations := make([]string, 0, 2)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, gateCommand) {
			invocations = append(invocations, trimmed)
		}
	}
	if len(invocations) != 2 || invocations[0] != provisionCommand || invocations[1] != gateCommand {
		t.Fatalf("the CI payload must carry exactly one provisioning step before exactly one gate step, got %v", invocations)
	}
	if !strings.Contains(content, "- name: Provision the declared capabilities") {
		t.Fatal("the provisioning step must carry the canonical step name")
	}
	for _, forbidden := range []string{"capabilities/", "extends"} {
		if strings.Contains(content, forbidden) {
			t.Fatalf("the CI payload must stay pack-agnostic and never reference %q", forbidden)
		}
	}
}

// TestCIPayloadFetchesFullHistory proves the CI payload checks out the full
// history and every branch (fetch-depth: 0): governed tenant suites may carry
// provenance guards — such as the pin-ancestry proof against the merged lines
// — that fail-closed require the merged refs and their history. A shallow
// single-ref checkout blinds exactly these guards and turns their evidence
// into a missing-reference failure.
func TestCIPayloadFetchesFullHistory(t *testing.T) {
	content := readArtifact(t, ".github/workflows/reusable-ci-go.yml")
	checkoutIndex := strings.Index(content, "- name: Check out source")
	if checkoutIndex == -1 {
		t.Fatal("the CI payload must carry the checkout step")
	}
	checkoutStep := content[checkoutIndex:]
	if nextStep := strings.Index(checkoutStep, "- name: Resolve the pinned toolchain"); nextStep != -1 {
		checkoutStep = checkoutStep[:nextStep]
	}
	if !strings.Contains(checkoutStep, "fetch-depth: 0") {
		t.Fatal("the CI payload checkout must fetch the full history and every branch (fetch-depth: 0): governed tenant suites carry provenance guards that fail-closed require the merged lines and their history")
	}
}

// TestWorkflowAndActionFilesAreWellFormedYAML proves that every workflow and
// action file of the home parses as well-formed YAML: the five payloads, the
// six caller masters, the two composite actions, and the ten home workflow
// files (the dogfooding and lifecycle callers). Pin and reference edits touch
// these files as text; a broken indentation is invisible to the string-based
// guards but breaks every YAML consumer, so the drift watcher parses each
// file fail-closed.
func TestWorkflowAndActionFilesAreWellFormedYAML(t *testing.T) {
	artifacts := make([]string, 0, len(payloads)+len(callers)+len(compositeActions)+len(homeWorkflows))
	for _, group := range [][]string{payloads, callers, compositeActions, homeWorkflows} {
		artifacts = append(artifacts, group...)
	}
	for _, artifact := range artifacts {
		t.Run(artifact, func(t *testing.T) {
			var document any
			if err := yaml.Unmarshal([]byte(readArtifact(t, artifact)), &document); err != nil {
				t.Fatalf("%s must be well-formed YAML: %v", artifact, err)
			}
		})
	}
}

func TestPayloadsCarryTheGateJobNames(t *testing.T) {
	expectations := map[string]string{
		".github/workflows/reusable-ci-go.yml":                 "name: ${{ matrix.name }}",
		".github/workflows/reusable-codeql-go.yml":             "name: CodeQL (go)",
		".github/workflows/reusable-dependency-review.yml":     "name: Dependency admission review",
		".github/workflows/reusable-release-config.yml":        "name: GoReleaser configuration check",
		".github/workflows/reusable-canonical-conformance.yml": "name: Canonical bindings verification",
	}
	for payload, jobName := range expectations {
		t.Run(filepath.Base(payload), func(t *testing.T) {
			if !strings.Contains(readArtifact(t, payload), jobName) {
				t.Fatalf("%s must carry the gate job name %q", payload, jobName)
			}
		})
	}
}

func TestCallersReferenceTheHomePayloadsBySHA(t *testing.T) {
	for _, caller := range callers {
		t.Run(filepath.Base(caller), func(t *testing.T) {
			content := readArtifact(t, caller)
			found := false
			for _, line := range strings.Split(content, "\n") {
				trimmed := strings.TrimSpace(line)
				uses, ok := strings.CutPrefix(trimmed, "uses: ")
				if !ok {
					continue
				}
				found = true
				if !strings.HasPrefix(uses, "t33n-software/repository-governance/.github/workflows/reusable-") {
					t.Fatalf("%s references %q, not the home payload", caller, uses)
				}
				if !actionSHA.MatchString(uses) {
					t.Fatalf("%s carries no full-length SHA pin: %q", caller, uses)
				}
			}
			if !found {
				t.Fatalf("%s carries no uses reference", caller)
			}
		})
	}
}

func TestCallersCarryTheExactJobNamesAndGrants(t *testing.T) {
	expectations := map[string][]string{
		"hosting-platforms/github/workflows/callers/go/ci.yml":                    {"  quality:\n    name: Quality gates\n", "quality_class: linux-only", "contents: read"},
		"hosting-platforms/github/workflows/callers/go/ci-full.yml":               {"  quality:\n    name: Quality gates\n", "quality_class: full", "contents: read"},
		"hosting-platforms/github/workflows/callers/go/codeql.yml":                {"  analyze:\n    name: CodeQL\n", "actions: read", "contents: read", "security-events: write"},
		"hosting-platforms/github/workflows/callers/go/dependency-review.yml":     {"  dependency-review:\n    name: Dependency review\n", "contents: read"},
		"hosting-platforms/github/workflows/callers/go/release-config.yml":        {"  release-config:\n    name: Release configuration\n", "contents: read"},
		"hosting-platforms/github/workflows/callers/go/canonical-conformance.yml": {"  conformance:\n    name: Canonical conformance\n", "contents: read"},
	}
	for caller, required := range expectations {
		t.Run(filepath.Base(caller), func(t *testing.T) {
			content := readArtifact(t, caller)
			for _, fragment := range required {
				if !strings.Contains(content, fragment) {
					t.Fatalf("%s must carry %q", caller, fragment)
				}
			}
			if !strings.Contains(content, "permissions: {}") {
				t.Fatalf("%s must carry the workflow-level default-deny baseline", caller)
			}
		})
	}
}

func TestCallersCoverEverySharedLine(t *testing.T) {
	const allLines = `branches: [main, develop, "release/**", "support/**"]`
	for _, caller := range callers {
		t.Run(filepath.Base(caller), func(t *testing.T) {
			content := readArtifact(t, caller)
			if !strings.Contains(content, allLines) {
				t.Fatalf("%s must cover every shared line in the push and pull request triggers", caller)
			}
			if filepath.Base(caller) == "dependency-review.yml" {
				if strings.Count(content, allLines) != 1 {
					t.Fatalf("%s is pull-request-native and must carry exactly one trigger family", caller)
				}
				return
			}
			if strings.Count(content, allLines) != 2 {
				t.Fatalf("%s must cover push and pull request on every shared line", caller)
			}
		})
	}
}

func TestHomeCallersAreByteIdenticalToTheMasters(t *testing.T) {
	pairs := map[string]string{
		".github/workflows/ci.yml":                "hosting-platforms/github/workflows/callers/go/ci.yml",
		".github/workflows/codeql.yml":            "hosting-platforms/github/workflows/callers/go/codeql.yml",
		".github/workflows/dependency-review.yml": "hosting-platforms/github/workflows/callers/go/dependency-review.yml",
	}
	for own, master := range pairs {
		t.Run(filepath.Base(own), func(t *testing.T) {
			if readArtifact(t, own) != readArtifact(t, master) {
				t.Fatalf("the home caller %s diverges from the master %s", own, master)
			}
		})
	}
}

func TestCallerHashesRecordMatchesTheMasters(t *testing.T) {
	record := readArtifact(t, "hosting-platforms/github/workflows/callers/go/caller-hashes.json")
	var document struct {
		SchemaVersion int `json:"schemaVersion"`
		Callers       []struct {
			Master string `json:"master"`
			SHA256 string `json:"sha256"`
		} `json:"callers"`
	}
	if err := json.Unmarshal([]byte(record), &document); err != nil {
		t.Fatalf("the caller-hashes record is not valid JSON: %v", err)
	}
	if document.SchemaVersion != 1 {
		t.Fatalf("record schemaVersion = %d", document.SchemaVersion)
	}
	if len(document.Callers) != len(callers) {
		t.Fatalf("the record carries %d callers, want %d", len(document.Callers), len(callers))
	}
	for _, entry := range document.Callers {
		master := readArtifact(t, entry.Master)
		if hash := canonical.Sum256Hex([]byte(master)); hash != entry.SHA256 {
			t.Fatalf("the recorded hash of %s diverges: %s != %s", entry.Master, hash, entry.SHA256)
		}
	}
}

func TestConformanceActionTracksTheCanonicalPin(t *testing.T) {
	record := readArtifact(t, "hosting-platforms/github/workflows/callers/go/caller-hashes.json")
	var document struct {
		Home struct {
			SHA string `json:"sha"`
		} `json:"home"`
	}
	if err := json.Unmarshal([]byte(record), &document); err != nil {
		t.Fatalf("the caller-hashes record is not valid JSON: %v", err)
	}
	if !actionSHA.MatchString("@" + document.Home.SHA) {
		t.Fatalf("the canonical home pin %q is not a full-length commit SHA", document.Home.SHA)
	}

	action := readArtifact(t, ".github/actions/verify-canonical-files/action.yml")
	reference := ""
	for _, line := range strings.Split(action, "\n") {
		trimmed := strings.TrimSpace(line)
		uses, found := strings.CutPrefix(trimmed, "uses: ")
		if !found || !strings.Contains(uses, "/.github/actions/setup-controlled-go@") {
			continue
		}
		_, sha, _ := strings.Cut(uses, "@")
		reference = strings.TrimSpace(sha)
	}
	if reference == "" {
		t.Fatal("the verify-canonical-files action carries no setup-controlled-go reference")
	}
	if reference != document.Home.SHA {
		t.Fatalf("the verify-canonical-files action references setup-controlled-go at %s, not the canonical home pin %s", reference, document.Home.SHA)
	}
}

// TestConformancePayloadTracksTheCanonicalPin proves the conformance payload
// references the verify-canonical-files action at the canonical home pin: the
// reference must never lag behind or jump ahead of the pin the fleet binds.
func TestConformancePayloadTracksTheCanonicalPin(t *testing.T) {
	record := readArtifact(t, "hosting-platforms/github/workflows/callers/go/caller-hashes.json")
	var document struct {
		Home struct {
			SHA string `json:"sha"`
		} `json:"home"`
	}
	if err := json.Unmarshal([]byte(record), &document); err != nil {
		t.Fatalf("the caller-hashes record is not valid JSON: %v", err)
	}
	payload := readArtifact(t, ".github/workflows/reusable-canonical-conformance.yml")
	reference := ""
	for _, line := range strings.Split(payload, "\n") {
		trimmed := strings.TrimSpace(line)
		uses, found := strings.CutPrefix(trimmed, "uses: ")
		if !found || !strings.Contains(uses, "/.github/actions/verify-canonical-files@") {
			continue
		}
		_, sha, _ := strings.Cut(uses, "@")
		reference = strings.TrimSpace(sha)
	}
	if reference == "" {
		t.Fatal("the conformance payload carries no verify-canonical-files reference")
	}
	if reference != document.Home.SHA {
		t.Fatalf("the conformance payload references verify-canonical-files at %s, not the canonical home pin %s", reference, document.Home.SHA)
	}
}

func TestCanonicalFileFamily(t *testing.T) {
	gitattributes := readArtifact(t, "hosting-platforms/github/files/gitattributes/.gitattributes")
	if gitattributes != "* text=auto eol=lf\n"+
		"*.7z filter=lfs diff=lfs merge=lfs -text\n"+
		"*.duckdb filter=lfs diff=lfs merge=lfs -text\n"+
		"*.gguf filter=lfs diff=lfs merge=lfs -text\n"+
		"*.onnx filter=lfs diff=lfs merge=lfs -text\n"+
		"*.parquet filter=lfs diff=lfs merge=lfs -text\n"+
		"*.pdf filter=lfs diff=lfs merge=lfs -text\n"+
		"*.pt filter=lfs diff=lfs merge=lfs -text\n"+
		"*.safetensors filter=lfs diff=lfs merge=lfs -text\n"+
		"*.sqlite filter=lfs diff=lfs merge=lfs -text\n"+
		"*.sqlite3 filter=lfs diff=lfs merge=lfs -text\n"+
		"*.tar filter=lfs diff=lfs merge=lfs -text\n"+
		"*.tar.gz filter=lfs diff=lfs merge=lfs -text\n"+
		"*.tgz filter=lfs diff=lfs merge=lfs -text\n"+
		"*.zip filter=lfs diff=lfs merge=lfs -text\n" {
		t.Fatalf("the gitattributes core drifted: %q", gitattributes)
	}

	lefthook := readArtifact(t, "hosting-platforms/github/files/lefthook/lefthook.yml")
	for _, required := range []string{
		"git-governance --interactive never commit validate --message-file",
		"git-governance --interactive never validate pre-push --remote",
	} {
		if !strings.Contains(lefthook, required) {
			t.Fatalf("the lefthook core must call the Git CLI: %q", required)
		}
	}

	dependabot := readArtifact(t, "hosting-platforms/github/files/dependabot/dependabot-go.yml")
	for _, ecosystem := range []string{"package-ecosystem: gomod", "package-ecosystem: github-actions", "target-branch: develop"} {
		if !strings.Contains(dependabot, ecosystem) {
			t.Fatalf("the dependabot variant must carry %q", ecosystem)
		}
	}

	dependabotNode := readArtifact(t, "hosting-platforms/github/files/dependabot/dependabot-node.yml")
	for _, ecosystem := range []string{"package-ecosystem: npm", "package-ecosystem: github-actions", "target-branch: develop"} {
		if !strings.Contains(dependabotNode, ecosystem) {
			t.Fatalf("the dependabot node variant must carry %q", ecosystem)
		}
	}
}

// gitignoreGoldenSets binds every registered fragment set to its golden
// render under conformance/gitignore/.
var gitignoreGoldenSets = map[string][]string{
	"core":                              {"core"},
	"core-go":                           {"core", "go/core"},
	"core-node":                         {"core", "node/core"},
	"core-opentofu":                     {"core", "opentofu/core"},
	"core-opentofu-lockfiles-committed": {"core", "opentofu/core", "opentofu/lockfiles-committed"},
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
	fragments := []string{"core.gitignore", "go/core.gitignore", "node/core.gitignore", "opentofu/core.gitignore", "opentofu/lockfiles-committed.gitignore"}
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
	for _, fragment := range []string{"core.gitignore", "go/core.gitignore", "node/core.gitignore", "opentofu/core.gitignore", "opentofu/lockfiles-committed.gitignore"} {
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

func TestCodeownersTemplateAndMaterialization(t *testing.T) {
	template := readArtifact(t, "hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl")
	if !strings.Contains(template, "{{defaultOwner}}") {
		t.Fatal("the CODEOWNERS template must carry the defaultOwner token")
	}
	rendered := strings.ReplaceAll(template, "{{defaultOwner}}", "@CyberT33N")
	if own := readArtifact(t, ".github/CODEOWNERS"); own != rendered {
		t.Fatalf("the home's own CODEOWNERS is not the materialization of the template:\n%s", own)
	}
}

// TestConventionsTemplate proves the canonical rule-sets conventions README
// template: the full token surface, the canonical section structure, LF-only
// bytes, and value freedom — the template never carries an organization,
// repository, class, or rationale value.
func TestConventionsTemplate(t *testing.T) {
	template := readArtifact(t, "hosting-platforms/github/files/conventions/rule-sets-readme.md.tmpl")
	for _, token := range []string{"{{organization}}", "{{repository}}", "{{class}}", "{{platforms}}", "{{rationale}}"} {
		if !strings.Contains(template, token) {
			t.Fatalf("the conventions template must carry the %s token", token)
		}
	}
	for _, section := range []string{"## Canonical source", "## Family in use", "## Bound rule sets", "## Management"} {
		if !strings.Contains(template, section) {
			t.Fatalf("the conventions template must carry the section %q", section)
		}
	}
	if strings.Contains(template, "\r") {
		t.Fatal("the conventions template must be LF-only")
	}
	if strings.Contains(template, "t33n") {
		t.Fatal("the conventions template must stay organization-agnostic")
	}
}

func TestSchemasConform(t *testing.T) {
	schemas := []string{
		"schemas/repo-bindings/v1/repo-bindings.schema.json",
		"schemas/repo-bindings/v2/repo-bindings.schema.json",
		"schemas/caller-hashes/v1/caller-hashes.schema.json",
	}
	for _, schema := range schemas {
		t.Run(schema, func(t *testing.T) {
			contents := readArtifact(t, schema)
			var document map[string]any
			if err := json.Unmarshal([]byte(contents), &document); err != nil {
				t.Fatalf("the schema is not valid JSON: %v", err)
			}
			if document["$id"] == "" {
				t.Fatal("the schema must carry a canonical $id")
			}
			if document["additionalProperties"] != false {
				t.Fatal("the schema must reject unknown properties")
			}
		})
	}
}

func TestConformanceVectors(t *testing.T) {
	for _, name := range listVectors(t, "positive") {
		t.Run("positive/"+name, func(t *testing.T) {
			contents := readArtifact(t, "conformance/positive/"+name)
			if _, err := canonical.DecodeBindings([]byte(contents)); err != nil {
				t.Fatalf("positive vector %s must decode: %v", name, err)
			}
		})
	}
	for _, name := range listVectors(t, "negative") {
		t.Run("negative/"+name, func(t *testing.T) {
			contents := readArtifact(t, "conformance/negative/"+name)
			if _, err := canonical.DecodeBindings([]byte(contents)); err == nil {
				t.Fatalf("negative vector %s must be rejected", name)
			}
		})
	}
}

func TestHomeBindingsAreSelfConsistent(t *testing.T) {
	manifest := readArtifact(t, "repo-bindings.json")
	bindings, err := canonical.DecodeBindings([]byte(manifest))
	if err != nil {
		t.Fatalf("the home's own binding manifest must decode: %v", err)
	}
	if bindings.Home.Repository != "t33n-software/repository-governance" {
		t.Fatalf("the home manifest binds %q", bindings.Home.Repository)
	}
	for _, caller := range bindings.Callers {
		contents := readArtifact(t, caller.File)
		if hash := canonical.Sum256Hex([]byte(contents)); hash != caller.SHA256 {
			t.Fatalf("the home's own caller %s diverges from its manifest hash", caller.File)
		}
		if !strings.Contains(contents, "@"+bindings.Home.SHA) {
			t.Fatalf("the home's own caller %s does not reference the bound home SHA", caller.File)
		}
	}
}

func TestHomeGoModCarriesTheToolchainDirective(t *testing.T) {
	if _, err := canonical.ToolchainDirective([]byte(readArtifact(t, "go.mod"))); err != nil {
		t.Fatalf("the home go.mod must carry the toolchain directive: %v", err)
	}
}

func TestNoLegacyArtifacts(t *testing.T) {
	root := repoRoot(t)
	var legacy []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".build" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), "_BAK") || strings.HasSuffix(entry.Name(), ".yml_BAK") {
			legacy = append(legacy, path)
		}
		if strings.HasSuffix(entry.Name(), ".json") && strings.Contains(filepath.ToSlash(path), "/docs/") {
			legacy = append(legacy, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the tree: %v", err)
	}
	if len(legacy) > 0 {
		t.Fatalf("legacy artifacts are forbidden: %v", legacy)
	}
}

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
