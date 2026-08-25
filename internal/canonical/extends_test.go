package canonical

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
)

// testPackDescriptor is a minimal descriptor carrying the identity binding the
// verifier proves; the full descriptor schema is owned by the shared kernel
// and is never redefined here.
func testPackDescriptor(capability, area string, version int) string {
	return fmt.Sprintf(`{"schema":"capability-pack/v1","capability":%q,"area":%q,"version":%d,"summary":"test pack"}`,
		capability, area, version)
}

// fakeRegistry is a virtual capability-pack registry tree: the area names of
// the capabilities root plus the descriptor contents by registry-relative
// path.
type fakeRegistry struct {
	areas   []string
	files   map[string][]byte
	listErr error
}

// extendsFixture binds the verifier seams for the extends-resolution tests.
type extendsFixture struct {
	config    string
	configErr error
	toolsMod  string
	toolsErr  error
	goMod     string
	gqa       *fakeRegistry
	gqaErr    error
	scg       *fakeRegistry
	scgErr    error
	tenant    *fakeRegistry

	resolvedModules []string
}

func (fixture *extendsFixture) registryFor(dir string) *fakeRegistry {
	switch dir {
	case "gqa":
		return fixture.gqa
	case "scg":
		return fixture.scg
	default:
		return nil
	}
}

func (fixture *extendsFixture) verifier() Verifier {
	return Verifier{
		TenantRoot: "tenant",
		ReadTenant: func(path string) ([]byte, error) {
			switch path {
			case "git-governance.quality.json":
				if fixture.configErr != nil {
					return nil, fixture.configErr
				}
				return []byte(fixture.config), nil
			case "tools/go.mod":
				if fixture.toolsErr != nil {
					return nil, fixture.toolsErr
				}
				return []byte(fixture.toolsMod), nil
			case "go.mod":
				if fixture.goMod == "" {
					return nil, fs.ErrNotExist
				}
				return []byte(fixture.goMod), nil
			default:
				if fixture.tenant != nil {
					if contents, found := fixture.tenant.files[strings.TrimPrefix(path, "capabilities/")]; found {
						return contents, nil
					}
				}
				return nil, fs.ErrNotExist
			}
		},
		ListTenant: func(path string) ([]string, error) {
			if fixture.tenant != nil && path == "capabilities" {
				return fixture.tenant.areas, fixture.tenant.listErr
			}
			return nil, fs.ErrNotExist
		},
		ResolveModule: func(ctx context.Context, dir, module string) (string, error) {
			fixture.resolvedModules = append(fixture.resolvedModules, module)
			switch module {
			case qualityAuthorityModule:
				if fixture.gqaErr != nil {
					return "", fixture.gqaErr
				}
				return "gqa", nil
			case sharedKernelModule:
				if fixture.scgErr != nil {
					return "", fixture.scgErr
				}
				return "scg", nil
			default:
				return "", errors.New("unexpected module " + module)
			}
		},
		ReadModule: func(dir, path string) ([]byte, error) {
			registry := fixture.registryFor(dir)
			if registry == nil {
				return nil, fs.ErrNotExist
			}
			contents, found := registry.files[strings.TrimPrefix(path, "capabilities/")]
			if !found {
				return nil, fs.ErrNotExist
			}
			return contents, nil
		},
		ListModule: func(dir, path string) ([]string, error) {
			registry := fixture.registryFor(dir)
			if registry == nil {
				return nil, fs.ErrNotExist
			}
			return registry.areas, registry.listErr
		},
	}
}

func extendsConfig(extends string) string {
	return `{"schemaVersion":4,"toolchain":{"language":"go","version":"1.26.6"},"extends":[` + extends + `],"gates":[{"name":"a","command":"go"}]}`
}

func extendsBindings() Bindings {
	return Bindings{
		Quality: QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4},
		Tools:   ToolsBinding{Module: "tools/go.mod", CatalogVersion: 1},
	}
}

