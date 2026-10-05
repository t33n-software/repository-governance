package canonical

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Materialization is one proven tenant write: the repository-relative tenant
// path, the proven bytes to write, and the home-relative source the bytes
// were copied or rendered from.
type Materialization struct {
	Path     string
	Contents []byte
	Source   string
}

// Provisioner renders a tenant's canonical surfaces from the pinned home and
// territory trees and writes them fail-closed. It is the write exposure of
// the same render core the Verifier proves with: every copied master, every
// pinned territory artifact, and every rendered region is proven against the
// binding manifest's recorded hashes before any write happens, so a
// provisioned tenant passes the conformance verifier byte for byte. Every
// seam is injected so each proof stays whitebox-testable without a real
// tree; the production bindings are constructed by NewProvisioner.
type Provisioner struct {
	// ReadHome reads a home file by its home-relative slash path.
	ReadHome func(path string) ([]byte, error)
	// ReadTerritory reads a territory file by its territory-relative slash
	// path; the territory carries the pinned config artifacts of the
	// tenant's language.
	ReadTerritory func(path string) ([]byte, error)
	// ReadTenant reads a tenant file by its repository-relative slash path.
	ReadTenant func(path string) ([]byte, error)
	// WriteTenant writes a tenant file by its repository-relative slash path.
	WriteTenant func(path string, contents []byte) error
}

// Plan renders every bound tenant surface from the pinned home and territory
// trees, proves each copied master, pinned territory artifact, and rendered
// region against the binding manifest's recorded hashes, and returns the
// proven materializations — without writing anything. An unreadable master,
// an unknown fragment, a diverging hash, an unknown artifact pairing, or an
// existing gitignore file that carries neither the rendered region nor the
// project-block mark is an error, never a weakened output.
func (p Provisioner) Plan(bindings Bindings) ([]Materialization, error) {
	materials := make([]Materialization, 0, 8)
	materials, err := p.planCallers(bindings, materials)
	if err != nil {
		return nil, err
	}
	materials, err = p.planFileTopics(bindings, materials)
	if err != nil {
		return nil, err
	}
	materials, err = p.planGitignore(bindings, materials)
	if err != nil {
		return nil, err
	}
	materials, err = p.planCodeowners(bindings, materials)
	if err != nil {
		return nil, err
	}
	materials, err = p.planConventions(bindings, materials)
	if err != nil {
		return nil, err
	}
	return p.planToolchain(bindings, materials)
}

// Apply plans the proven materializations and writes them to the tenant
// tree. Every proof runs before the first write, so a failing proof leaves
// the tenant untouched; a failing write surfaces the exact path.
func (p Provisioner) Apply(bindings Bindings) ([]Materialization, error) {
	materials, err := p.Plan(bindings)
	if err != nil {
		return nil, err
	}
	for _, material := range materials {
		if err := p.WriteTenant(material.Path, material.Contents); err != nil {
			return nil, fmt.Errorf("write %s: %w", material.Path, err)
		}
	}
	return materials, nil
}

// planCallers materializes every bound caller as the byte-identical copy of
// its canonical master, proven against the bound hash.
func (p Provisioner) planCallers(bindings Bindings, materials []Materialization) ([]Materialization, error) {
	for _, caller := range bindings.Callers {
		master, err := p.ReadHome(caller.Master)
		if err != nil {
			return nil, fmt.Errorf("read the canonical caller master %s: %w", caller.Master, err)
		}
		if hash := Sum256Hex(master); hash != caller.SHA256 {
			return nil, fmt.Errorf("the canonical caller master %s hashes %s, not the bound %s", caller.Master, hash, caller.SHA256)
		}
		materials = append(materials, Materialization{Path: caller.File, Contents: master, Source: "master " + caller.Master})
	}
	return materials, nil
}

// planFileTopics materializes every byte-identical canonical file topic from
// its home master, proven against the bound hash.
func (p Provisioner) planFileTopics(bindings Bindings, materials []Materialization) ([]Materialization, error) {
	for _, topic := range fileTopics {
		binding := topic.binding(bindings.Files)
		master, err := p.ReadHome(topic.homePath)
		if err != nil {
			return nil, fmt.Errorf("read the canonical %s master %s: %w", topic.topic, topic.homePath, err)
		}
		if hash := Sum256Hex(master); hash != binding.SHA256 {
			return nil, fmt.Errorf("the canonical %s master %s hashes %s, not the bound %s", topic.topic, topic.homePath, hash, binding.SHA256)
		}
		materials = append(materials, Materialization{Path: binding.Path, Contents: master, Source: "master " + topic.homePath})
	}
	return materials, nil
}

