package canonical

import (
	"fmt"
	"strings"
	"testing"
)

// testTerritorySHA is the fixture territory trust anchor.
const testTerritorySHA = "1234567890123456789012345678901234567890"

// testPnpmBaseline is the fixture fortress baseline of the territory.
const testPnpmBaseline = "minimumReleaseAge: 10080\ntrustPolicy: no-downgrade\ncatalog: {}\ncatalogMode: strict\ncleanupUnusedCatalogs: true\n"

// testPnpmTenant is a conforming tenant workspace: the governed keys carry
// the baseline values, the catalog content is the tenant's own set.
const testPnpmTenant = "minimumReleaseAge: 10080\ntrustPolicy: no-downgrade\ncatalog:\n  zod: 4.6.5\ncatalogMode: strict\ncleanupUnusedCatalogs: true\n"

// testBundlerLeaf is a conforming bundler-owned delivery lane with comments
// and a schema URL whose slashes must survive the comment stripping.
const testBundlerLeaf = "{\n  \"$schema\": \"https://json.schemastore.org/tsconfig\",\n  /* the lane deltas */\n  \"compilerOptions\": {\n    \"noEmit\": true // the bundler owns dist\n  },\n  \"include\": [\"src/**/*\"]\n}\n"

// testDirectNodeLeaf is a conforming direct-node delivery lane.
const testDirectNodeLeaf = "{\n  \"compilerOptions\": {\n    \"module\": \"Node20\",\n    \"moduleResolution\": \"Node16\",\n    \"noEmit\": false\n  },\n  \"include\": [\"./src/**/*\"]\n}\n"

// testNoPrebuildLeaf is a conforming no-prebuild delivery lane.
const testNoPrebuildLeaf = "{\n  \"compilerOptions\": {\n    \"module\": \"Node20\",\n    \"moduleResolution\": \"Node16\"\n  },\n  \"include\": [\"src/**/*\"]\n}\n"

func territoryRegistryJSON() string {
	return `{"schemaVersion":1,"categories":[` +
		`{"id":"single-project/bundler-owned","title":"Bundler-owned","artifacts":{"tsconfig":"configs/tsconfig/single-project/bundler-owned/","tsdown":"configs/tsdown/single-project/bundler-owned/"},"proof":{"baseByteIdentity":true,"leafInvariants":["node.noEmit == true"],"behaviorGate":["install","typecheck","test","build"]}},` +
		`{"id":"single-project/direct-node","title":"Direct-node","artifacts":{"tsconfig":"configs/tsconfig/single-project/direct-node/"},"proof":{"baseByteIdentity":true,"behaviorGate":["install","typecheck","test","build"]}},` +
		`{"id":"single-project/direct-node/no-prebuild","title":"No-prebuild","artifacts":{"tsconfig":"configs/tsconfig/single-project/direct-node/no-prebuild/"},"proof":{"baseByteIdentity":true,"behaviorGate":["install","typecheck","test","build"]}},` +
		`{"id":"monorepo","title":"Monorepo","artifacts":{"tsconfig":"configs/tsconfig/monorepo/"},"proof":{"baseByteIdentity":true,"behaviorGate":["install","typecheck","test","build"]}}]}`
}

// toolchainFixture carries the territory and tenant surfaces of the
// config-topic proofs.
type toolchainFixture struct {
	territory    map[string][]byte
	tenant       map[string][]byte
	territoryErr error
	tenantErr    error
}

func (fixture toolchainFixture) verifier() Verifier {
	return Verifier{
		ReadTerritory: func(path string) ([]byte, error) {
			if fixture.territoryErr != nil {
				return nil, fixture.territoryErr
			}
			contents, found := fixture.territory[path]
			if !found {
				return nil, fmt.Errorf("no such territory file: %s", path)
			}
			return contents, nil
		},
		ReadTenant: func(path string) ([]byte, error) {
			if fixture.tenantErr != nil {
				return nil, fixture.tenantErr
			}
			contents, found := fixture.tenant[path]
			if !found {
				return nil, fmt.Errorf("no such tenant file: %s", path)
			}
			return contents, nil
		},
	}
}

// passingToolchainFixture binds the conforming bundler-owned surfaces.
func passingToolchainFixture() toolchainFixture {
	territory, tenant := toolchainFixtureContents()
	return toolchainFixture{territory: territory, tenant: tenant}
}