// sharedKernelPack binds a fixture whose shared kernel registry carries the
// opentofu pack and whose territory registry carries no packs.
func sharedKernelPack() *extendsFixture {
	return &extendsFixture{
		config:   extendsConfig(`"opentofu@1"`),
		toolsMod: "module example.com/tenant/tools\n",
		goMod:    "module example.com/tenant\n",
		gqa:      &fakeRegistry{},
		scg: &fakeRegistry{
			areas: []string{"infrastructure"},
			files: map[string][]byte{
				"infrastructure/opentofu/v1/pack.json": []byte(testPackDescriptor("opentofu", "infrastructure", 1)),
			},
		},
	}
}

func TestParsePackReference(t *testing.T) {
	reference, err := parsePackReference("opentofu@1")
	if err != nil {
		t.Fatalf("parsePackReference: %v", err)
	}
	if reference.capability != "opentofu" || reference.major != 1 || reference.raw != "opentofu@1" {
		t.Fatalf("reference = %+v", reference)
	}
}

func TestParsePackReferenceRejections(t *testing.T) {
	for _, reference := range []string{
		"opentofu",
		"opentofu@",
		"@1",
		"opentofu@latest",
		"opentofu@1.2",
		"OpenTofu@1",
		"opentofu @1",
		"opentofu@x",
	} {
		t.Run(reference, func(t *testing.T) {
			if _, err := parsePackReference(reference); err == nil {
				t.Fatalf("expected the rejection of %q", reference)
			}
		})
	}
}

