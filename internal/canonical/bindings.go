// Package canonical implements the conformance verifier domain of the
// repository-governance home: the tenant binding manifest (repo-bindings/v1),
// the caller-hash and canonical-file proofs, the CODEOWNERS and conventions
// README materialization proofs, the config-seam conformance proof, the
// tool-pin admission proof, and the license content proof orchestrated
// through the tenant-pinned hub CLI.
//
// The manifest is a typed trust boundary between the fleet and a tenant. It is
// strictly decoded, versioned, and owned by this home; every proof is
// fail-closed — missing or diverging evidence is never a pass.
package canonical

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
)

// BindingsSchemaVersion is the canonical repo-bindings schema version
// published by this home.
const BindingsSchemaVersion = 3

// QualitySchemaVersion is the config-seam schema version this verifier proves.
const QualitySchemaVersion = 4

const (
	maxBindingsBytes = 1 << 20
	maxCallerCount   = 16
	maxArtifactCount = 32
)

// byteIdentityArtifactFamilies is the closed set of byte-identity config
// families the verifier proves; the workspace document of the derived
// package manager is the class-four policy proof and binds through the
// registry's derivation block instead — never through an artifact row,
// because a second binding path for one file is a double truth.
var byteIdentityArtifactFamilies = map[string]struct{}{
	"tsconfig": {},
	"vitest":   {},
	"tsdown":   {},
	"eslint":   {},
}

// compositionArtifactFamilies is the closed set of composition config
// families: the tenant file renders from an ordered fragment list of its
// provenance home and proves the rendered governed region byte-exact.
var compositionArtifactFamilies = map[string]struct{}{
	"vscodeignore": {},
}

// distributionClasses is the closed artifact-class axis of the manifest: the
// distribution shape the fleet's declaration-coupling proofs bind against.
// It is a selection, not a fact: no ecosystem contract file declares it.
var distributionClasses = map[string]struct{}{
	"library":          {},
	"cli":              {},
	"vscode-extension": {},
	"container-image":  {},
	"reference-hub":    {},
}

