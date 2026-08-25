// Package canonical implements the conformance verifier domain of the
// repository-governance home: the tenant binding manifest (repo-bindings/v1),
// the caller-hash and canonical-file proofs, the config-seam conformance
// proof, the tool-pin admission proof, and the license-lane wiring proof.
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
)

// BindingsSchemaVersion is the canonical repo-bindings schema version
// published by this home.
const BindingsSchemaVersion = 1

// QualitySchemaVersion is the config-seam schema version this verifier proves.
const QualitySchemaVersion = 4

const (
	maxBindingsBytes = 1 << 20
	maxCallerCount   = 16
)

var (
	repositoryPattern = regexp.MustCompile(`^[a-z0-9-]+/[a-z0-9-]+$`)
	shaPattern        = regexp.MustCompile(`^[0-9a-f]{40}$`)
	hashPattern       = regexp.MustCompile(`^[0-9a-f]{64}$`)
	versionPattern    = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	callerFilePattern = regexp.MustCompile(`^\.github/workflows/[a-z0-9-]+\.yml$`)
	masterPattern     = regexp.MustCompile(`^hosting-platforms/github/workflows/callers/[a-z0-9-]+/[a-z0-9-]+\.yml$`)
)

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

// CallerBinding binds one tenant caller to its canonical master and hash.
type CallerBinding struct {
	File   string
	Master string
	SHA256 string
}

// FileBinding binds one canonical file topic to its path and content hash.
type FileBinding struct {
	Path   string
	SHA256 string
}

// FileBindings carries the canonical file topics. The gitignore topic is
// verified in prefix mode (canonical core plus marked project block); every
// other topic is byte-identical.
type FileBindings struct {
	Lefthook      FileBinding
	Gitattributes FileBinding
	Gitignore     FileBinding
	Dependabot    FileBinding
}

// CodeownersBinding binds the ownership render values.
type CodeownersBinding struct {
	Path         string
	DefaultOwner string
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

// Bindings is the tenant's canonical binding manifest (repo-bindings/v1).
type Bindings struct {
	SchemaVersion int
	Home          HomePin
	Class         Class
	Callers       []CallerBinding
	Files         FileBindings
	Codeowners    CodeownersBinding
	Quality       QualityBinding
	Tools         ToolsBinding
}

// bindingsDocument is the wire form of the manifest. Unknown fields are
// rejected at decode time.
type bindingsDocument struct {
	SchemaVersion int            `json:"schemaVersion"`
	Home          homeJSON       `json:"home"`
	Class         classJSON      `json:"class"`
	Callers       []callerJSON   `json:"callers"`
	Files         filesJSON      `json:"files"`
	Codeowners    codeownersJSON `json:"codeowners"`
	Quality       qualityJSON    `json:"quality"`
	Tools         toolsJSON      `json:"tools"`
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

type filesJSON struct {
	Lefthook      fileJSON `json:"lefthook"`
	Gitattributes fileJSON `json:"gitattributes"`
	Gitignore     fileJSON `json:"gitignore"`
	Dependabot    fileJSON `json:"dependabot"`
}

type codeownersJSON struct {
	Path         string `json:"path"`
	DefaultOwner string `json:"defaultOwner"`
}

type qualityJSON struct {
	Config        string `json:"config"`
	SchemaVersion int    `json:"schemaVersion"`
}

type toolsJSON struct {
	Module         string `json:"module"`
	CatalogVersion int    `json:"catalogVersion"`
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
	if err := validateFiles(document.Files); err != nil {
		return Bindings{}, err
	}
	if err := validateCodeowners(document.Codeowners); err != nil {
		return Bindings{}, err
	}
	if err := validateQuality(document.Quality); err != nil {
		return Bindings{}, err
	}
	if err := validateTools(document.Tools); err != nil {
		return Bindings{}, err
	}
	callers, err := validateCallers(document.Callers)
	if err != nil {
		return Bindings{}, err
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
		Callers: callers,
		Files: FileBindings{
			Lefthook:      FileBinding(document.Files.Lefthook),
			Gitattributes: FileBinding(document.Files.Gitattributes),
			Gitignore:     FileBinding(document.Files.Gitignore),
			Dependabot:    FileBinding(document.Files.Dependabot),
		},
		Codeowners: CodeownersBinding{
			Path:         document.Codeowners.Path,
			DefaultOwner: document.Codeowners.DefaultOwner,
		},
		Quality: QualityBinding{
			Config:        document.Quality.Config,
			SchemaVersion: document.Quality.SchemaVersion,
		},
		Tools: ToolsBinding{
			Module:         document.Tools.Module,
			CatalogVersion: document.Tools.CatalogVersion,
		},
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
		{name: "gitignore", binding: files.Gitignore},
		{name: "dependabot", binding: files.Dependabot},
	} {
		if err := validateManifestPath("files."+topic.name+".path", topic.binding.Path); err != nil {
			return err
		}
		if !hashPattern.MatchString(topic.binding.SHA256) {
			return fmt.Errorf("files.%s.sha256 must be a lowercase SHA-256 hex digest", topic.name)
		}
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