func TestDecodePackIdentity(t *testing.T) {
	identity, err := decodePackIdentity([]byte(testPackDescriptor("opentofu", "infrastructure", 1)))
	if err != nil {
		t.Fatalf("decodePackIdentity: %v", err)
	}
	if identity.Schema != packSchemaID || identity.Capability != "opentofu" || identity.Area != "infrastructure" || identity.Version != 1 {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestDecodePackIdentityRejections(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{name: "empty", doc: ``},
		{name: "not json", doc: `not json`},
		{name: "wrong schema", doc: `{"schema":"capability-pack/v2","capability":"opentofu","area":"infrastructure","version":1}`},
		{name: "missing schema", doc: `{"capability":"opentofu","area":"infrastructure","version":1}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodePackIdentity([]byte(test.doc)); err == nil {
				t.Fatal("expected a rejection")
			}
		})
	}
}

func TestVerifyExtendsConfigReadAndDecodeSkip(t *testing.T) {
	bindings := extendsBindings()

	t.Run("read error", func(t *testing.T) {
		fixture := &extendsFixture{configErr: errors.New("boom")}
		if findings := fixture.verifier().verifyExtends(context.Background(), bindings); findings != nil {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		fixture := &extendsFixture{config: "not json"}
		if findings := fixture.verifier().verifyExtends(context.Background(), bindings); findings != nil {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("no extends declaration", func(t *testing.T) {
		fixture := &extendsFixture{config: `{"schemaVersion":4,"toolchain":{"language":"go","version":"1.26.6"},"gates":[{"name":"a","command":"go"}]}`}
		if findings := fixture.verifier().verifyExtends(context.Background(), bindings); findings != nil {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("empty extends", func(t *testing.T) {
		fixture := &extendsFixture{config: extendsConfig("")}
		if findings := fixture.verifier().verifyExtends(context.Background(), bindings); findings != nil {
			t.Fatalf("findings = %v", findings)
		}
	})
}

func TestVerifyExtendsMalformedReference(t *testing.T) {
	fixture := &extendsFixture{
		config:   extendsConfig(`"opentofu"`),
		toolsMod: "module example.com/tenant/tools\n",
	}
	findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "not a pinned <capability>@<major> reference") {
		t.Fatalf("findings = %v", findings)
	}
	if len(fixture.resolvedModules) != 0 {
		t.Fatalf("no registry resolution must run for a malformed reference: %v", fixture.resolvedModules)
	}
}

func TestVerifyExtendsMalformedAndValidReference(t *testing.T) {
	fixture := sharedKernelPack()
	fixture.config = extendsConfig(`"bogus", "opentofu@1"`)
	findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, `"bogus"`) {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyExtendsWithoutToolsModule(t *testing.T) {
	fixture := sharedKernelPack()
	fixture.toolsErr = fs.ErrNotExist
	findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "no tools module is present") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyExtendsToolsModuleReadError(t *testing.T) {
	fixture := sharedKernelPack()
	fixture.toolsErr = errors.New("boom")
	findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "boom") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyExtendsResolvesTheSharedKernelRegistry(t *testing.T) {
	fixture := sharedKernelPack()
	if findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings()); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
	if strings.Join(fixture.resolvedModules, ",") != qualityAuthorityModule+","+sharedKernelModule {
		t.Fatalf("resolved modules = %v", fixture.resolvedModules)
	}
}

func TestVerifyExtendsRegistryWithoutCapabilitiesTree(t *testing.T) {
	fixture := sharedKernelPack()
	fixture.gqa = nil // the territory registry resolves but carries no capabilities tree
	if findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings()); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyExtendsResolvesTheTerritoryRegistry(t *testing.T) {
	fixture := sharedKernelPack()
	fixture.config = extendsConfig(`"gofumpt@1"`)
	fixture.gqa = &fakeRegistry{
		areas: []string{"formatting"},
		files: map[string][]byte{
			"formatting/gofumpt/v1/pack.json": []byte(testPackDescriptor("gofumpt", "formatting", 1)),
		},
	}
	if findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings()); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyExtendsUnknownReference(t *testing.T) {
	fixture := sharedKernelPack()
	fixture.config = extendsConfig(`"terraform@1"`)
	findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings())
	if len(findings) != 1 {
		t.Fatalf("findings = %v", findings)
	}
	detail := findings[0].Detail
	if !strings.Contains(detail, "unknown to the capability-pack registries") ||
		!strings.Contains(detail, qualityAuthorityModule) ||
		!strings.Contains(detail, sharedKernelModule) {
		t.Fatalf("detail = %q", detail)
	}
}

func TestVerifyExtendsUnknownReferenceNamesTheUnavailableRegistry(t *testing.T) {
	fixture := sharedKernelPack()
	fixture.config = extendsConfig(`"opentofu@1"`)
	fixture.scgErr = errors.New("not a known dependency")
	findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings())
	if len(findings) != 1 {
		t.Fatalf("findings = %v", findings)
	}
	if !strings.Contains(findings[0].Detail, "unavailable") || !strings.Contains(findings[0].Detail, "not a known dependency") {
		t.Fatalf("detail = %q", findings[0].Detail)
	}
}

func TestVerifyExtendsAmbiguousReference(t *testing.T) {
	fixture := sharedKernelPack()
	fixture.gqa = &fakeRegistry{
		areas: []string{"infrastructure"},
		files: map[string][]byte{
			"infrastructure/opentofu/v1/pack.json": []byte(testPackDescriptor("opentofu", "infrastructure", 1)),
		},
	}
	findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "ambiguous") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyExtendsRegistryIntegrityFailures(t *testing.T) {
	tests := []struct {
		name   string
		scg    *fakeRegistry
		detail string
	}{
		{
			name: "invalid descriptor json",
			scg: &fakeRegistry{
				areas: []string{"infrastructure"},
				files: map[string][]byte{"infrastructure/opentofu/v1/pack.json": []byte("not json")},
			},
			detail: "is invalid",
		},
		{
			name: "wrong descriptor schema",
			scg: &fakeRegistry{
				areas: []string{"infrastructure"},
				files: map[string][]byte{"infrastructure/opentofu/v1/pack.json": []byte(`{"schema":"other/v9","capability":"opentofu","area":"infrastructure","version":1}`)},
			},
			detail: "is invalid",
		},
		{
			name: "identity mismatch",
			scg: &fakeRegistry{
				areas: []string{"infrastructure"},
				files: map[string][]byte{"infrastructure/opentofu/v1/pack.json": []byte(testPackDescriptor("opentofu", "platform", 1))},
			},
			detail: "not the infrastructure/opentofu v1 of its registry location",
		},
		{
			name: "registry list error",
			scg: &fakeRegistry{
				listErr: errors.New("boom"),
			},
			detail: "list the registry",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := sharedKernelPack()
			fixture.scg = test.scg
			findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings())
			if len(findings) != 1 || !strings.Contains(findings[0].Detail, test.detail) {
				t.Fatalf("findings = %v", findings)
			}
		})
	}
}

func TestVerifyExtendsDescriptorReadError(t *testing.T) {
	fixture := sharedKernelPack()
	verifier := fixture.verifier()
	verifier.ReadModule = func(dir, path string) ([]byte, error) {
		if dir == "scg" && strings.HasSuffix(path, "pack.json") {
			return nil, errors.New("boom")
		}
		return nil, fs.ErrNotExist
	}
	findings := verifier.verifyExtends(context.Background(), extendsBindings())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "read the pack descriptor") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyExtendsHomeWorkingTreeRegistries(t *testing.T) {
	t.Run("tenant is the territory home", func(t *testing.T) {
		fixture := sharedKernelPack()
		fixture.config = extendsConfig(`"gofumpt@1"`)
		fixture.goMod = "module " + qualityAuthorityModule + "\n"
		fixture.tenant = &fakeRegistry{
			areas: []string{"formatting"},
			files: map[string][]byte{
				"formatting/gofumpt/v1/pack.json": []byte(testPackDescriptor("gofumpt", "formatting", 1)),
			},
		}
		if findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings()); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
		for _, module := range fixture.resolvedModules {
			if module == qualityAuthorityModule {
				t.Fatalf("the home registry must resolve from the working tree: %v", fixture.resolvedModules)
			}
		}
	})

	t.Run("tenant is the shared kernel", func(t *testing.T) {
		fixture := sharedKernelPack()
		fixture.goMod = "module " + sharedKernelModule + "\n"
		fixture.tenant = fixture.scg
		fixture.scg = nil
		if findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings()); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
		for _, module := range fixture.resolvedModules {
			if module == sharedKernelModule {
				t.Fatalf("the home registry must resolve from the working tree: %v", fixture.resolvedModules)
			}
		}
	})

	t.Run("tenant without a module declaration", func(t *testing.T) {
		fixture := sharedKernelPack()
		fixture.goMod = ""
		if findings := fixture.verifier().verifyExtends(context.Background(), extendsBindings()); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
		if len(fixture.resolvedModules) != 2 {
			t.Fatalf("both registries must resolve through the tooling channel: %v", fixture.resolvedModules)
		}
	})
}

func TestTenantModuleIdentity(t *testing.T) {
	t.Run("module line", func(t *testing.T) {
		verifier := Verifier{ReadTenant: func(path string) ([]byte, error) {
			return []byte("module example.com/tenant\r\n\r\ngo 1.26\n"), nil
		}}
		if identity := verifier.tenantModuleIdentity(); identity != "example.com/tenant" {
			t.Fatalf("identity = %q", identity)
		}
	})

	t.Run("read error", func(t *testing.T) {
		verifier := Verifier{ReadTenant: func(path string) ([]byte, error) { return nil, errors.New("boom") }}
		if identity := verifier.tenantModuleIdentity(); identity != "" {
			t.Fatalf("identity = %q", identity)
		}
	})

	t.Run("no module line", func(t *testing.T) {
		verifier := Verifier{ReadTenant: func(path string) ([]byte, error) { return []byte("go 1.26\n"), nil }}
		if identity := verifier.tenantModuleIdentity(); identity != "" {
			t.Fatalf("identity = %q", identity)
		}
	})
}

func TestVerifyRunsTheExtendsProof(t *testing.T) {
	verifier := Verifier{
		ReadTenant: func(path string) ([]byte, error) {
			switch path {
			case "git-governance.quality.json":
				return []byte(extendsConfig(`"opentofu@1"`)), nil
			case "tools/go.mod":
				return []byte("module example.com/tenant/tools\n"), nil
			default:
				return nil, errTestMissing
			}
		},
		ReadHome: func(path string) ([]byte, error) { return nil, errTestMissing },
		ListTenant: func(path string) ([]string, error) {
			return nil, errTestMissing
		},
		ListModule: func(dir, path string) ([]string, error) {
			return nil, errTestMissing
		},
		ResolveModule: func(ctx context.Context, dir, module string) (string, error) {
			return "", errors.New("no module")
		},
	}
	findings := verifier.Verify(context.Background(), extendsBindings())
	found := false
	for _, finding := range findings {
		if finding.Check == "capability pack opentofu@1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the extends proof must run within Verify: %v", findings)
	}
}
