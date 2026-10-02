package canonical

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// territoryConfigRoot is the territory-relative root of the config family
// tree.
const territoryConfigRoot = "configs"

// deliveryLaneLeafPath is the tenant-relative path of the delivery-lane leaf
// whose invariants the single-project lanes guard.
const deliveryLaneLeafPath = "tsconfig.node.json"

// pnpmCatalogKey is the tenant-owned catalog key of the pnpm workspace
// document: its existence is governed, its content is the tenant's approved
// version set.
const pnpmCatalogKey = "catalog"

// categoryProofContract is the proof map of one registry entry.
type categoryProofContract struct {
	BaseByteIdentity bool     `json:"baseByteIdentity"`
	LeafInvariants   []string `json:"leafInvariants"`
	BehaviorGate     []string `json:"behaviorGate"`
}

// categoryRegistryEntry is one category of the pinned territory registry.
type categoryRegistryEntry struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Artifacts map[string]string     `json:"artifacts"`
	Proof     categoryProofContract `json:"proof"`
}

// categoryRegistryInstance is the decoded wire form of the pinned territory
// registry instance. The instance is hash-pinned by the binding; the decode
// carries the fields the verifier proves with and tolerates additive
// territory-owned fields.
type categoryRegistryInstance struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Categories    []categoryRegistryEntry `json:"categories"`
}

// tsconfigDocument carries the two surfaces of a tsconfig leaf the verifier
// guards: the compiler options and the top-level include list.
type tsconfigDocument struct {
	CompilerOptions map[string]any
	Include         []string
}

// categoryTerritoryDir derives the territory directory of a category's
// artifacts for one family: configs/<family>/<category-id>/ — the
// declaration, the folder, and the artifacts are provably the same path
// without a mapping table.
func categoryTerritoryDir(family, category string) string {
	return territoryConfigRoot + "/" + family + "/" + category + "/"
}

// territoryPnpmBaselinePath derives the territory-relative path of the pnpm
// fortress baseline: the family-root form, category-independent.
func territoryPnpmBaselinePath() string {
	return territoryConfigRoot + "/pnpm/pnpm-workspace.base.yaml"
}

// categoryRegistryLookup resolves the declared category's registry entry; a
// nil result declares an unregistered category.
func categoryRegistryLookup(instance categoryRegistryInstance, category string) *categoryRegistryEntry {
	for index := range instance.Categories {
		if instance.Categories[index].ID == category {
			return &instance.Categories[index]
		}
	}
	return nil
}

// verifyToolchainConfig runs the category binding's fail-closed proofs — the
// levels one to four of the registry's verification contract. The fifth
// level stays the tenant's toolchain behavior gate and is never substituted
// by this proof. A tenant without a toolchain binding and without a declared
// category skips every config-topic proof; a declaration on exactly one
// surface is a finding, never a silent state.
func (v Verifier) verifyToolchainConfig(bindings Bindings, seam qualityConfigDocument) []Finding {
	toolchain := bindings.Toolchain
	seamCategory := seam.Toolchain.Category
	check := "toolchain config"
	if toolchain == nil {
		if seamCategory != "" {
			return []Finding{mismatchFinding(check,
				fmt.Sprintf("the config seam declares the category %q, but the binding manifest carries no toolchain section", seamCategory))}
		}
		return nil
	}
	if seamCategory == "" {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the binding manifest declares the category %q, but the config seam declares none", toolchain.Category))}
	}
	if seamCategory != toolchain.Category {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the config seam declares the category %q, but the binding manifest binds %q", seamCategory, toolchain.Category))}
	}
	if seam.Toolchain.Language != "node-typescript" {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the toolchain section binds the node-typescript territory's config families, but the config seam declares the language %q", seam.Toolchain.Language))}
	}

	instance, err := v.pinnedCategoryRegistry(toolchain.Registry)
	if err != nil {
		return []Finding{mismatchFinding(check, err.Error())}
	}
	entry := categoryRegistryLookup(instance, toolchain.Category)
	if entry == nil {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the pinned registry carries no entry for the declared category %q", toolchain.Category))}
	}

	findings := make([]Finding, 0)
	findings = append(findings, v.verifyConfigArtifacts(check, *entry, toolchain.Category, toolchain.Artifacts)...)
	findings = append(findings, v.verifyConfigDeliveryLane(check, toolchain)...)
	findings = append(findings, v.verifyConfigPnpmWorkspace(check, *toolchain)...)
	return findings
}

// pinnedCategoryRegistry reads and proves the pinned registry instance: the
// territory file's content hash must equal the bound hash, and the decode
// must carry a schema version and at least one category.
func (v Verifier) pinnedCategoryRegistry(pin RegistryPin) (categoryRegistryInstance, error) {
	var instance categoryRegistryInstance
	contents, err := v.ReadTerritory(pin.Path)
	if err != nil {
		return instance, fmt.Errorf("read the pinned registry %s: %w", pin.Path, err)
	}
	if hash := Sum256Hex(contents); hash != pin.SHA256 {
		return instance, fmt.Errorf("the pinned registry hash %s diverges from the bound hash %s", hash, pin.SHA256)
	}
	if err := json.Unmarshal(contents, &instance); err != nil {
		return instance, fmt.Errorf("the pinned registry must contain valid JSON: %w", err)
	}
	if instance.SchemaVersion < 1 {
		return instance, fmt.Errorf("the pinned registry carries no schema version")
	}
	if len(instance.Categories) == 0 {
		return instance, fmt.Errorf("the pinned registry carries no categories")
	}
	return instance, nil
}