// planGitignore materializes the composed tenant gitignore: the governed
// region rendered from the bound fragment list at the bound home pin and
// proven against the bound hash, with the tenant's free project block
// preserved below the mark. A missing tenant file provisions the rendered
// region alone.
func (p Provisioner) planGitignore(bindings Bindings, materials []Materialization) ([]Materialization, error) {
	binding := bindings.Files.Gitignore
	rendered, err := RenderGitignoreGovernedRegion(p.ReadHome, binding.Fragments, bindings.Home.SHA)
	if err != nil {
		return nil, fmt.Errorf("the bound gitignore fragments do not render: %w", err)
	}
	if hash := Sum256Hex(rendered); hash != binding.SHA256 {
		return nil, fmt.Errorf("the rendered gitignore governed region hashes %s, not the bound %s", hash, binding.SHA256)
	}
	contents := rendered
	if existing, err := p.ReadTenant(binding.Path); err == nil {
		composed, err := preserveGitignoreProjectBlock(existing, rendered)
		if err != nil {
			return nil, err
		}
		contents = composed
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read the tenant %s: %w", binding.Path, err)
	}
	materials = append(materials, Materialization{
		Path:     binding.Path,
		Contents: contents,
		Source:   "render gitignore " + strings.Join(binding.Fragments, " + "),
	})
	return materials, nil
}

// preserveGitignoreProjectBlock composes the re-rendered governed region
// with the existing tenant file's free project block: a tenant file that
// already carries the rendered region keeps everything below it; a drifted
// file that still carries the project-block mark keeps everything below the
// mark; a file that carries neither is refused — the provisioner never
// overwrites unmarked content.
func preserveGitignoreProjectBlock(existing, rendered []byte) ([]byte, error) {
	if bytes.HasPrefix(existing, rendered) {
		return append(slices.Clone(rendered), existing[len(rendered):]...), nil
	}
	if index := bytes.Index(existing, []byte(projectBlockMark)); index >= 0 {
		tail := bytes.TrimPrefix(existing[index+len(projectBlockMark):], []byte("\n"))
		return append(slices.Clone(rendered), tail...), nil
	}
	return nil, errors.New("the existing tenant gitignore carries neither the rendered governed region nor the project-block mark; unmarked content is never overwritten")
}

// planCodeowners materializes the tenant ownership file as the render of the
// canonical template with the manifest's values.
func (p Provisioner) planCodeowners(bindings Bindings, materials []Materialization) ([]Materialization, error) {
	template, err := p.ReadHome(codeownersTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("read the canonical CODEOWNERS template %s: %w", codeownersTemplatePath, err)
	}
	if !strings.Contains(string(template), codeownersToken) {
		return nil, fmt.Errorf("the canonical CODEOWNERS template carries no %s token", codeownersToken)
	}
	rendered := strings.ReplaceAll(string(template), codeownersToken, bindings.Codeowners.DefaultOwner)
	materials = append(materials, Materialization{
		Path:     bindings.Codeowners.Path,
		Contents: []byte(rendered),
		Source:   "render " + codeownersTemplatePath,
	})
	return materials, nil
}

// planConventions materializes the rule-sets conventions README where the
// manifest binds the family; a tenant without the binding is not provisioned
// for it.
func (p Provisioner) planConventions(bindings Bindings, materials []Materialization) ([]Materialization, error) {
	if bindings.Conventions == nil {
		return materials, nil
	}
	template, err := p.ReadHome(conventionsTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("read the canonical conventions template %s: %w", conventionsTemplatePath, err)
	}
	rendered, err := renderConventionsReadme(string(template), *bindings.Conventions, bindings.Class.QualityGates)
	if err != nil {
		return nil, fmt.Errorf("the conventions template does not render: %w", err)
	}
	materials = append(materials, Materialization{
		Path:     bindings.Conventions.Path,
		Contents: []byte(rendered),
		Source:   "render " + conventionsTemplatePath,
	})
	return materials, nil
}

// planToolchain materializes the optional territory config-topic binding:
// the byte-identity config artifacts from the pinned territory tree and the
// composed pnpm workspace document. A nil binding skips the family for
// tenants that bind no category.
func (p Provisioner) planToolchain(bindings Bindings, materials []Materialization) ([]Materialization, error) {
	if bindings.Toolchain == nil {
		return materials, nil
	}
	materials, err := p.planToolchainArtifacts(*bindings.Toolchain, materials)
	if err != nil {
		return nil, err
	}
	return p.planPnpmWorkspace(*bindings.Toolchain, materials)
}