var (
	repositoryPattern = regexp.MustCompile(`^[a-z0-9-]+/[a-z0-9-]+$`)
	shaPattern        = regexp.MustCompile(`^[0-9a-f]{40}$`)
	hashPattern       = regexp.MustCompile(`^[0-9a-f]{64}$`)
	versionPattern    = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	callerFilePattern = regexp.MustCompile(`^\.github/workflows/[a-z0-9-]+\.yml$`)
	masterPattern     = regexp.MustCompile(`^hosting-platforms/github/workflows/callers/[a-z0-9-]+/[a-z0-9-]+\.yml$`)
	categoryPattern   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*(?:/[a-z0-9]+(?:-[a-z0-9]+)*){0,2}$`)
	kebabNamePattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// ArtifactFamilies lists the closed artifact family sets the decoder accepts:
// the byte-identity families and the composition families, each sorted. The
// home's contract test proves the schema's family enum equals this union, so
// schema and decoder can never drift apart in one direction.
func ArtifactFamilies() (byteIdentity []string, composition []string) {
	byteIdentity = sortedKeys(byteIdentityArtifactFamilies)
	composition = sortedKeys(compositionArtifactFamilies)
	return byteIdentity, composition
}

// DistributionClasses lists the closed artifact-class axis the decoder
// accepts, sorted. The home's contract test proves the schema's distribution
// enum equals this set through the same lockstep direction.
func DistributionClasses() []string {
	return sortedKeys(distributionClasses)
}

// sortedKeys returns the map keys in sorted order for deterministic proofs.
func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// HomePin binds the home repository coordinate and its release identity. The
// SHA is the trust anchor; the version is documentation.
type HomePin struct {
	Repository string
	Version    string
	SHA        string
}

// Class binds the tenant's fleet classes.
type Class struct {
	QualityGates string
	CodeScanning bool
	LicenseHub   bool
}

// Distribution binds the optional artifact-class axis; the empty string
// declares no distribution class and skips the declaration-coupling proofs.
const DistributionUnset = ""

// CallerBinding binds one tenant caller to its canonical master and hash.
type CallerBinding struct {
	File   string
	Master string
	SHA256 string
}

// FileBinding binds one byte-identical canonical file topic to its path and
// content hash.
type FileBinding struct {
	Path   string
	SHA256 string
}

// GitignoreBinding binds the gitignore topic to its path, the ordered
// fragment list, and the hash of the rendered governed region.
type GitignoreBinding struct {
	Path      string
	Fragments []string
	SHA256    string
}

// FileBindings carries the canonical file topics. The byte-identical topics
// compare by hash; the gitignore topic composes at bind time from the bound
// fragment list and proves the rendered governed region byte-exact.
type FileBindings struct {
	Lefthook      FileBinding
	Gitattributes FileBinding
	Gitignore     GitignoreBinding
	Dependabot    FileBinding
}

// CodeownersBinding binds the ownership render values.
type CodeownersBinding struct {
	Path         string
	DefaultOwner string
}

// ConventionsBinding binds the rule-sets conventions README render values.
type ConventionsBinding struct {
	Path         string
	Organization string
	Repository   string
	Rationale    string
}

// QualityBinding binds the config-seam expectations.
type QualityBinding struct {
	Config        string
	SchemaVersion int
}

// ToolsBinding binds the tooling-module expectations.
type ToolsBinding struct {
	Module         string
	CatalogVersion int
}

// TerritoryPin binds the territory home coordinate and its trust anchor. The
// SHA is the trust anchor; the territory carries the pinned config artifacts.
type TerritoryPin struct {
	Repository string
	SHA        string
}

// RegistryPin binds the territory category registry instance: its
// territory-relative path and its content hash.
type RegistryPin struct {
	Path   string
	SHA256 string
}

// ArtifactBinding binds one config artifact topic: the tenant path, its
// config family, the content hash the verifier proves against both the tenant
// file and the pinned artifact of the provenance home, the optional
// provenance home (an empty home binds the toolchain territory), and the
// ordered composition fragment list of a composition family.
type ArtifactBinding struct {
	Family    string
	Path      string
	SHA256    string
	Home      string
	Fragments []string
}

// ToolchainBindings binds the territory config topics. A nil binding skips
// every config-topic proof for tenants that carry no category binding; a
// binding without the seam's category declaration is a finding, never a
// silent state. The binding carries selections only: the package manager and
// the workspace document path are derived from the ecosystem package contract
// and the tool's own resolution rule, never declared here.
type ToolchainBindings struct {
	Territory   TerritoryPin
	Registry    RegistryPin
	Category    string
	Artifacts   []ArtifactBinding
	SourceRoots []string
}

// Bindings is the tenant's canonical binding manifest (repo-bindings/v3).
// Distribution carries the optional artifact-class axis.
type Bindings struct {
	SchemaVersion int
	Home          HomePin
	Class         Class
	Distribution  string
	Callers       []CallerBinding
	Files         FileBindings
	Codeowners    CodeownersBinding
	// Conventions carries the conventions README render binding; nil skips
	// the proof for tenants that do not carry the family.
	Conventions *ConventionsBinding
	Quality     QualityBinding
	Tools       ToolsBinding
	// Toolchain carries the optional territory config-topic binding; nil
	// skips the category proofs for tenants that bind none.
	Toolchain *ToolchainBindings
}

// bindingsDocument is the wire form of the manifest. Unknown fields are
// rejected at decode time.
type bindingsDocument struct {
	SchemaVersion int              `json:"schemaVersion"`
	Home          homeJSON         `json:"home"`
	Class         classJSON        `json:"class"`
	Distribution  string           `json:"distribution"`
	Callers       []callerJSON     `json:"callers"`
	Files         filesJSON        `json:"files"`
	Codeowners    codeownersJSON   `json:"codeowners"`
	Conventions   *conventionsJSON `json:"conventions"`
	Quality       qualityJSON      `json:"quality"`
	Tools         toolsJSON        `json:"tools"`
	Toolchain     *toolchainJSON   `json:"toolchain"`
}

type homeJSON struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
	SHA        string `json:"sha"`
}

type classJSON struct {
	QualityGates string `json:"qualityGates"`
	CodeScanning bool   `json:"codeScanning"`
	LicenseHub   bool   `json:"licenseHub"`
}

type callerJSON struct {
	File   string `json:"file"`
	Master string `json:"master"`
	SHA256 string `json:"sha256"`
}

type fileJSON struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type gitignoreJSON struct {
	Path      string   `json:"path"`
	Fragments []string `json:"fragments"`
	SHA256    string   `json:"sha256"`
}

type filesJSON struct {
	Lefthook      fileJSON      `json:"lefthook"`
	Gitattributes fileJSON      `json:"gitattributes"`
	Gitignore     gitignoreJSON `json:"gitignore"`
	Dependabot    fileJSON      `json:"dependabot"`
}

type codeownersJSON struct {
	Path         string `json:"path"`
	DefaultOwner string `json:"defaultOwner"`
}

type conventionsJSON struct {
	Path         string `json:"path"`
	Organization string `json:"organization"`
	Repository   string `json:"repository"`
	Rationale    string `json:"rationale"`
}

type qualityJSON struct {
	Config        string `json:"config"`
	SchemaVersion int    `json:"schemaVersion"`
}

type toolsJSON struct {
	Module         string `json:"module"`
	CatalogVersion int    `json:"catalogVersion"`
}

type territoryJSON struct {
	Repository string `json:"repository"`
	SHA        string `json:"sha"`
}

type registryJSON struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type artifactJSON struct {
	Family    string   `json:"family"`
	Path      string   `json:"path"`
	SHA256    string   `json:"sha256"`
	Home      string   `json:"home"`
	Fragments []string `json:"fragments"`
}

type toolchainJSON struct {
	Territory   territoryJSON  `json:"territory"`
	Registry    registryJSON   `json:"registry"`
	Category    string         `json:"category"`
	Artifacts   []artifactJSON `json:"artifacts"`
	SourceRoots []string       `json:"sourceRoots"`
}

// DecodeBindings strictly decodes and validates the canonical binding
// manifest. Unknown fields, trailing documents, and invariant violations are
// rejected with a precise field error.
func DecodeBindings(contents []byte) (Bindings, error) {
	if len(contents) == 0 {
		return Bindings{}, errors.New("repo bindings must not be empty")
	}
	if len(contents) > maxBindingsBytes {
		return Bindings{}, fmt.Errorf("repo bindings must not exceed %d bytes", maxBindingsBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var document bindingsDocument
	if err := decoder.Decode(&document); err != nil {
		return Bindings{}, fmt.Errorf("repo bindings must contain valid JSON with known fields: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Bindings{}, errors.New("repo bindings must contain exactly one JSON document")
	}
	return validateDocument(document)
}

func validateDocument(document bindingsDocument) (Bindings, error) {
	if document.SchemaVersion != BindingsSchemaVersion {
		return Bindings{}, fmt.Errorf("schemaVersion must equal %d", BindingsSchemaVersion)
	}
	if err := validateHome(document.Home); err != nil {
		return Bindings{}, err
	}
	if err := validateClass(document.Class); err != nil {
		return Bindings{}, err
	}
	if err := validateDistribution(document.Distribution); err != nil {
		return Bindings{}, err
	}
	if err := validateFiles(document.Files); err != nil {
		return Bindings{}, err
	}
	if err := validateCodeowners(document.Codeowners); err != nil {
		return Bindings{}, err
	}
	if err := validateConventions(document.Conventions); err != nil {
		return Bindings{}, err
	}
	if err := validateQuality(document.Quality); err != nil {
		return Bindings{}, err
	}
	if err := validateTools(document.Tools); err != nil {
		return Bindings{}, err
	}
	toolchain, err := validateToolchain(document.Toolchain)
	if err != nil {
		return Bindings{}, err
	}
	callers, err := validateCallers(document.Callers)
	if err != nil {
		return Bindings{}, err
	}
	var conventions *ConventionsBinding
	if document.Conventions != nil {
		conventions = &ConventionsBinding{
			Path:         document.Conventions.Path,
			Organization: document.Conventions.Organization,
			Repository:   document.Conventions.Repository,
			Rationale:    document.Conventions.Rationale,
		}
	}

	return Bindings{
		SchemaVersion: document.SchemaVersion,
		Home: HomePin{
			Repository: document.Home.Repository,
			Version:    document.Home.Version,
			SHA:        document.Home.SHA,
		},
		Class: Class{
			QualityGates: document.Class.QualityGates,
			CodeScanning: document.Class.CodeScanning,
			LicenseHub:   document.Class.LicenseHub,
		},
		Distribution: document.Distribution,
		Callers:      callers,
		Files: FileBindings{
			Lefthook:      FileBinding(document.Files.Lefthook),
			Gitattributes: FileBinding(document.Files.Gitattributes),
			Gitignore:     GitignoreBinding(document.Files.Gitignore),
			Dependabot:    FileBinding(document.Files.Dependabot),
		},
		Codeowners: CodeownersBinding{
			Path:         document.Codeowners.Path,
			DefaultOwner: document.Codeowners.DefaultOwner,
		},
		Conventions: conventions,
		Quality: QualityBinding{
			Config:        document.Quality.Config,
			SchemaVersion: document.Quality.SchemaVersion,
		},
		Tools: ToolsBinding{
			Module:         document.Tools.Module,
			CatalogVersion: document.Tools.CatalogVersion,
		},
		Toolchain: toolchain,
	}, nil
}

func validateHome(home homeJSON) error {
	if !repositoryPattern.MatchString(home.Repository) {
		return fmt.Errorf("home.repository must be an owner/repository coordinate: %q", home.Repository)
	}
	if !shaPattern.MatchString(home.SHA) {
		return errors.New("home.sha must be a full-length lowercase commit SHA")
	}
	if home.Version != "" && !versionPattern.MatchString(home.Version) {
		return fmt.Errorf("home.version must be a release tag such as v1.0.0: %q", home.Version)
	}
	return nil
}

func validateClass(class classJSON) error {
	switch class.QualityGates {
	case "full", "linux-only", "pending":
		return nil
	default:
		return fmt.Errorf("class.qualityGates must be full, linux-only, or pending: %q", class.QualityGates)
	}
}

// validateDistribution validates the optional artifact-class axis; an empty
// axis declares no class and is valid.
func validateDistribution(distribution string) error {
	if distribution == DistributionUnset {
		return nil
	}
	if _, known := distributionClasses[distribution]; !known {
		return fmt.Errorf("distribution must be library, cli, vscode-extension, container-image, or reference-hub: %q", distribution)
	}
	return nil
}

func validateCallers(callers []callerJSON) ([]CallerBinding, error) {
	if len(callers) == 0 || len(callers) > maxCallerCount {
		return nil, fmt.Errorf("callers must contain between 1 and %d entries", maxCallerCount)
	}
	seenFiles := make(map[string]struct{}, len(callers))
	seenMasters := make(map[string]struct{}, len(callers))
	bindings := make([]CallerBinding, 0, len(callers))
	for _, caller := range callers {
		if !callerFilePattern.MatchString(caller.File) {
			return nil, fmt.Errorf("callers file must be a workflow path under .github/workflows: %q", caller.File)
		}
		if !masterPattern.MatchString(caller.Master) {
			return nil, fmt.Errorf("callers master must be a canonical caller path: %q", caller.Master)
		}
		if !hashPattern.MatchString(caller.SHA256) {
			return nil, fmt.Errorf("callers sha256 must be a lowercase SHA-256 hex digest: %q", caller.SHA256)
		}
		if _, found := seenFiles[caller.File]; found {
			return nil, fmt.Errorf("callers file must be unique: %q", caller.File)
		}
		if _, found := seenMasters[caller.Master]; found {
			return nil, fmt.Errorf("callers master must be unique: %q", caller.Master)
		}
		seenFiles[caller.File] = struct{}{}
		seenMasters[caller.Master] = struct{}{}
		bindings = append(bindings, CallerBinding(caller))
	}
	return bindings, nil
}

func validateFiles(files filesJSON) error {
	for _, topic := range []struct {
		name    string
		binding fileJSON
	}{
		{name: "lefthook", binding: files.Lefthook},
		{name: "gitattributes", binding: files.Gitattributes},
		{name: "dependabot", binding: files.Dependabot},
	} {
		if err := validateManifestPath("files."+topic.name+".path", topic.binding.Path); err != nil {
			return err
		}
		if !hashPattern.MatchString(topic.binding.SHA256) {
			return fmt.Errorf("files.%s.sha256 must be a lowercase SHA-256 hex digest", topic.name)
		}
	}
	if err := validateManifestPath("files.gitignore.path", files.Gitignore.Path); err != nil {
		return err
	}
	if err := ValidateGitignoreFragments(files.Gitignore.Fragments); err != nil {
		return fmt.Errorf("files.gitignore.fragments %s", err)
	}
	if !hashPattern.MatchString(files.Gitignore.SHA256) {
		return errors.New("files.gitignore.sha256 must be a lowercase SHA-256 hex digest")
	}
	return nil
}

func validateCodeowners(codeowners codeownersJSON) error {
	if err := validateManifestPath("codeowners.path", codeowners.Path); err != nil {
		return err
	}
	if codeowners.DefaultOwner == "" {
		return errors.New("codeowners.defaultOwner must not be empty")
	}
	return nil
}

// validateConventions validates the optional conventions render binding; a
// nil binding skips the family.
func validateConventions(conventions *conventionsJSON) error {
	if conventions == nil {
		return nil
	}
	if err := validateManifestPath("conventions.path", conventions.Path); err != nil {
		return err
	}
	if conventions.Organization == "" {
		return errors.New("conventions.organization must not be empty")
	}
	if conventions.Repository == "" {
		return errors.New("conventions.repository must not be empty")
	}
	if conventions.Rationale == "" {
		return errors.New("conventions.rationale must not be empty")
	}
	return nil
}

func validateQuality(quality qualityJSON) error {
	if err := validateManifestPath("quality.config", quality.Config); err != nil {
		return err
	}
	if quality.SchemaVersion != QualitySchemaVersion {
		return fmt.Errorf("quality.schemaVersion must equal %d", QualitySchemaVersion)
	}
	return nil
}

func validateTools(tools toolsJSON) error {
	if err := validateManifestPath("tools.module", tools.Module); err != nil {
		return err
	}
	if tools.CatalogVersion != 1 {
		return fmt.Errorf("tools.catalogVersion must equal %d", 1)
	}
	return nil
}

// validateToolchain validates the optional territory config-topic binding; a
// nil binding skips the category proofs for tenants that bind none.
func validateToolchain(toolchain *toolchainJSON) (*ToolchainBindings, error) {
	if toolchain == nil {
		return nil, nil
	}
	if !repositoryPattern.MatchString(toolchain.Territory.Repository) {
		return nil, fmt.Errorf("toolchain.territory.repository must be an owner/repository coordinate: %q", toolchain.Territory.Repository)
	}
	if !shaPattern.MatchString(toolchain.Territory.SHA) {
		return nil, errors.New("toolchain.territory.sha must be a full-length lowercase commit SHA")
	}
	if err := validateManifestPath("toolchain.registry.path", toolchain.Registry.Path); err != nil {
		return nil, err
	}
	if !hashPattern.MatchString(toolchain.Registry.SHA256) {
		return nil, errors.New("toolchain.registry.sha256 must be a lowercase SHA-256 hex digest")
	}
	if !categoryPattern.MatchString(toolchain.Category) {
		return nil, fmt.Errorf("toolchain.category must be a hierarchical kebab category path: %q", toolchain.Category)
	}
	if len(toolchain.Artifacts) == 0 || len(toolchain.Artifacts) > maxArtifactCount {
		return nil, fmt.Errorf("toolchain.artifacts must contain between 1 and %d entries", maxArtifactCount)
	}
	seen := make(map[string]struct{}, len(toolchain.Artifacts))
	artifacts := make([]ArtifactBinding, 0, len(toolchain.Artifacts))
	for _, artifact := range toolchain.Artifacts {
		_, byteIdentity := byteIdentityArtifactFamilies[artifact.Family]
		_, composition := compositionArtifactFamilies[artifact.Family]
		if !byteIdentity && !composition {
			return nil, fmt.Errorf("toolchain.artifacts family must be eslint, tsconfig, tsdown, vitest, or vscodeignore: %q", artifact.Family)
		}
		if err := validateManifestPath("toolchain.artifacts.path", artifact.Path); err != nil {
			return nil, err
		}
		if !hashPattern.MatchString(artifact.SHA256) {
			return nil, fmt.Errorf("toolchain.artifacts sha256 for the family %s must be a lowercase SHA-256 hex digest", artifact.Family)
		}
		if artifact.Home != "" && !kebabNamePattern.MatchString(artifact.Home) {
			return nil, fmt.Errorf("toolchain.artifacts home for the family %s must be a lowercase kebab repository name: %q", artifact.Family, artifact.Home)
		}
		if byteIdentity && len(artifact.Fragments) > 0 {
			return nil, fmt.Errorf("toolchain.artifacts fragments bind only on composition families, not on the byte-identity family %q", artifact.Family)
		}
		if composition {
			if err := validateCompositionFragments(artifact.Fragments); err != nil {
				return nil, fmt.Errorf("toolchain.artifacts fragments for the family %s %s", artifact.Family, err)
			}
		}
		key := artifact.Family + "/" + artifact.Path
		if _, found := seen[key]; found {
			return nil, fmt.Errorf("toolchain.artifacts must not repeat a family and path: %q", key)
		}
		seen[key] = struct{}{}
		artifacts = append(artifacts, ArtifactBinding(artifact))
	}
	seenRoots := make(map[string]struct{}, len(toolchain.SourceRoots))
	sourceRoots := make([]string, 0, len(toolchain.SourceRoots))
	for _, root := range toolchain.SourceRoots {
		if err := validateManifestPath("toolchain.sourceRoots", root); err != nil {
			return nil, err
		}
		if _, found := seenRoots[root]; found {
			return nil, fmt.Errorf("toolchain.sourceRoots must not repeat a root: %q", root)
		}
		seenRoots[root] = struct{}{}
		sourceRoots = append(sourceRoots, root)
	}
	return &ToolchainBindings{
		Territory: TerritoryPin{
			Repository: toolchain.Territory.Repository,
			SHA:        toolchain.Territory.SHA,
		},
		Registry: RegistryPin{
			Path:   toolchain.Registry.Path,
			SHA256: toolchain.Registry.SHA256,
		},
		Category:    toolchain.Category,
		Artifacts:   artifacts,
		SourceRoots: sourceRoots,
	}, nil
}

// validateCompositionFragments binds the composition fragment list form: a
// non-empty list, the composition core first, every name in the kebab
// fragment grammar, and no duplicates.
func validateCompositionFragments(fragments []string) error {
	if len(fragments) == 0 {
		return errors.New("must contain at least the core fragment")
	}
	if fragments[0] != "core" {
		return fmt.Errorf("must begin with the core fragment, got %q", fragments[0])
	}
	seen := make(map[string]struct{}, len(fragments))
	for _, fragment := range fragments {
		if !kebabNamePattern.MatchString(fragment) {
			return fmt.Errorf("must be a canonical fragment name, got %q", fragment)
		}
		if _, found := seen[fragment]; found {
			return fmt.Errorf("must not repeat a fragment, got %q", fragment)
		}
		seen[fragment] = struct{}{}
	}
	return nil
}

// validateManifestPath rejects absolute paths, parent traversal, and
// backslashes in manifest-declared repository-relative paths.
func validateManifestPath(field, path string) error {
	if path == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if path[0] == '/' || path[0] == '\\' {
		return fmt.Errorf("%s must be repository-relative: %q", field, path)
	}
	if len(path) > 1 && path[1] == ':' {
		return fmt.Errorf("%s must be repository-relative: %q", field, path)
	}
	for _, segment := range splitPath(path) {
		if segment == ".." {
			return fmt.Errorf("%s must not contain parent traversal: %q", field, path)
		}
	}
	for _, r := range path {
		if r == '\\' || r < 0x20 {
			return fmt.Errorf("%s must use forward slashes and no control characters: %q", field, path)
		}
	}
	return nil
}

func splitPath(path string) []string {
	segments := make([]string, 0, 4)
	start := 0
	for index := 0; index <= len(path); index++ {
		if index == len(path) || path[index] == '/' {
			segments = append(segments, path[start:index])
			start = index + 1
		}
	}
	return segments
}