// verifyConfigArtifacts proves the byte-identity artifacts (level two):
// every bound artifact's territory form and tenant form must carry the
// declared hash, and the registry's artifact mapping must equal the derived
// category folder — the declaration, the folder, and the artifacts are one
// path.
func (v Verifier) verifyConfigArtifacts(check string, entry categoryRegistryEntry, category string, artifacts []ArtifactBinding) []Finding {
	findings := make([]Finding, 0)
	for _, artifact := range artifacts {
		dir, mapped := entry.Artifacts[artifact.Family]
		if !mapped {
			findings = append(findings, mismatchFinding(check,
				fmt.Sprintf("the pinned registry entry carries no artifact mapping for the family %q", artifact.Family)))
			continue
		}
		if derived := categoryTerritoryDir(artifact.Family, category); dir != derived {
			findings = append(findings, mismatchFinding(check,
				fmt.Sprintf("the registry maps the family %q to %q, but the category identity derives %q", artifact.Family, dir, derived)))
			continue
		}
		territoryPath := dir + path.Base(artifact.Path)
		territoryContents, err := v.ReadTerritory(territoryPath)
		if err != nil {
			findings = append(findings, readErrorFinding(check, territoryPath, err))
			continue
		}
		if hash := Sum256Hex(territoryContents); hash != artifact.SHA256 {
			findings = append(findings, mismatchFinding(check,
				fmt.Sprintf("the territory artifact %s hash %s diverges from the bound hash %s", territoryPath, hash, artifact.SHA256)))
			continue
		}
		tenantContents, err := v.ReadTenant(artifact.Path)
		if err != nil {
			findings = append(findings, readErrorFinding(check, artifact.Path, err))
			continue
		}
		if hash := Sum256Hex(tenantContents); hash != artifact.SHA256 {
			findings = append(findings, mismatchFinding(check,
				fmt.Sprintf("the tenant %s hash %s diverges from the bound hash %s", artifact.Path, hash, artifact.SHA256)))
		}
	}
	return findings
}

// verifyConfigDeliveryLane proves the delivery-lane leaf's lane invariants
// (level three) and the declared source roots (level four). The guard set is
// implemented per category lane; a lane without an implemented guard set is
// a fail-closed finding, never a skipped proof.
func (v Verifier) verifyConfigDeliveryLane(check string, toolchain *ToolchainBindings) []Finding {
	contents, err := v.ReadTenant(deliveryLaneLeafPath)
	if err != nil {
		return []Finding{readErrorFinding(check, deliveryLaneLeafPath, err)}
	}
	document, err := parseTSConfigDocument(contents)
	if err != nil {
		return []Finding{mismatchFinding(check, err.Error())}
	}
	findings := make([]Finding, 0)
	switch toolchain.Category {
	case "single-project/bundler-owned":
		findings = append(findings, guardCompilerOption(check, document.CompilerOptions, deliveryLaneLeafPath, "noEmit", true)...)
	case "single-project/direct-node":
		findings = append(findings, guardCompilerOption(check, document.CompilerOptions, deliveryLaneLeafPath, "module", "Node20")...)
		findings = append(findings, guardCompilerOption(check, document.CompilerOptions, deliveryLaneLeafPath, "moduleResolution", "Node16")...)
		findings = append(findings, guardCompilerOption(check, document.CompilerOptions, deliveryLaneLeafPath, "noEmit", false)...)
	case "single-project/direct-node/no-prebuild":
		findings = append(findings, guardCompilerOption(check, document.CompilerOptions, deliveryLaneLeafPath, "module", "Node20")...)
		findings = append(findings, guardCompilerOption(check, document.CompilerOptions, deliveryLaneLeafPath, "moduleResolution", "Node16")...)
	default:
		findings = append(findings, mismatchFinding(check,
			fmt.Sprintf("the verifier carries no implemented leaf-guard set for the category %q", toolchain.Category)))
		return findings
	}
	findings = append(findings, verifyConfigSourceRoots(check, document, toolchain.SourceRoots)...)
	return findings
}

// guardCompilerOption proves one invariant guard on the delivery lane's
// compiler options: the property must exist and carry exactly the guarded
// value.
func guardCompilerOption(check string, options map[string]any, leafPath, property string, guarded any) []Finding {
	value, found := options[property]
	if !found {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the delivery lane %s carries no %s property for its lane invariant", leafPath, property))}
	}
	if !reflect.DeepEqual(value, guarded) {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the delivery lane %s carries %s = %v, but the lane invariant guards %v", leafPath, property, value, guarded))}
	}
	return nil
}