// planToolchainArtifacts materializes every byte-identity config artifact as
// the copy of its pinned territory form: the pinned registry proves the
// category's artifact mapping, and the territory artifact's hash must equal
// the bound hash before the write — the same proof the verifier runs against
// both trees after it.
func (p Provisioner) planToolchainArtifacts(toolchain ToolchainBindings, materials []Materialization) ([]Materialization, error) {
	instance, err := pinnedCategoryRegistry(p.ReadTerritory, toolchain.Registry)
	if err != nil {
		return nil, err
	}
	entry := categoryRegistryLookup(instance, toolchain.Category)
	if entry == nil {
		return nil, fmt.Errorf("the pinned registry carries no entry for the declared category %q", toolchain.Category)
	}
	for _, artifact := range toolchain.Artifacts {
		dir, mapped := entry.Artifacts[artifact.Family]
		if !mapped {
			return nil, fmt.Errorf("the pinned registry entry carries no artifact mapping for the family %q", artifact.Family)
		}
		if derived := categoryTerritoryDir(artifact.Family, toolchain.Category); dir != derived {
			return nil, fmt.Errorf("the pinned registry maps the family %q to %q, but the category identity derives %q", artifact.Family, dir, derived)
		}
		territoryPath := dir + path.Base(artifact.Path)
		contents, err := p.ReadTerritory(territoryPath)
		if err != nil {
			return nil, fmt.Errorf("read the territory %s artifact %s: %w", artifact.Family, territoryPath, err)
		}
		if hash := Sum256Hex(contents); hash != artifact.SHA256 {
			return nil, fmt.Errorf("the territory %s artifact %s hashes %s, not the bound %s", artifact.Family, territoryPath, hash, artifact.SHA256)
		}
		materials = append(materials, Materialization{Path: artifact.Path, Contents: contents, Source: "territory " + territoryPath})
	}
	return materials, nil
}

// planPnpmWorkspace materializes the composed tenant pnpm workspace document:
// every governed key of the territory's fortress baseline carries the exact
// baseline value, and the tenant's own mapping keys — the catalog above all —
// are preserved; a fresh tenant receives the baseline with an empty catalog.
// The policy invariants, never a byte identity, are this family's proof form:
// the composed re-render is what the verifier's class-four proof passes.
func (p Provisioner) planPnpmWorkspace(toolchain ToolchainBindings, materials []Materialization) ([]Materialization, error) {
	baselinePath := territoryPnpmBaselinePath()
	baselineContents, err := p.ReadTerritory(baselinePath)
	if err != nil {
		return nil, fmt.Errorf("read the territory pnpm fortress baseline %s: %w", baselinePath, err)
	}
	baseline := map[string]any{}
	if err := yaml.Unmarshal(baselineContents, &baseline); err != nil {
		return nil, fmt.Errorf("the territory pnpm fortress baseline %s must be a valid YAML mapping: %w", baselinePath, err)
	}
	tenant := map[string]any{}
	if existing, err := p.ReadTenant(toolchain.PnpmWorkspace.Path); err == nil {
		if err := yaml.Unmarshal(existing, &tenant); err != nil {
			return nil, fmt.Errorf("the existing tenant %s must be a valid YAML mapping; unparseable content is never overwritten", toolchain.PnpmWorkspace.Path)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read the tenant %s: %w", toolchain.PnpmWorkspace.Path, err)
	}
	for _, key := range sortedYAMLKeys(baseline) {
		tenant[key] = baseline[key]
	}
	if _, found := tenant[pnpmCatalogKey]; !found {
		tenant[pnpmCatalogKey] = map[string]any{}
	}
	// The composed mapping carries only YAML-decoded values with string
	// keys, so the re-render cannot fail; its key order is deterministic.
	composed, _ := yaml.Marshal(tenant)
	materials = append(materials, Materialization{
		Path:     toolchain.PnpmWorkspace.Path,
		Contents: composed,
		Source:   "compose " + baselinePath,
	})
	return materials, nil
}

// NewProvisioner binds the production seams of a Provisioner: os-backed reads
// rooted at the tenant, home, and territory trees and an os-backed write seam
// rooted at the tenant tree that creates missing parent directories.
func NewProvisioner(tenantRoot, homeRoot, territoryRoot string) Provisioner {
	return Provisioner{
		ReadHome: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(homeRoot, filepath.FromSlash(path)))
		},
		ReadTerritory: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(territoryRoot, filepath.FromSlash(path)))
		},
		ReadTenant: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(tenantRoot, filepath.FromSlash(path)))
		},
		WriteTenant: func(path string, contents []byte) error {
			return writeTenantFile(tenantRoot, path, contents)
		},
	}
}

// writeTenantFile writes one tenant file below the tenant root, creating
// missing parent directories.
func writeTenantFile(tenantRoot, path string, contents []byte) error {
	target := filepath.Join(tenantRoot, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, contents, 0o644)
}