func toolchainFixtureContents() (map[string][]byte, map[string][]byte) {
	territory := map[string][]byte{
		"configs/registry.json": []byte(territoryRegistryJSON()),
		"configs/tsconfig/single-project/bundler-owned/tsconfig.base.json": []byte("base-canonical"),
		"configs/tsdown/single-project/bundler-owned/tsdown.config.ts":     []byte("tsdown-canonical"),
		"configs/pnpm/pnpm-workspace.base.yaml":                            []byte(testPnpmBaseline),
	}
	tenant := map[string][]byte{
		"tsconfig.base.json":  []byte("base-canonical"),
		"tsdown.config.ts":    []byte("tsdown-canonical"),
		"tsconfig.node.json":  []byte(testBundlerLeaf),
		"pnpm-workspace.yaml": []byte(testPnpmTenant),
	}
	return territory, tenant
}

func toolchainBinding() *ToolchainBindings {
	return &ToolchainBindings{
		Territory: TerritoryPin{Repository: "t33n-software/node-quality-authority", SHA: testTerritorySHA},
		Registry:  RegistryPin{Path: "configs/registry.json", SHA256: Sum256Hex([]byte(territoryRegistryJSON()))},
		Category:  "single-project/bundler-owned",
		Artifacts: []ArtifactBinding{
			{Family: "tsconfig", Path: "tsconfig.base.json", SHA256: Sum256Hex([]byte("base-canonical"))},
			{Family: "tsdown", Path: "tsdown.config.ts", SHA256: Sum256Hex([]byte("tsdown-canonical"))},
		},
		PnpmWorkspace: PnpmWorkspaceBinding{Path: "pnpm-workspace.yaml"},
		SourceRoots:   []string{"src"},
	}
}

func seamWithCategory(category string) qualityConfigDocument {
	return qualityConfigDocument{Toolchain: qualityToolchainJSON{Language: "node-typescript", Category: category}}
}

func TestVerifyToolchainConfigPass(t *testing.T) {
	fixture := passingToolchainFixture()
	findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
	if len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyToolchainConfigDeclarationSurfaces(t *testing.T) {
	t.Run("seam declares without a binding", func(t *testing.T) {
		fixture := passingToolchainFixture()
		findings := fixture.verifier().verifyToolchainConfig(Bindings{}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "carries no toolchain section")
	})
	t.Run("no declaration on either surface", func(t *testing.T) {
		fixture := passingToolchainFixture()
		if findings := fixture.verifier().verifyToolchainConfig(Bindings{}, qualityConfigDocument{}); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})
	t.Run("binding declares without a seam category", func(t *testing.T) {
		fixture := passingToolchainFixture()
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, qualityConfigDocument{})
		assertFindingContains(t, findings, "the config seam declares none")
	})
	t.Run("divergent declarations", func(t *testing.T) {
		fixture := passingToolchainFixture()
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/direct-node"))
		assertFindingContains(t, findings, "the binding manifest binds")
	})
	t.Run("a foreign seam language", func(t *testing.T) {
		fixture := passingToolchainFixture()
		seam := seamWithCategory("single-project/bundler-owned")
		seam.Toolchain.Language = "go"
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seam)
		assertFindingContains(t, findings, "declares the language \"go\"")
	})
	t.Run("an undeclared seam language", func(t *testing.T) {
		fixture := passingToolchainFixture()
		seam := seamWithCategory("single-project/bundler-owned")
		seam.Toolchain.Language = ""
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seam)
		assertFindingContains(t, findings, "declares the language \"\"")
	})
}

