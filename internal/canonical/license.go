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

// The canonical file names the license-hub family wires into a tenant.
const (
	licenseValuesPath = "license.values.json"
	licenseLockPath   = "license.lock.json"
)

// licenseBindings are the files the license-hub family wires into a tenant.
var licenseBindings = []string{licenseValuesPath, licenseLockPath}

// licenseHubModule is the module path of the license hub that owns the
// canonical license semantics; the verifier resolves it through the tenant's
// integrity-pinned tooling channel and never trusts a warm cache.
const licenseHubModule = "github.com/t33n-software/license-hub"

// licenseHubToolPackage is the hub CLI package a tenant pins through its
// tooling module when the license-hub family is bound; the package is
// admitted by the canonical tool catalog of the language territory home.
const licenseHubToolPackage = licenseHubModule + "/cmd/license"

// licenseHubTemplatePrefix is the repository-name identity prefix every
// locked template path carries; the verifier binds the prefix and resolves
// the remainder inside the pinned module directory.
const licenseHubTemplatePrefix = "license-hub/"

// licenseLockProjection is the wiring projection of the tenant's license
// lock: the template identity the verifier needs to orchestrate the proof.
// The lock schema is owned by the license hub; the proof itself — pin
// integrity, drift freedom, and completeness — is executed by the hub CLI
// and is never re-implemented here.
type licenseLockProjection struct {
	Template string `json:"template"`
	Version  string `json:"version"`
	Digest   string `json:"digest"`
}

// verifyLicense proves the license-hub family where bound: the binding
// values and lock exist and decode, and the tenant-pinned hub CLI proves the
// committed instance against the canonical render fail-closed inside this
// check. The verification semantics live exactly once in the hub; the
// verifier orchestrates the pinned tool and never re-implements the proof.
func (v Verifier) verifyLicense(ctx context.Context, bindings Bindings) []Finding {
	if !bindings.Class.LicenseHub {
		return nil
	}
	findings := make([]Finding, 0)
	var lock licenseLockProjection
	for _, path := range licenseBindings {
		contents, err := v.ReadTenant(path)
		if err != nil {
			findings = append(findings, readErrorFinding("license", path, err))
			continue
		}
		if err := decodeJSONDocument(contents); err != nil {
			findings = append(findings, mismatchFinding("license",
				path+" must contain a valid JSON document"))
			continue
		}
		if path == licenseLockPath {
			projection, err := decodeLicenseLock(contents)
			if err != nil {
				findings = append(findings, mismatchFinding("license", err.Error()))
				continue
			}
			lock = projection
		}
	}
	if len(findings) > 0 {
		return findings
	}
	return v.verifyLicenseContent(ctx, bindings, lock)
}

// verifyLicenseContent orchestrates the tenant-pinned hub CLI: the locked
// template is resolved inside the pinned license-hub module directory, and
// the pinned tool proves pin integrity, drift freedom, and completeness of
// the committed instance. Every precondition failure is a precise
// fail-closed finding before any process runs; a failing proof surfaces the
// tool's own violation output.
func (v Verifier) verifyLicenseContent(ctx context.Context, bindings Bindings, lock licenseLockProjection) []Finding {
	check := "license"
	template, found := strings.CutPrefix(lock.Template, licenseHubTemplatePrefix)
	if !found || template == "" {
		return []Finding{mismatchFinding(check, fmt.Sprintf(
			"the locked template %q must carry the %q identity prefix", lock.Template, licenseHubTemplatePrefix))}
	}

	toolsContents, err := v.ReadTenant(bindings.Tools.Module)
	if errors.Is(err, fs.ErrNotExist) {
		return []Finding{mismatchFinding(check, fmt.Sprintf(
			"the license content proof requires the tenant's integrity-pinned tooling module (%s): no tools module is present",
			bindings.Tools.Module))}
	}
	if err != nil {
		return []Finding{readErrorFinding(check, bindings.Tools.Module, err)}
	}
	directives, err := ToolDirectives(toolsContents)
	if err != nil {
		return []Finding{mismatchFinding(check, err.Error())}
	}
	pinned := false
	for _, directive := range directives {
		if directive == licenseHubToolPackage {
			pinned = true
			break
		}
	}
	if !pinned {
		return []Finding{mismatchFinding(check, fmt.Sprintf(
			"the license-hub family requires the tool pin %s in the tooling module", licenseHubToolPackage))}
	}

	toolsDir := filepath.Join(v.TenantRoot, path.Dir(bindings.Tools.Module))
	moduleDir, err := v.ResolveModule(ctx, toolsDir, licenseHubModule)
	if err != nil {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("resolve the license-hub module: %v", err))}
	}

	output, err := v.RunTool(ctx, v.TenantRoot,
		"tool", "-modfile", bindings.Tools.Module, "license", "verify",
		"--template", filepath.Join(moduleDir, filepath.FromSlash(template)),
		"--org-defaults", filepath.Join(moduleDir, "org-defaults.json"),
		"--values", filepath.Join(v.TenantRoot, licenseValuesPath),
		"--lock", filepath.Join(v.TenantRoot, licenseLockPath),
		"--dir", v.TenantRoot)
	if err != nil {
		detail := fmt.Sprintf("the tenant-pinned license CLI failed: %v", err)
		if trimmed := strings.TrimSpace(output); trimmed != "" {
			detail += " — " + trimmed
		}
		return []Finding{mismatchFinding(check, detail)}
	}
	return nil
}

// decodeLicenseLock decodes the wiring projection of a tenant lock file; the
// document shape was already proven by the wiring check, so unknown fields
// evolve with the hub-owned schema without breaking this proof.
func decodeLicenseLock(contents []byte) (licenseLockProjection, error) {
	var lock licenseLockProjection
	if err := json.Unmarshal(contents, &lock); err != nil {
		return licenseLockProjection{}, fmt.Errorf("license.lock.json must contain the lock projection: %w", err)
	}
	if lock.Template == "" || lock.Version == "" || lock.Digest == "" {
		return licenseLockProjection{}, errors.New("license.lock.json requires template, version and digest")
	}
	return lock, nil
}

// decodeJSONDocument proves the contents hold exactly one valid JSON value.
func decodeJSONDocument(contents []byte) error {
	if len(contents) == 0 {
		return errors.New("empty document")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	var document any
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("more than one JSON document")
	}
	return nil
}
