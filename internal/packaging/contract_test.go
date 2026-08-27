// Package packaging binds the canonical artifacts of the repository-governance
// home — the workflow payloads, the composite actions, the canonical callers,
// the file family, the schemas, and the conformance vectors — to the contract
// through the one canonical contract-test set. The drift watcher itself never
// drifts: this is the only contract-test set of the home.
package packaging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/t33n-software/repository-governance/internal/canonical"
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
}

var callers = []string{
	"hosting-platforms/github/workflows/callers/go/ci.yml",
	"hosting-platforms/github/workflows/callers/go/ci-full.yml",
	"hosting-platforms/github/workflows/callers/go/codeql.yml",
	"hosting-platforms/github/workflows/callers/go/dependency-review.yml",
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
		".github/workflows/reusable-ci-go.yml":             {"contents: read"},
		".github/workflows/reusable-codeql-go.yml":         {"actions: read", "contents: read", "security-events: write"},
		".github/workflows/reusable-dependency-review.yml": {"contents: read"},
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

func TestPayloadsAreOrganizationAgnostic(t *testing.T) {
	for _, payload := range payloads {
		t.Run(filepath.Base(payload), func(t *testing.T) {
			content := readArtifact(t, payload)
			for _, forbidden := range []string{"t33n", "pkg.dev", "googleapis", "gcloud"} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s carries the organization-bound literal %q", payload, forbidden)
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

func TestPayloadsCarryTheGateJobNames(t *testing.T) {
	expectations := map[string]string{
		".github/workflows/reusable-ci-go.yml":             "name: ${{ matrix.name }}",
		".github/workflows/reusable-codeql-go.yml":         "name: CodeQL (go)",
		".github/workflows/reusable-dependency-review.yml": "name: Dependency admission review",
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
		"hosting-platforms/github/workflows/callers/go/ci.yml":                {"  quality:\n    name: Quality gates\n", "quality_class: linux-only", "contents: read"},
		"hosting-platforms/github/workflows/callers/go/ci-full.yml":           {"  quality:\n    name: Quality gates\n", "quality_class: full", "contents: read"},
		"hosting-platforms/github/workflows/callers/go/codeql.yml":            {"  analyze:\n    name: CodeQL\n", "actions: read", "contents: read", "security-events: write"},
		"hosting-platforms/github/workflows/callers/go/dependency-review.yml": {"  dependency-review:\n    name: Dependency review\n", "contents: read"},
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

func TestCanonicalFileFamily(t *testing.T) {
	gitattributes := readArtifact(t, "hosting-platforms/github/files/gitattributes/.gitattributes")
	if gitattributes != "* text=auto eol=lf\n" {
		t.Fatalf("the gitattributes core drifted: %q", gitattributes)
	}

	gitignore := readArtifact(t, "hosting-platforms/github/files/gitignore/.gitignore")
	if !strings.HasSuffix(gitignore, "# -- project additions below this line --\n") {
		t.Fatal("the gitignore core must end with the project-block mark")
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

func TestSchemasConform(t *testing.T) {
	schemas := []string{
		"schemas/repo-bindings/v1/repo-bindings.schema.json",
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
