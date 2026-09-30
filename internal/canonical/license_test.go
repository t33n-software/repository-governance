package canonical

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// licenseLockFixture is the pinned reference lock of the license-hub family.
const licenseLockFixture = `{"template":"license-hub/templates/custom/norepublish/NoRepublish-1.0.0.hbs","version":"1.0.0","digest":"sha256:3236e146edd71b4b1a27951c929276073af3995daacdc1daab22376c91fd9a37"}`

// licenseToolsModule is the tooling module carrying the admitted hub CLI pin.
const licenseToolsModule = "module example.com/tenant/tools\n\ntool github.com/t33n-software/license-hub/cmd/license\n"

func TestVerifyLicenseNotBound(t *testing.T) {
	verifier := Verifier{
		ReadTenant: func(path string) ([]byte, error) {
			t.Fatalf("no tenant read expected, got %q", path)
			return nil, nil
		},
	}
	if findings := verifier.verifyLicense(context.Background(), Bindings{Class: Class{LicenseHub: false}}); findings != nil {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyLicenseBoundPass(t *testing.T) {
	verifier := Verifier{TenantRoot: "tenant"}
	contents := map[string][]byte{
		"license.values.json": []byte(`{}`),
		"license.lock.json":   []byte(licenseLockFixture),
		"tools/go.mod":        []byte(licenseToolsModule),
	}
	verifier.ReadTenant = func(path string) ([]byte, error) {
		data, found := contents[path]
		if !found {
			return nil, fs.ErrNotExist
		}
		return data, nil
	}
	var resolvedDir, resolvedModule string
	verifier.ResolveModule = func(ctx context.Context, dir, module string) (string, error) {
		resolvedDir, resolvedModule = dir, module
		return "hubdir", nil
	}
	var runDir string
	var runArgs []string
	verifier.RunTool = func(ctx context.Context, dir string, args ...string) (string, error) {
		runDir = dir
		runArgs = append([]string(nil), args...)
		return "", nil
	}

	bindings := Bindings{
		Class: Class{LicenseHub: true},
		Tools: ToolsBinding{Module: "tools/go.mod", CatalogVersion: 1},
	}
	if findings := verifier.verifyLicense(context.Background(), bindings); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}

	if resolvedDir != filepath.Join("tenant", "tools") || resolvedModule != licenseHubModule {
		t.Fatalf("resolved %q %q", resolvedDir, resolvedModule)
	}
	if runDir != "tenant" {
		t.Fatalf("run dir = %q", runDir)
	}
	want := []string{
		"tool", "-modfile", "tools/go.mod", "license", "verify",
		"--template", filepath.Join("hubdir", "templates", "custom", "norepublish", "NoRepublish-1.0.0.hbs"),
		"--org-defaults", filepath.Join("hubdir", "org-defaults.json"),
		"--values", filepath.Join("tenant", "license.values.json"),
		"--lock", filepath.Join("tenant", "license.lock.json"),
		"--dir", "tenant",
	}
	if strings.Join(runArgs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args = %q, want %q", runArgs, want)
	}
}

func TestVerifyLicenseBound(t *testing.T) {
	bindings := Bindings{Class: Class{LicenseHub: true}}
	neverRun := func(t *testing.T) func(ctx context.Context, dir string, args ...string) (string, error) {
		return func(ctx context.Context, dir string, args ...string) (string, error) {
			t.Fatalf("no tool execution expected, got %v", args)
			return "", nil
		}
	}

	t.Run("missing file", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return nil, fs.ErrNotExist },
			RunTool:    neverRun(t),
		}
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 2 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return []byte(`not json`), nil },
			RunTool:    neverRun(t),
		}
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 2 {
			t.Fatalf("findings = %v", findings)
		}
		if !strings.Contains(findings[0].Detail, "valid JSON") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("lock projection rejects a non-object document", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) {
				if path == licenseLockPath {
					return []byte(`[]`), nil
				}
				return []byte(`{}`), nil
			},
			RunTool: neverRun(t),
		}
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "lock projection") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("lock projection requires the pinned fields", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return []byte(`{}`), nil },
			RunTool:    neverRun(t),
		}
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "template, version and digest") {
			t.Fatalf("findings = %v", findings)
		}
	})
}

func TestVerifyLicenseReadErrorPropagates(t *testing.T) {
	boom := errors.New("boom")
	verifier := Verifier{
		ReadTenant: func(path string) ([]byte, error) { return nil, boom },
	}
	findings := verifier.verifyLicense(context.Background(), Bindings{Class: Class{LicenseHub: true}})
	if len(findings) != 2 || !strings.Contains(findings[0].Detail, "boom") {
		t.Fatalf("findings = %v", findings)
	}
}

