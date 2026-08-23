package canonical

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
)

// ToolCatalog is the canonical admitted Go tool set owned by the language
// territory home.
type ToolCatalog struct {
	SchemaVersion int
	// Schema is the catalog's bound schema identity (the $schema reference),
	// asserted against the canonical identity and never dereferenced.
	Schema   string
	Packages []string
}

// toolCatalogDocument is the wire form of the canonical tool catalog.
type toolCatalogDocument struct {
	Schema        string `json:"$schema"`
	SchemaVersion int   `json:"schemaVersion"`
	Tools         []struct {
		Name    string `json:"name"`
		Module  string `json:"module"`
		Package string `json:"package"`
		Purpose string `json:"purpose"`
	} `json:"tools"`
}

// DecodeToolCatalog strictly decodes the canonical tool catalog.
func DecodeToolCatalog(contents []byte) (ToolCatalog, error) {
	if len(contents) == 0 {
		return ToolCatalog{}, fmt.Errorf("tool catalog must not be empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var document toolCatalogDocument
	if err := decoder.Decode(&document); err != nil {
		return ToolCatalog{}, fmt.Errorf("tool catalog must contain valid JSON with known fields: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ToolCatalog{}, fmt.Errorf("tool catalog must contain exactly one JSON document")
	}
	if document.SchemaVersion != 1 {
		return ToolCatalog{}, fmt.Errorf("tool catalog schemaVersion must equal %d", 1)
	}
	if strings.TrimSpace(document.Schema) == "" {
		return ToolCatalog{}, errors.New("tool catalog must carry the $schema identity")
	}
	packages := make([]string, 0, len(document.Tools))
	seen := make(map[string]struct{}, len(document.Tools))
	for _, tool := range document.Tools {
		if tool.Name == "" || tool.Module == "" || tool.Package == "" || tool.Purpose == "" {
			return ToolCatalog{}, fmt.Errorf("every tool catalog entry must be complete: %+v", tool)
		}
		if _, found := seen[tool.Package]; found {
			return ToolCatalog{}, fmt.Errorf("tool catalog package must be unique: %q", tool.Package)
		}
		seen[tool.Package] = struct{}{}
		packages = append(packages, tool.Package)
	}
	return ToolCatalog{SchemaVersion: document.SchemaVersion, Schema: document.Schema, Packages: packages}, nil
}

// ToolDirectives parses the tool directives of a Go tooling module
// (tools/go.mod). Both the block form and the single-line form are
// recognized; comments and blank lines are ignored.
func ToolDirectives(contents []byte) ([]string, error) {
	tools := make([]string, 0, 8)
	inBlock := false
	for _, raw := range strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if index := strings.Index(line, "//"); index >= 0 {
			line = strings.TrimSpace(line[:index])
		}
		if line == "" {
			continue
		}
		if inBlock {
			if line == ")" {
				inBlock = false
				continue
			}
			tools = append(tools, line)
			continue
		}
		if line == "tool (" {
			inBlock = true
			continue
		}
		if rest, found := strings.CutPrefix(line, "tool "); found {
			tools = append(tools, rest)
			continue
		}
	}
	if inBlock {
		return nil, fmt.Errorf("tools module contains an unterminated tool block")
	}
	return tools, nil
}

// UnadmittedTools returns the tool-directive packages that the canonical
// catalog and the home's own tool do not admit.
func UnadmittedTools(tools []string, catalog ToolCatalog, homeToolPackage string) []string {
	admitted := make(map[string]struct{}, len(catalog.Packages)+1)
	for _, pkg := range catalog.Packages {
		admitted[pkg] = struct{}{}
	}
	if homeToolPackage != "" {
		admitted[homeToolPackage] = struct{}{}
	}
	unadmitted := make([]string, 0)
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if _, duplicate := seen[tool]; duplicate {
			continue
		}
		seen[tool] = struct{}{}
		if _, ok := admitted[tool]; !ok {
			unadmitted = append(unadmitted, tool)
		}
	}
	return unadmitted
}

// qualityAuthorityModule is the module path of the language territory home
// that owns the canonical tool catalog.
const qualityAuthorityModule = "github.com/t33n-software/go-quality-authority"

// canonicalCatalogSchemaID is the canonical identity of the tool catalog
// schema, owned by the language territory home. The verifier asserts the
// catalog's $schema reference against it fail-closed; the reference is an
// identity binding and is never dereferenced by any governed path.
const canonicalCatalogSchemaID = "https://raw.githubusercontent.com/t33n-software/go-quality-authority/main/catalog/tools.schema.json"

// verifyTools proves every tool pin of the tenant's tooling module is admitted
// by the canonical catalog or is the home's own verifier tool. A tenant
// without a tooling module carries no pins to admit — the check is vacuously
// satisfied; the presence of the gate's tooling is the quality lane's
// contract, not this proof's.
func (v Verifier) verifyTools(ctx context.Context, bindings Bindings) []Finding {
	check := "tool pins"
	contents, err := v.ReadTenant(bindings.Tools.Module)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return []Finding{readErrorFinding(check, bindings.Tools.Module, err)}
	}
	directives, err := ToolDirectives(contents)
	if err != nil {
		return []Finding{mismatchFinding(check, err.Error())}
	}
	if len(directives) == 0 {
		return nil
	}

	toolsDir := path.Dir(bindings.Tools.Module)
	catalogDir, err := v.ResolveModule(ctx, filepath.Join(v.TenantRoot, toolsDir), qualityAuthorityModule)
	if err != nil {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("resolve the canonical catalog module: %v", err))}
	}
	catalogContents, err := v.ReadModule(catalogDir, "catalog/tools.json")
	if err != nil {
		return []Finding{readErrorFinding(check, "catalog/tools.json", err)}
	}
	catalog, err := DecodeToolCatalog(catalogContents)
	if err != nil {
		return []Finding{mismatchFinding(check, err.Error())}
	}
	if catalog.Schema != canonicalCatalogSchemaID {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the tool catalog schema identity %q diverges from the canonical %q", catalog.Schema, canonicalCatalogSchemaID))}
	}

	homeTool, err := v.homeToolPackage()
	if err != nil {
		return []Finding{mismatchFinding(check, err.Error())}
	}
	findings := make([]Finding, 0)
	for _, tool := range UnadmittedTools(directives, catalog, homeTool) {
		findings = append(findings, mismatchFinding(check,
			fmt.Sprintf("the tool pin %q is not admitted by the canonical catalog", tool)))
	}
	return findings
}

// homeToolPackage derives the home's own verifier tool package from the home
// module declaration.
func (v Verifier) homeToolPackage() (string, error) {
	contents, err := v.ReadHome("go.mod")
	if err != nil {
		return "", fmt.Errorf("read the home module declaration: %w", err)
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n") {
		if module, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return module + "/cmd/verify-canonical", nil
		}
	}
	return "", fmt.Errorf("the home module declaration carries no module line")
}