// verifyConfigSourceRoots proves the declared source roots (level four):
// every declared root must be covered by at least one include entry of the
// delivery lane leaf.
func verifyConfigSourceRoots(check string, document tsconfigDocument, sourceRoots []string) []Finding {
	findings := make([]Finding, 0)
	for _, root := range sourceRoots {
		covered := false
		for _, entry := range document.Include {
			if includeCoversRoot(entry, root) {
				covered = true
				break
			}
		}
		if !covered {
			findings = append(findings, mismatchFinding(check,
				fmt.Sprintf("the declared source root %q is covered by no include entry of the delivery lane", root)))
		}
	}
	return findings
}

// includeCoversRoot decides whether one include entry covers a declared
// source root: the entry, with a leading ./ stripped, must equal the root or
// extend it below the same segment.
func includeCoversRoot(entry, root string) bool {
	normalized := strings.TrimPrefix(entry, "./")
	if normalized == root {
		return true
	}
	return strings.HasPrefix(normalized, root+"/")
}

// verifyConfigPnpmWorkspace proves the pnpm workspace document's policy
// invariants (class four with class-three elements): every governed key of
// the territory's fortress baseline must carry the exact baseline value in
// the tenant document; the catalog key must exist while its content stays
// the tenant's approved version set.
func (v Verifier) verifyConfigPnpmWorkspace(check string, toolchain ToolchainBindings) []Finding {
	tenantContents, err := v.ReadTenant(toolchain.PnpmWorkspace.Path)
	if err != nil {
		return []Finding{readErrorFinding(check, toolchain.PnpmWorkspace.Path, err)}
	}
	var tenant map[string]any
	if err := yaml.Unmarshal(tenantContents, &tenant); err != nil {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the tenant %s must be a valid YAML mapping: %v", toolchain.PnpmWorkspace.Path, err))}
	}
	baselinePath := territoryPnpmBaselinePath()
	baselineContents, err := v.ReadTerritory(baselinePath)
	if err != nil {
		return []Finding{readErrorFinding(check, baselinePath, err)}
	}
	var baseline map[string]any
	if err := yaml.Unmarshal(baselineContents, &baseline); err != nil {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the territory baseline %s must be a valid YAML mapping: %v", baselinePath, err))}
	}
	findings := make([]Finding, 0)
	for _, key := range sortedYAMLKeys(baseline) {
		guarded := baseline[key]
		value, found := tenant[key]
		if !found {
			findings = append(findings, mismatchFinding(check,
				fmt.Sprintf("the tenant %s misses the governed key %q", toolchain.PnpmWorkspace.Path, key)))
			continue
		}
		if key == pnpmCatalogKey {
			continue
		}
		if !reflect.DeepEqual(value, guarded) {
			findings = append(findings, mismatchFinding(check,
				fmt.Sprintf("the tenant %s carries %s = %v, but the fortress baseline binds %v", toolchain.PnpmWorkspace.Path, key, value, guarded)))
		}
	}
	return findings
}

// sortedYAMLKeys returns the mapping keys in sorted order so the findings
// stay deterministic across runs.
func sortedYAMLKeys(mapping map[string]any) []string {
	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// parseTSConfigDocument extracts the guarded surfaces of a tsconfig
// document. TypeScript configuration files carry comments; the extraction
// strips comment forms outside string literals before the JSON decode.
func parseTSConfigDocument(contents []byte) (tsconfigDocument, error) {
	stripped, err := stripJSONComments(contents)
	if err != nil {
		return tsconfigDocument{}, err
	}
	var document struct {
		CompilerOptions map[string]any `json:"compilerOptions"`
		Include         []string       `json:"include"`
	}
	if err := json.Unmarshal([]byte(stripped), &document); err != nil {
		return tsconfigDocument{}, fmt.Errorf("the tsconfig document must carry valid JSON with comments: %w", err)
	}
	return tsconfigDocument{CompilerOptions: document.CompilerOptions, Include: document.Include}, nil
}

// stripJSONComments removes // line comments and /* block comments from a
// JSON-with-comments document without touching comment-like forms inside
// string literals.
func stripJSONComments(contents []byte) (string, error) {
	var out strings.Builder
	inString := false
	escaped := false
	for index := 0; index < len(contents); index++ {
		char := contents[index]
		if inString {
			out.WriteByte(char)
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
			out.WriteByte(char)
		case '/':
			if index+1 < len(contents) && contents[index+1] == '/' {
				for index+1 < len(contents) && contents[index+1] != '\n' {
					index++
				}
			} else if index+1 < len(contents) && contents[index+1] == '*' {
				end := bytes.Index(contents[index+2:], []byte("*/"))
				if end < 0 {
					return "", fmt.Errorf("the document carries an unterminated block comment")
				}
				index = index + 2 + end + 1
			} else {
				out.WriteByte(char)
			}
		default:
			out.WriteByte(char)
		}
	}
	if inString {
		return "", fmt.Errorf("the document carries an unterminated string")
	}
	return out.String(), nil
}