func TestVerifyToolchainConfigRegistryProofs(t *testing.T) {
	t.Run("territory read error", func(t *testing.T) {
		fixture := passingToolchainFixture()
		fixture.territoryErr = fmt.Errorf("boom")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "boom")
	})
	t.Run("registry hash mismatch", func(t *testing.T) {
		territory, tenant := toolchainFixtureContents()
		territory["configs/registry.json"] = []byte(`{"schemaVersion":1,"categories":[]}`)
		fixture := toolchainFixture{territory: territory, tenant: tenant}
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "the pinned registry hash")
	})
	t.Run("invalid registry JSON", func(t *testing.T) {
		territory, tenant := toolchainFixtureContents()
		registry := toolchainBinding().Registry
		registry.SHA256 = Sum256Hex([]byte("not json"))
		territory["configs/registry.json"] = []byte("not json")
		binding := toolchainBinding()
		binding.Registry = registry
		fixture := toolchainFixture{territory: territory, tenant: tenant}
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: binding}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "valid JSON")
	})
	t.Run("registry without a schema version", func(t *testing.T) {
		registryJSON := `{"categories":[{"id":"single-project/bundler-owned","artifacts":{}}]}`
		binding := toolchainBinding()
		binding.Registry.SHA256 = Sum256Hex([]byte(registryJSON))
		territory, tenant := toolchainFixtureContents()
		territory["configs/registry.json"] = []byte(registryJSON)
		fixture := toolchainFixture{territory: territory, tenant: tenant}
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: binding}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "no schema version")
	})
	t.Run("registry without categories", func(t *testing.T) {
		registryJSON := `{"schemaVersion":1,"categories":[]}`
		binding := toolchainBinding()
		binding.Registry.SHA256 = Sum256Hex([]byte(registryJSON))
		territory, tenant := toolchainFixtureContents()
		territory["configs/registry.json"] = []byte(registryJSON)
		fixture := toolchainFixture{territory: territory, tenant: tenant}
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: binding}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "no categories")
	})
	t.Run("unregistered category", func(t *testing.T) {
		registryJSON := `{"schemaVersion":1,"categories":[{"id":"single-project/direct-node","artifacts":{}}]}`
		binding := toolchainBinding()
		binding.Registry.SHA256 = Sum256Hex([]byte(registryJSON))
		territory, tenant := toolchainFixtureContents()
		territory["configs/registry.json"] = []byte(registryJSON)
		fixture := toolchainFixture{territory: territory, tenant: tenant}
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: binding}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "carries no entry")
	})
}

func TestVerifyConfigArtifactsProofs(t *testing.T) {
	t.Run("unmapped family", func(t *testing.T) {
		registryJSON := `{"schemaVersion":1,"categories":[{"id":"single-project/bundler-owned","artifacts":{"tsdown":"configs/tsdown/single-project/bundler-owned/"}}]}`
		binding := toolchainBinding()
		binding.Registry.SHA256 = Sum256Hex([]byte(registryJSON))
		territory, tenant := toolchainFixtureContents()
		territory["configs/registry.json"] = []byte(registryJSON)
		fixture := toolchainFixture{territory: territory, tenant: tenant}
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: binding}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "no artifact mapping for the family")
	})
	t.Run("folder identity mismatch", func(t *testing.T) {
		registryJSON := `{"schemaVersion":1,"categories":[{"id":"single-project/bundler-owned","artifacts":{"tsconfig":"configs/tsconfig/other-lane/"}}]}`
		binding := toolchainBinding()
		binding.Registry.SHA256 = Sum256Hex([]byte(registryJSON))
		territory, tenant := toolchainFixtureContents()
		territory["configs/registry.json"] = []byte(registryJSON)
		fixture := toolchainFixture{territory: territory, tenant: tenant}
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: binding}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "the category identity derives")
	})
	t.Run("territory artifact read error", func(t *testing.T) {
		fixture := passingToolchainFixture()
		delete(fixture.territory, "configs/tsconfig/single-project/bundler-owned/tsconfig.base.json")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "no such territory file")
	})
	t.Run("territory artifact hash mismatch", func(t *testing.T) {
		fixture := passingToolchainFixture()
		fixture.territory["configs/tsconfig/single-project/bundler-owned/tsconfig.base.json"] = []byte("drifted")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "the territory artifact")
	})
	t.Run("tenant artifact read error", func(t *testing.T) {
		fixture := passingToolchainFixture()
		delete(fixture.tenant, "tsdown.config.ts")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "no such tenant file")
	})
	t.Run("tenant artifact hash mismatch", func(t *testing.T) {
		fixture := passingToolchainFixture()
		fixture.tenant["tsconfig.base.json"] = []byte("drifted")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "the tenant tsconfig.base.json hash")
	})
}

