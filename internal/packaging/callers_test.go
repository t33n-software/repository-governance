package packaging

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/t33n-software/repository-governance/internal/canonical"
)

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
