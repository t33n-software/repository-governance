package packaging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/t33n-software/repository-governance/internal/canonical"
)

func TestSchemasConform(t *testing.T) {
	schemas := []string{
		"schemas/repo-bindings/v1/repo-bindings.schema.json",
		"schemas/repo-bindings/v2/repo-bindings.schema.json",
		"schemas/repo-bindings/v3/repo-bindings.schema.json",
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

// TestRepoBindingsSchemaDecoderLockstep proves the bidirectional lockstep of
// the v3 schema and the canonical decoder: every family enum value exists in
// the decoder's closed family sets and every decoder family exists in the
// enum; the same holds for the distribution axis. A one-sided entry is a
// finding, so schema and decoder can never drift apart in either direction.
func TestRepoBindingsSchemaDecoderLockstep(t *testing.T) {
	contents := readArtifact(t, "schemas/repo-bindings/v3/repo-bindings.schema.json")
	var schema struct {
		Properties struct {
			Distribution struct {
				Enum []string `json:"enum"`
			} `json:"distribution"`
			Toolchain struct {
				Properties struct {
					Artifacts struct {
						Items struct {
							Properties struct {
								Family struct {
									Enum []string `json:"enum"`
								} `json:"family"`
							} `json:"properties"`
						} `json:"items"`
					} `json:"artifacts"`
				} `json:"properties"`
			} `json:"toolchain"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(contents), &schema); err != nil {
		t.Fatalf("the v3 schema is not valid JSON: %v", err)
	}
	byteIdentity, composition := canonical.ArtifactFamilies()
	wantFamilies := append(append([]string{}, byteIdentity...), composition...)
	sort.Strings(wantFamilies)
	gotFamilies := append([]string{}, schema.Properties.Toolchain.Properties.Artifacts.Items.Properties.Family.Enum...)
	sort.Strings(gotFamilies)
	if !slices.Equal(gotFamilies, wantFamilies) {
		t.Fatalf("the schema family enum %v diverges from the decoder families %v", gotFamilies, wantFamilies)
	}
	gotDistribution := append([]string{}, schema.Properties.Distribution.Enum...)
	sort.Strings(gotDistribution)
	if !slices.Equal(gotDistribution, canonical.DistributionClasses()) {
		t.Fatalf("the schema distribution enum %v diverges from the decoder classes %v", gotDistribution, canonical.DistributionClasses())
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
