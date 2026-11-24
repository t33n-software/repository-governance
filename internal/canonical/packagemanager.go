package canonical

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// PackageIdentity is the package manager derived from the ecosystem package
// contract: the tool name and its exact declared version.
type PackageIdentity struct {
	Name    string
	Version string
}

// corepackPattern is the Corepack name@version identity form. The form is a
// fleet-owned format primitive; which contract file and which field carry it
// is territory-owned data in the pinned registry's derivation block.
var corepackPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*@[0-9]+\.[0-9]+\.[0-9]+$`)

// IdentityExtraction declares how the identity is read from the contract
// document: the field name and the closed identity form.
type IdentityExtraction struct {
	Field string `json:"field"`
	Form  string `json:"form"`
}

// DerivationContract is the territory-owned declaration of how the fleet
// derives the package-manager identity and the conditional workspace
// document from the ecosystem package contract. The binding manifest never
// restates any of it; the pinned registry hash proves it. This is the
// Selection-vs-Fact boundary: the manifest selects the registry, the registry
// carries the facts, the fleet carries only the readers.
type DerivationContract struct {
	ContractFile    string             `json:"contractFile"`
	Format          string             `json:"format"`
	Identity        IdentityExtraction `json:"identity"`
	WorkspaceFiles  map[string]string  `json:"workspaceFiles"`
	BaselineFile    string             `json:"baselineFile"`
	TenantOwnedKeys []string           `json:"tenantOwnedKeys"`
}

// validateDerivation binds the closed grammar of the derivation block: a
// repository-relative contract file, the implemented format, the implemented
// identity form, a workspace mapping whose values are repository-relative
// paths, and — whenever a workspace document is mapped — a baseline file and
// at least one tenant-owned key. An unimplemented format or form is a
// fail-closed error, never a silent fallback.
func validateDerivation(derivation DerivationContract) error {
	if err := validateManifestPath("derivation.contractFile", derivation.ContractFile); err != nil {
		return err
	}
	if derivation.Format != "json" {
		return fmt.Errorf("derivation.format must be json for the implemented reader set: %q", derivation.Format)
	}
	if derivation.Identity.Field == "" {
		return errors.New("derivation.identity.field must not be empty")
	}
	if derivation.Identity.Form != "corepack" {
		return fmt.Errorf("derivation.identity.form must be corepack for the implemented parser set: %q", derivation.Identity.Form)
	}
	for name, workspacePath := range derivation.WorkspaceFiles {
		if name == "" {
			return errors.New("derivation.workspaceFiles must not carry an empty tool name")
		}
		if err := validateManifestPath("derivation.workspaceFiles", workspacePath); err != nil {
			return err
		}
	}
	if len(derivation.WorkspaceFiles) > 0 {
		if err := validateManifestPath("derivation.baselineFile", derivation.BaselineFile); err != nil {
			return err
		}
		if len(derivation.TenantOwnedKeys) == 0 {
			return errors.New("derivation.tenantOwnedKeys must declare at least one key when a workspace document is mapped")
		}
	}
	return nil
}

// derivePackageManager reads the package-manager identity from the ecosystem
// package contract through the territory-declared extraction. A missing
// contract document, a missing identity field, an empty declaration, or a
// declaration that violates the declared form is a fail-closed error: the
// fact has exactly one owner surface and it must carry it.
func derivePackageManager(contents []byte, derivation DerivationContract) (PackageIdentity, error) {
	var contract map[string]any
	if err := json.Unmarshal(contents, &contract); err != nil {
		return PackageIdentity{}, fmt.Errorf("the ecosystem package contract %s must contain valid JSON: %w", derivation.ContractFile, err)
	}
	raw, found := contract[derivation.Identity.Field]
	if !found {
		return PackageIdentity{}, fmt.Errorf("the ecosystem package contract %s carries no %q declaration", derivation.ContractFile, derivation.Identity.Field)
	}
	declaration, ok := raw.(string)
	if !ok || declaration == "" {
		return PackageIdentity{}, fmt.Errorf("the ecosystem package contract %s carries a non-string %q declaration", derivation.ContractFile, derivation.Identity.Field)
	}
	switch derivation.Identity.Form {
	case "corepack":
		if !corepackPattern.MatchString(declaration) {
			return PackageIdentity{}, fmt.Errorf("the package manager declaration %q is not a Corepack name@version identity", declaration)
		}
		name, version, _ := strings.Cut(declaration, "@")
		return PackageIdentity{Name: name, Version: version}, nil
	default:
		return PackageIdentity{}, fmt.Errorf("derivation.identity.form %q has no implemented parser", derivation.Identity.Form)
	}
}

// workspaceDocumentPath resolves the conditional workspace document of a
// derived identity through the territory-declared mapping. An empty result
// declares that the tool carries no workspace document and the composition
// proof is skipped; the binding never selects the path.
func workspaceDocumentPath(identity PackageIdentity, mapping map[string]string) string {
	if identity.Name == "" {
		return ""
	}
	return mapping[identity.Name]
}