func TestVerifyConfigDeliveryLaneProofs(t *testing.T) {
	laneFindings := func(category, leaf string, sourceRoots []string) []Finding {
		fixture := passingToolchainFixture()
		fixture.tenant["tsconfig.node.json"] = []byte(leaf)
		toolchain := ToolchainBindings{Category: category, SourceRoots: sourceRoots}
		return fixture.verifier().verifyConfigDeliveryLane("toolchain config", &toolchain)
	}
	t.Run("bundler-owned pass", func(t *testing.T) {
		if findings := laneFindings("single-project/bundler-owned", testBundlerLeaf, []string{"src"}); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})
	t.Run("bundler-owned missing invariant", func(t *testing.T) {
		findings := laneFindings("single-project/bundler-owned", "{\n  \"compilerOptions\": {},\n  \"include\": [\"src/**/*\"]\n}\n", nil)
		assertFindingContains(t, findings, "carries no noEmit property")
	})
	t.Run("bundler-owned wrong invariant", func(t *testing.T) {
		findings := laneFindings("single-project/bundler-owned", "{\n  \"compilerOptions\": { \"noEmit\": false },\n  \"include\": [\"src/**/*\"]\n}\n", nil)
		assertFindingContains(t, findings, "the lane invariant guards true")
	})
	t.Run("direct-node pass", func(t *testing.T) {
		if findings := laneFindings("single-project/direct-node", testDirectNodeLeaf, []string{"src"}); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})
	t.Run("direct-node wrong module pair", func(t *testing.T) {
		findings := laneFindings("single-project/direct-node", "{\n  \"compilerOptions\": { \"module\": \"Preserve\", \"moduleResolution\": \"Bundler\", \"noEmit\": false },\n  \"include\": [\"src/**/*\"]\n}\n", nil)
		assertFindingContains(t, findings, "module = Preserve")
	})
	t.Run("no-prebuild pass", func(t *testing.T) {
		if findings := laneFindings("single-project/direct-node/no-prebuild", testNoPrebuildLeaf, nil); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})
	t.Run("monorepo carries no implemented guard set", func(t *testing.T) {
		findings := laneFindings("monorepo", testBundlerLeaf, nil)
		assertFindingContains(t, findings, "no implemented leaf-guard set")
	})
	t.Run("leaf read error", func(t *testing.T) {
		fixture := passingToolchainFixture()
		delete(fixture.tenant, "tsconfig.node.json")
		toolchain := ToolchainBindings{Category: "single-project/bundler-owned"}
		findings := fixture.verifier().verifyConfigDeliveryLane("toolchain config", &toolchain)
		assertFindingContains(t, findings, "no such tenant file")
	})
	t.Run("leaf is not valid JSON with comments", func(t *testing.T) {
		findings := laneFindings("single-project/bundler-owned", "not json\n", nil)
		assertFindingContains(t, findings, "valid JSON with comments")
	})
	t.Run("leaf carries an unterminated block comment", func(t *testing.T) {
		findings := laneFindings("single-project/bundler-owned", "{ /* open\n", nil)
		assertFindingContains(t, findings, "unterminated block comment")
	})
	t.Run("uncovered source root", func(t *testing.T) {
		findings := laneFindings("single-project/bundler-owned", testBundlerLeaf, []string{"tooling"})
		assertFindingContains(t, findings, "covered by no include entry")
	})
}

func TestVerifyConfigPnpmWorkspaceProofs(t *testing.T) {
	t.Run("missing governed key", func(t *testing.T) {
		fixture := passingToolchainFixture()
		fixture.tenant["pnpm-workspace.yaml"] = []byte("minimumReleaseAge: 10080\n")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "misses the governed key")
	})
	t.Run("divergent governed value", func(t *testing.T) {
		fixture := passingToolchainFixture()
		fixture.tenant["pnpm-workspace.yaml"] = []byte(strings.Replace(testPnpmTenant, "10080", "99999", 1))
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "the fortress baseline binds")
	})
	t.Run("tenant yaml is not a mapping", func(t *testing.T) {
		fixture := passingToolchainFixture()
		fixture.tenant["pnpm-workspace.yaml"] = []byte("[not, a, mapping]\n")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "valid YAML mapping")
	})
	t.Run("baseline read error", func(t *testing.T) {
		fixture := passingToolchainFixture()
		delete(fixture.territory, "configs/pnpm/pnpm-workspace.base.yaml")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "no such territory file")
	})
	t.Run("baseline yaml is not a mapping", func(t *testing.T) {
		fixture := passingToolchainFixture()
		fixture.territory["configs/pnpm/pnpm-workspace.base.yaml"] = []byte("- not\n- a mapping\n")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "the territory baseline")
	})
	t.Run("tenant read error", func(t *testing.T) {
		fixture := passingToolchainFixture()
		fixture.tenantErr = fmt.Errorf("boom")
		findings := fixture.verifier().verifyToolchainConfig(Bindings{Toolchain: toolchainBinding()}, seamWithCategory("single-project/bundler-owned"))
		assertFindingContains(t, findings, "boom")
	})
}

