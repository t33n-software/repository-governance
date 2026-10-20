package packaging

import (
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

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