// licenseContentVerifier binds the seams whose wiring prechecks pass, so the
// content-orchestration branches decide the finding.
func licenseContentVerifier(t *testing.T, toolsModule []byte) Verifier {
	t.Helper()
	contents := map[string][]byte{
		"license.values.json": []byte(`{}`),
		"license.lock.json":   []byte(licenseLockFixture),
	}
	if toolsModule != nil {
		contents["tools/go.mod"] = toolsModule
	}
	return Verifier{
		TenantRoot: "tenant",
		ReadTenant: func(path string) ([]byte, error) {
			data, found := contents[path]
			if !found {
				return nil, fs.ErrNotExist
			}
			return data, nil
		},
	}
}

func TestVerifyLicenseContentPreconditions(t *testing.T) {
	bindings := Bindings{
		Class: Class{LicenseHub: true},
		Tools: ToolsBinding{Module: "tools/go.mod", CatalogVersion: 1},
	}

	t.Run("template without the hub identity prefix", func(t *testing.T) {
		verifier := licenseContentVerifier(t, []byte(licenseToolsModule))
		verifier.ReadTenant = func(path string) ([]byte, error) {
			if path == licenseLockPath {
				return []byte(`{"template":"other/templates/x.hbs","version":"1.0.0","digest":"sha256:abc"}`), nil
			}
			return []byte(`{}`), nil
		}
		verifier.RunTool = func(ctx context.Context, dir string, args ...string) (string, error) {
			t.Fatalf("no tool execution expected, got %v", args)
			return "", nil
		}
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "identity prefix") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("no tools module", func(t *testing.T) {
		verifier := licenseContentVerifier(t, nil)
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "no tools module is present") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("tools module read error", func(t *testing.T) {
		boom := errors.New("boom")
		verifier := licenseContentVerifier(t, nil)
		verifier.ReadTenant = func(path string) ([]byte, error) {
			if path == "tools/go.mod" {
				return nil, boom
			}
			if path == licenseLockPath {
				return []byte(licenseLockFixture), nil
			}
			return []byte(`{}`), nil
		}
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "boom") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("tools module with an unterminated tool block", func(t *testing.T) {
		verifier := licenseContentVerifier(t, []byte("module example.com/tenant/tools\n\ntool (\n"))
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "unterminated tool block") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("hub CLI pin missing", func(t *testing.T) {
		verifier := licenseContentVerifier(t, []byte("module example.com/tenant/tools\n\ntool example.com/other/tool\n"))
		verifier.RunTool = func(ctx context.Context, dir string, args ...string) (string, error) {
			t.Fatalf("no tool execution expected, got %v", args)
			return "", nil
		}
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, licenseHubToolPackage) {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("module resolution failure", func(t *testing.T) {
		verifier := licenseContentVerifier(t, []byte(licenseToolsModule))
		verifier.ResolveModule = func(ctx context.Context, dir, module string) (string, error) {
			return "", errors.New("network unreachable")
		}
		findings := verifier.verifyLicense(context.Background(), bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "network unreachable") {
			t.Fatalf("findings = %v", findings)
		}
	})
}

func TestVerifyLicenseContentProofFailure(t *testing.T) {
	bindings := Bindings{
		Class: Class{LicenseHub: true},
		Tools: ToolsBinding{Module: "tools/go.mod", CatalogVersion: 1},
	}
	cases := map[string]string{
		"drift":            "violation: rendered file drifted from canonical render: tenant/LICENSE",
		"digest break":     "violation: template digest does not match pinned lock digest: tenant/license.lock.json",
		"placeholder rest": "violation: unresolved placeholders: PROJECT_NAME",
		"silent failure":   "",
	}
	for name, output := range cases {
		t.Run(name, func(t *testing.T) {
			verifier := licenseContentVerifier(t, []byte(licenseToolsModule))
			verifier.ResolveModule = func(ctx context.Context, dir, module string) (string, error) {
				return "hubdir", nil
			}
			verifier.RunTool = func(ctx context.Context, dir string, args ...string) (string, error) {
				return output, errors.New("exit status 1")
			}
			findings := verifier.verifyLicense(context.Background(), bindings)
			if len(findings) != 1 || findings[0].Check != "license" {
				t.Fatalf("findings = %v", findings)
			}
			if !strings.Contains(findings[0].Detail, "exit status 1") {
				t.Fatalf("findings = %v", findings)
			}
			if output != "" && !strings.Contains(findings[0].Detail, output) {
				t.Fatalf("the violation output must surface: %v", findings)
			}
		})
	}
}

func TestDecodeJSONDocument(t *testing.T) {
	if err := decodeJSONDocument([]byte(`{"a":1}`)); err != nil {
		t.Fatalf("decodeJSONDocument: %v", err)
	}
	if err := decodeJSONDocument([]byte(`[1,2]`)); err != nil {
		t.Fatalf("decodeJSONDocument array: %v", err)
	}
	if err := decodeJSONDocument(nil); err == nil {
		t.Fatal("expected the empty rejection")
	}
	if err := decodeJSONDocument([]byte(`not json`)); err == nil {
		t.Fatal("expected the invalid rejection")
	}
	if err := decodeJSONDocument([]byte(`{} {}`)); err == nil {
		t.Fatal("expected the trailing document rejection")
	}
}