func TestStripJSONComments(t *testing.T) {
	t.Run("preserves slashes inside strings", func(t *testing.T) {
		stripped, err := stripJSONComments([]byte("{\"a\": \"https://example.org/x\"}\n"))
		if err != nil {
			t.Fatalf("stripJSONComments: %v", err)
		}
		if stripped != "{\"a\": \"https://example.org/x\"}\n" {
			t.Fatalf("stripped = %q", stripped)
		}
	})
	t.Run("strips line and block comments outside strings", func(t *testing.T) {
		stripped, err := stripJSONComments([]byte("{\n  // line\n  \"a\": 1, /* block */\n  \"b\": 2\n}\n"))
		if err != nil {
			t.Fatalf("stripJSONComments: %v", err)
		}
		if !strings.Contains(stripped, "\"a\": 1,") || strings.Contains(stripped, "line") || strings.Contains(stripped, "block") {
			t.Fatalf("stripped = %q", stripped)
		}
	})
	t.Run("escaped quote stays inside the string", func(t *testing.T) {
		stripped, err := stripJSONComments([]byte("{\"a\": \"quote \\\" then // not a comment\"}\n"))
		if err != nil {
			t.Fatalf("stripJSONComments: %v", err)
		}
		if !strings.Contains(stripped, "// not a comment") {
			t.Fatalf("stripped = %q", stripped)
		}
	})
	t.Run("unterminated block comment", func(t *testing.T) {
		if _, err := stripJSONComments([]byte("{ /* open\n")); err == nil || !strings.Contains(err.Error(), "unterminated block comment") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("line comment runs to EOF without a newline", func(t *testing.T) {
		stripped, err := stripJSONComments([]byte("{\"a\": 1} // tail"))
		if err != nil {
			t.Fatalf("stripJSONComments: %v", err)
		}
		if !strings.Contains(stripped, "\"a\": 1}") || strings.Contains(stripped, "tail") {
			t.Fatalf("stripped = %q", stripped)
		}
	})
	t.Run("a bare slash at EOF survives", func(t *testing.T) {
		stripped, err := stripJSONComments([]byte("{\"a\": 1}/"))
		if err != nil {
			t.Fatalf("stripJSONComments: %v", err)
		}
		if !strings.HasSuffix(stripped, "/") {
			t.Fatalf("stripped = %q", stripped)
		}
	})
	t.Run("unterminated string", func(t *testing.T) {
		if _, err := stripJSONComments([]byte("{\"a\": \"open\n")); err == nil || !strings.Contains(err.Error(), "unterminated string") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a bare slash survives", func(t *testing.T) {
		stripped, err := stripJSONComments([]byte("{\"a\": \"/\"}\n"))
		if err != nil {
			t.Fatalf("stripJSONComments: %v", err)
		}
		if !strings.Contains(stripped, "\"/\"") {
			t.Fatalf("stripped = %q", stripped)
		}
	})
}

func TestIncludeCoversRoot(t *testing.T) {
	if !includeCoversRoot("src/**/*", "src") {
		t.Fatal("src/**/* must cover src")
	}
	if !includeCoversRoot("./src/**/*", "src") {
		t.Fatal("the ./ prefix must be stripped before the comparison")
	}
	if !includeCoversRoot("src", "src") {
		t.Fatal("the exact entry must cover the root")
	}
	if !includeCoversRoot("src/inner/**/*", "src") {
		t.Fatal("a nested entry must cover its parent root")
	}
	if includeCoversRoot("sources/**/*", "src") {
		t.Fatal("a shared prefix without the segment boundary must not cover")
	}
	if includeCoversRoot("test/**/*", "src") {
		t.Fatal("an unrelated entry must not cover the root")
	}
}

func TestCategoryTerritoryDirDerivesTheIdentity(t *testing.T) {
	if got := categoryTerritoryDir("tsconfig", "single-project/bundler-owned"); got != "configs/tsconfig/single-project/bundler-owned/" {
		t.Fatalf("dir = %q", got)
	}
	if got := categoryTerritoryDir("tsconfig", "monorepo"); got != "configs/tsconfig/monorepo/" {
		t.Fatalf("dir = %q", got)
	}
	if got := territoryPnpmBaselinePath(); got != "configs/pnpm/pnpm-workspace.base.yaml" {
		t.Fatalf("baseline path = %q", got)
	}
}

// toolchainJSONFragment is the valid toolchain wire form the decode tests
// mutate.
func toolchainJSONFragment(mutation func(s string) string) string {
	base := `"toolchain": {
  "territory": { "repository": "t33n-software/node-quality-authority", "sha": "` + testTerritorySHA + `" },
  "registry": { "path": "configs/registry.json", "sha256": "` + strings.Repeat("1", 64) + `" },
  "category": "single-project/bundler-owned",
  "artifacts": [
    { "family": "tsconfig", "path": "tsconfig.base.json", "sha256": "` + strings.Repeat("2", 64) + `" }
  ],
  "pnpmWorkspace": { "path": "pnpm-workspace.yaml" },
  "sourceRoots": ["src"]
}`
	return mutation(base)
}

func TestDecodeBindingsToolchainSection(t *testing.T) {
	t.Run("valid section decodes", func(t *testing.T) {
		manifest := "{" + `"schemaVersion":2,` +
			`"home":{"repository":"t33n-software/repository-governance","sha":"` + testHomeSHA + `"},` +
			`"class":{"qualityGates":"full","codeScanning":true,"licenseHub":false},` +
			`"callers":[{"file":".github/workflows/ci.yml","master":"hosting-platforms/github/workflows/callers/go/ci.yml","sha256":"` + strings.Repeat("a", 64) + `"}],` +
			`"files":{"lefthook":{"path":"lefthook.yml","sha256":"` + strings.Repeat("b", 64) + `"},"gitattributes":{"path":".gitattributes","sha256":"` + strings.Repeat("c", 64) + `"},"gitignore":{"path":".gitignore","fragments":["core"],"sha256":"` + strings.Repeat("d", 64) + `"},"dependabot":{"path":".github/dependabot.yml","sha256":"` + strings.Repeat("e", 64) + `"}},` +
			`"codeowners":{"path":".github/CODEOWNERS","defaultOwner":"@CyberT33N"},` +
			`"quality":{"config":"git-governance.quality.json","schemaVersion":4},` +
			`"tools":{"module":"tools/go.mod","catalogVersion":1},` +
			toolchainJSONFragment(func(s string) string { return s }) + "}"
		bindings, err := DecodeBindings([]byte(manifest))
		if err != nil {
			t.Fatalf("DecodeBindings: %v", err)
		}
		if bindings.Toolchain == nil {
			t.Fatal("the toolchain binding must decode")
		}
		if bindings.Toolchain.Category != "single-project/bundler-owned" || bindings.Toolchain.Territory.SHA != testTerritorySHA {
			t.Fatalf("toolchain = %+v", bindings.Toolchain)
		}
		if len(bindings.Toolchain.SourceRoots) != 1 || bindings.Toolchain.SourceRoots[0] != "src" {
			t.Fatalf("sourceRoots = %v", bindings.Toolchain.SourceRoots)
		}
	})
	t.Run("absent section decodes", func(t *testing.T) {
		manifest := "{" + `"schemaVersion":2,` +
			`"home":{"repository":"t33n-software/repository-governance","sha":"` + testHomeSHA + `"},` +
			`"class":{"qualityGates":"full","codeScanning":true,"licenseHub":false},` +
			`"callers":[{"file":".github/workflows/ci.yml","master":"hosting-platforms/github/workflows/callers/go/ci.yml","sha256":"` + strings.Repeat("a", 64) + `"}],` +
			`"files":{"lefthook":{"path":"lefthook.yml","sha256":"` + strings.Repeat("b", 64) + `"},"gitattributes":{"path":".gitattributes","sha256":"` + strings.Repeat("c", 64) + `"},"gitignore":{"path":".gitignore","fragments":["core"],"sha256":"` + strings.Repeat("d", 64) + `"},"dependabot":{"path":".github/dependabot.yml","sha256":"` + strings.Repeat("e", 64) + `"}},` +
			`"codeowners":{"path":".github/CODEOWNERS","defaultOwner":"@CyberT33N"},` +
			`"quality":{"config":"git-governance.quality.json","schemaVersion":4},` +
			`"tools":{"module":"tools/go.mod","catalogVersion":1}}`
		bindings, err := DecodeBindings([]byte(manifest))
		if err != nil {
			t.Fatalf("DecodeBindings: %v", err)
		}
		if bindings.Toolchain != nil {
			t.Fatal("the absent section must decode to nil")
		}
	})
	rejections := []struct {
		name     string
		section  string
		contains string
	}{
		{name: "bad territory repository", section: `"toolchain":{"territory":{"repository":"no coordinate","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "owner/repository"},
		{name: "bad territory sha", section: `"toolchain":{"territory":{"repository":"o/r","sha":"short"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "commit SHA"},
		{name: "bad registry path", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"../escape","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "parent traversal"},
		{name: "bad registry hash", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"short"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "hex digest"},
		{name: "bad category form", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"Single_Project","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "hierarchical kebab"},
		{name: "empty artifacts", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "between 1 and"},
		{name: "unknown family", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"eslint","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "tsconfig, vitest, or tsdown"},
		{name: "bad artifact path", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"../a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "parent traversal"},
		{name: "bad artifact hash", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"short"}],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "hex digest"},
		{name: "duplicate artifact", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"},{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("3", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"}}`, contains: "must not repeat"},
		{name: "bad pnpm path", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"../p.yaml"}}`, contains: "parent traversal"},
		{name: "bad source root", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"},"sourceRoots":["../escape"]}`, contains: "parent traversal"},
		{name: "duplicate source root", section: `"toolchain":{"territory":{"repository":"o/r","sha":"` + testTerritorySHA + `"},"registry":{"path":"configs/registry.json","sha256":"` + strings.Repeat("1", 64) + `"},"category":"monorepo","artifacts":[{"family":"tsconfig","path":"a.json","sha256":"` + strings.Repeat("2", 64) + `"}],"pnpmWorkspace":{"path":"p.yaml"},"sourceRoots":["src","src"]}`, contains: "must not repeat"},
	}
	for _, rejection := range rejections {
		t.Run(rejection.name, func(t *testing.T) {
			manifest := "{" + `"schemaVersion":2,` +
				`"home":{"repository":"t33n-software/repository-governance","sha":"` + testHomeSHA + `"},` +
				`"class":{"qualityGates":"full","codeScanning":true,"licenseHub":false},` +
				`"callers":[{"file":".github/workflows/ci.yml","master":"hosting-platforms/github/workflows/callers/go/ci.yml","sha256":"` + strings.Repeat("a", 64) + `"}],` +
				`"files":{"lefthook":{"path":"lefthook.yml","sha256":"` + strings.Repeat("b", 64) + `"},"gitattributes":{"path":".gitattributes","sha256":"` + strings.Repeat("c", 64) + `"},"gitignore":{"path":".gitignore","fragments":["core"],"sha256":"` + strings.Repeat("d", 64) + `"},"dependabot":{"path":".github/dependabot.yml","sha256":"` + strings.Repeat("e", 64) + `"}},` +
				`"codeowners":{"path":".github/CODEOWNERS","defaultOwner":"@CyberT33N"},` +
				`"quality":{"config":"git-governance.quality.json","schemaVersion":4},` +
				`"tools":{"module":"tools/go.mod","catalogVersion":1},` +
				rejection.section + "}"
			if _, err := DecodeBindings([]byte(manifest)); err == nil || !strings.Contains(err.Error(), rejection.contains) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestDecodeQualityConfigCategory(t *testing.T) {
	t.Run("declared", func(t *testing.T) {
		category, err := DecodeQualityConfigCategory([]byte(`{"schemaVersion":4,"toolchain":{"language":"node-typescript","version":"26.10.0","category":"single-project/bundler-owned"},"gates":[]}`))
		if err != nil {
			t.Fatalf("DecodeQualityConfigCategory: %v", err)
		}
		if category != "single-project/bundler-owned" {
			t.Fatalf("category = %q", category)
		}
	})
	t.Run("absent", func(t *testing.T) {
		category, err := DecodeQualityConfigCategory([]byte(`{"schemaVersion":4,"toolchain":{"language":"go","version":"1.26.6"},"gates":[]}`))
		if err != nil {
			t.Fatalf("DecodeQualityConfigCategory: %v", err)
		}
		if category != "" {
			t.Fatalf("category = %q", category)
		}
	})
	t.Run("decode error", func(t *testing.T) {
		if _, err := DecodeQualityConfigCategory([]byte(`not json`)); err == nil {
			t.Fatal("expected a rejection")
		}
	})
}
