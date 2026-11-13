package canonical

import (
	"errors"
	"strings"
	"testing"
)

// validDerivationFixture is the conforming territory derivation declaration
// every mutation test starts from.
func validDerivationFixture() DerivationContract {
	return DerivationContract{
		ContractFile:    "package.json",
		Format:          "json",
		Identity:        IdentityExtraction{Field: "packageManager", Form: "corepack"},
		WorkspaceFiles:  map[string]string{"pnpm": "pnpm-workspace.yaml"},
		BaselineFile:    "configs/pnpm/pnpm-workspace.base.yaml",
		TenantOwnedKeys: []string{"catalog"},
	}
}

func TestValidateDerivation(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		if err := validateDerivation(validDerivationFixture()); err != nil {
			t.Fatalf("validateDerivation: %v", err)
		}
	})
	t.Run("empty contract file", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.ContractFile = ""
		if err := validateDerivation(derivation); err == nil || !strings.Contains(err.Error(), "must not be empty") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("traversal contract file", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.ContractFile = "../package.json"
		if err := validateDerivation(derivation); err == nil || !strings.Contains(err.Error(), "parent traversal") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unsupported format", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.Format = "toml"
		if err := validateDerivation(derivation); err == nil || !strings.Contains(err.Error(), "must be json") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("empty identity field", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.Identity.Field = ""
		if err := validateDerivation(derivation); err == nil || !strings.Contains(err.Error(), "identity.field must not be empty") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unsupported identity form", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.Identity.Form = "directive"
		if err := validateDerivation(derivation); err == nil || !strings.Contains(err.Error(), "must be corepack") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("empty tool name", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.WorkspaceFiles = map[string]string{"": "workspace.yaml"}
		if err := validateDerivation(derivation); err == nil || !strings.Contains(err.Error(), "empty tool name") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("bad workspace path", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.WorkspaceFiles = map[string]string{"pnpm": "../pnpm-workspace.yaml"}
		if err := validateDerivation(derivation); err == nil || !strings.Contains(err.Error(), "parent traversal") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("missing baseline file", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.BaselineFile = ""
		if err := validateDerivation(derivation); err == nil || !strings.Contains(err.Error(), "baselineFile must not be empty") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("missing tenant owned keys", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.TenantOwnedKeys = nil
		if err := validateDerivation(derivation); err == nil || !strings.Contains(err.Error(), "at least one key") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("no workspace mapping needs no baseline", func(t *testing.T) {
		derivation := validDerivationFixture()
		derivation.WorkspaceFiles = nil
		derivation.BaselineFile = ""
		derivation.TenantOwnedKeys = nil
		if err := validateDerivation(derivation); err != nil {
			t.Fatalf("validateDerivation without workspace documents: %v", err)
		}
	})
}

func TestDerivePackageManager(t *testing.T) {
	derivation := validDerivationFixture()
	t.Run("pnpm identity", func(t *testing.T) {
		identity, err := derivePackageManager([]byte("{\"packageManager\": \"pnpm@12.17.0\"}"), derivation)
		if err != nil {
			t.Fatalf("derivePackageManager: %v", err)
		}
		if identity.Name != "pnpm" || identity.Version != "12.17.0" {
			t.Fatalf("identity = %+v", identity)
		}
	})
	t.Run("npm identity", func(t *testing.T) {
		identity, err := derivePackageManager([]byte("{\"packageManager\": \"npm@11.20.0\"}"), derivation)
		if err != nil || identity.Name != "npm" || identity.Version != "11.20.0" {
			t.Fatalf("identity = %+v, err = %v", identity, err)
		}
	})
	t.Run("invalid json", func(t *testing.T) {
		if _, err := derivePackageManager([]byte("not json"), derivation); err == nil || !strings.Contains(err.Error(), "valid JSON") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("missing declaration", func(t *testing.T) {
		if _, err := derivePackageManager([]byte("{}"), derivation); err == nil || !strings.Contains(err.Error(), "carries no \"packageManager\" declaration") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("empty declaration", func(t *testing.T) {
		if _, err := derivePackageManager([]byte("{\"packageManager\": \"\"}"), derivation); err == nil || !strings.Contains(err.Error(), "non-string") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("non-string declaration", func(t *testing.T) {
		if _, err := derivePackageManager([]byte("{\"packageManager\": 12}"), derivation); err == nil || !strings.Contains(err.Error(), "non-string") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("malformed declaration", func(t *testing.T) {
		if _, err := derivePackageManager([]byte("{\"packageManager\": \"pnpm@11\"}"), derivation); err == nil || !strings.Contains(err.Error(), "Corepack name@version") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unimplemented form", func(t *testing.T) {
		unimplemented := derivation
		unimplemented.Identity.Form = "directive"
		if _, err := derivePackageManager([]byte("{\"packageManager\": \"pnpm@12.17.0\"}"), unimplemented); err == nil || !strings.Contains(err.Error(), "no implemented parser") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestArtifactFamiliesAndDistributionClasses(t *testing.T) {
	byteIdentity, composition := ArtifactFamilies()
	if strings.Join(byteIdentity, ",") != "eslint,tsconfig,tsdown,vitest" {
		t.Fatalf("byte identity families = %v", byteIdentity)
	}
	if strings.Join(composition, ",") != "vscodeignore" {
		t.Fatalf("composition families = %v", composition)
	}
	if got := strings.Join(DistributionClasses(), ","); got != "cli,container-image,library,reference-hub,vscode-extension" {
		t.Fatalf("distribution classes = %q", got)
	}
}

func TestWorkspaceDocumentPath(t *testing.T) {
	mapping := map[string]string{"pnpm": "pnpm-workspace.yaml"}
	if got := workspaceDocumentPath(PackageIdentity{}, mapping); got != "" {
		t.Fatalf("an empty identity must derive no path, got %q", got)
	}
	if got := workspaceDocumentPath(PackageIdentity{Name: "pnpm"}, mapping); got != "pnpm-workspace.yaml" {
		t.Fatalf("pnpm must derive the mapped document, got %q", got)
	}
	if got := workspaceDocumentPath(PackageIdentity{Name: "npm"}, mapping); got != "" {
		t.Fatalf("a tool without a mapping must derive no path, got %q", got)
	}
}

func TestPinnedCategoryRegistryValidatesDerivation(t *testing.T) {
	read := func(path string) ([]byte, error) {
		if path != "configs/registry.json" {
			return nil, errors.New("no such territory file: " + path)
		}
		return []byte(`{"schemaVersion":1,"derivation":{"contractFile":"package.json","format":"toml","identity":{"field":"packageManager","form":"corepack"}},"categories":[{"id":"c","artifacts":{}}]}`), nil
	}
	pin := RegistryPin{Path: "configs/registry.json", SHA256: Sum256Hex([]byte(`{"schemaVersion":1,"derivation":{"contractFile":"package.json","format":"toml","identity":{"field":"packageManager","form":"corepack"}},"categories":[{"id":"c","artifacts":{}}]}`))}
	if _, err := pinnedCategoryRegistry(read, pin); err == nil || !strings.Contains(err.Error(), "derivation block is invalid") {
		t.Fatalf("err = %v", err)
	}
}
