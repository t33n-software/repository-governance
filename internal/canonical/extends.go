package canonical

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// packSchemaID is the canonical capability pack descriptor schema identifier.
// The schema is owned by the supply-chain-governance shared kernel; the
// verifier asserts the identity binding of a resolved descriptor and never
// redefines or copies the schema.
const packSchemaID = "capability-pack/v1"

// sharedKernelModule is the module path of the shared kernel that carries the
// language-neutral capability packs. The territory home module
// (qualityAuthorityModule, tools.go) carries the language-bound packs; a pack
// exists exactly once across the union of both registries.
const sharedKernelModule = "github.com/t33n-software/supply-chain-governance"

// packReferencePattern is the extends reference grammar: a pinned
// <capability>@<major> reference — lowercase kebab, an exact major version,
// no ranges, no latest.
var packReferencePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*@[0-9]+$`)

// packReference is one validated extends declaration.
type packReference struct {
	raw        string
	capability string
	major      int
}

// parsePackReference validates and splits a <capability>@<major> reference. A
// malformed or unpinned reference is a fail-closed governance finding, never
// a silent skip.
func parsePackReference(reference string) (packReference, error) {
	if !packReferencePattern.MatchString(reference) {
		return packReference{}, fmt.Errorf("the capability pack reference %q is not a pinned <capability>@<major> reference", reference)
	}
	capability, rawMajor, _ := strings.Cut(reference, "@")
	major, _ := strconv.Atoi(rawMajor)
	return packReference{raw: reference, capability: capability, major: major}, nil
}

// packIdentityDocument is the identity projection of a capability-pack/v1
// descriptor. The descriptor schema is owned by the shared kernel and is
// strictly decoded by the orchestrator at execution time; the verifier
// asserts the identity binding of the registry location only, so a schema
// evolution within the pack contract never breaks the conformance proof.
type packIdentityDocument struct {
	Schema     string `json:"schema"`
	Capability string `json:"capability"`
	Area       string `json:"area"`
	Version    int    `json:"version"`
}

// decodePackIdentity decodes the identity projection of a pack descriptor and
// asserts its schema identity.
func decodePackIdentity(contents []byte) (packIdentityDocument, error) {
	var document packIdentityDocument
	if len(contents) == 0 {
		return document, errors.New("the pack descriptor must not be empty")
	}
	if err := json.Unmarshal(contents, &document); err != nil {
		return document, fmt.Errorf("the pack descriptor must contain valid JSON: %w", err)
	}
	if document.Schema != packSchemaID {
		return document, fmt.Errorf("the pack descriptor schema must be %q, got %q", packSchemaID, document.Schema)
	}
	return document, nil
}

// registryTree is one resolvable capability-pack registry tree: the
// capabilities tree of an owning module at the tenant's pinned tool stand or
// of a home's own working tree.
type registryTree struct {
	owner string
	read  func(path string) ([]byte, error)
	list  func(path string) ([]string, error)
}

// registryPath joins a registry-relative path below the capabilities root.
func registryPath(path string) string {
	if path == "" {
		return "capabilities"
	}
	return "capabilities/" + path
}

// registrySearch carries the resolvable registry trees plus the owners whose
// registry could not be resolved, so an unknown-reference finding names both
// the searched and the unavailable registries.
type registrySearch struct {
	trees       []registryTree
	unavailable []string
}

// describe renders the searched and unavailable registries for the
// unknown-reference finding.
func (s registrySearch) describe() string {
	searched := make([]string, 0, len(s.trees))
	for _, tree := range s.trees {
		searched = append(searched, tree.owner)
	}
	text := " (searched: " + strings.Join(searched, ", ") + ")"
	if len(s.unavailable) > 0 {
		text += " (unavailable: " + strings.Join(s.unavailable, "; ") + ")"
	}
	return text
}

// verifyExtends proves every extends reference of the tenant's configuration
// seam against the capability-pack registry at the tenant's pinned tool
// stand: the reference grammar, the registry resolution through the tenant's
// integrity-pinned tooling channel, and the identity match of the resolved
// descriptor. An unknown, unpinned, or ambiguous reference is a fail-closed
// finding. A tenant without declarations resolves nothing. The config-seam
// proof owns the read and decode findings of the configuration itself, so an
// unreadable or undecodable config skips this proof without weakening the
// fail-closed result.
func (v Verifier) verifyExtends(ctx context.Context, bindings Bindings) []Finding {
	check := "capability packs"
	contents, err := v.ReadTenant(bindings.Quality.Config)
	if err != nil {
		return nil
	}
	references, err := DecodeQualityConfigExtends(contents)
	if err != nil {
		return nil
	}
	if len(references) == 0 {
		return nil
	}

	findings := make([]Finding, 0)
	parsed := make([]packReference, 0, len(references))
	for _, reference := range references {
		ref, err := parsePackReference(reference)
		if err != nil {
			findings = append(findings, mismatchFinding(check, err.Error()))
			continue
		}
		parsed = append(parsed, ref)
	}

	if len(parsed) == 0 {
		return findings
	}

	// A pack resolution requires the tenant's integrity-pinned tooling
	// channel: at least one registry is always resolved through it, because a
	// tenant is never both registry owners at once.
	if _, err := v.ReadTenant(bindings.Tools.Module); errors.Is(err, fs.ErrNotExist) {
		return append(findings, mismatchFinding(check,
			fmt.Sprintf("capability packs require the tenant's integrity-pinned tooling module (%s): no tools module is present", bindings.Tools.Module)))
	} else if err != nil {
		return append(findings, readErrorFinding(check, bindings.Tools.Module, err))
	}

	search := v.resolveRegistries(ctx, bindings)
	for _, ref := range parsed {
		findings = append(findings, v.resolvePackReference(search, ref)...)
	}
	return findings
}

// tenantModuleIdentity reads the tenant's own module declaration; a
// repository without a go.mod or without a module line is not a registry
// owner. The toolchain proof owns the go.mod findings.
func (v Verifier) tenantModuleIdentity() string {
	contents, err := v.ReadTenant("go.mod")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n") {
		if module, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return strings.TrimSpace(module)
		}
	}
	return ""
}

// resolveRegistries locates the registry trees of the two-tier registry. A
// home resolves its own registry from the working tree; every other registry
// is resolved through the tenant's integrity-pinned tooling module. A
// registry that cannot be resolved is recorded as unavailable; only a
// reference found nowhere fails closed.
func (v Verifier) resolveRegistries(ctx context.Context, bindings Bindings) registrySearch {
	search := registrySearch{}
	module := v.tenantModuleIdentity()
	toolsDir := filepath.Join(v.TenantRoot, path.Dir(bindings.Tools.Module))
	for _, owner := range []string{qualityAuthorityModule, sharedKernelModule} {
		if module == owner {
			search.trees = append(search.trees, registryTree{
				owner: owner,
				read: func(path string) ([]byte, error) {
					return v.ReadTenant(registryPath(path))
				},
				list: func(path string) ([]string, error) {
					return v.ListTenant(registryPath(path))
				},
			})
			continue
		}
		dir, err := v.ResolveModule(ctx, toolsDir, owner)
		if err != nil {
			search.unavailable = append(search.unavailable, owner+": "+err.Error())
			continue
		}
		resolved := dir
		search.trees = append(search.trees, registryTree{
			owner: owner,
			read: func(path string) ([]byte, error) {
				return v.ReadModule(resolved, registryPath(path))
			},
			list: func(path string) ([]string, error) {
				return v.ListModule(resolved, registryPath(path))
			},
		})
	}
	return search
}

// resolvePackReference resolves one validated reference against every
// registry tree: exactly one match passes, no match is a fail-closed unknown
// finding, and more than one match is a fail-closed ambiguity finding — a
// pack exists exactly once. A descriptor that cannot be read, decoded, or
// identity-matched is a registry integrity finding.
func (v Verifier) resolvePackReference(search registrySearch, reference packReference) []Finding {
	check := "capability pack " + reference.raw
	suffix := "/" + reference.capability + "/v" + strconv.Itoa(reference.major) + "/pack.json"
	matches := make([]string, 0, 1)
	findings := make([]Finding, 0)
	for _, tree := range search.trees {
		areas, err := tree.list("")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			findings = append(findings, mismatchFinding(check,
				fmt.Sprintf("list the registry %s: %v", tree.owner, err)))
			continue
		}
		for _, area := range areas {
			descriptorPath := area + suffix
			contents, err := tree.read(descriptorPath)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				findings = append(findings, mismatchFinding(check,
					fmt.Sprintf("read the pack descriptor %s in %s: %v", descriptorPath, tree.owner, err)))
				continue
			}
			identity, err := decodePackIdentity(contents)
			if err != nil {
				findings = append(findings, mismatchFinding(check,
					fmt.Sprintf("the pack descriptor %s in %s is invalid: %v", descriptorPath, tree.owner, err)))
				continue
			}
			if identity.Capability != reference.capability || identity.Area != area || identity.Version != reference.major {
				findings = append(findings, mismatchFinding(check, fmt.Sprintf(
					"the pack descriptor %s in %s carries the identity %s/%s v%d, not the %s/%s v%d of its registry location",
					descriptorPath, tree.owner, identity.Area, identity.Capability, identity.Version,
					area, reference.capability, reference.major)))
				continue
			}
			matches = append(matches, tree.owner)
		}
	}
	if len(findings) > 0 {
		return findings
	}
	switch len(matches) {
	case 0:
		return []Finding{mismatchFinding(check,
			"the reference is unknown to the capability-pack registries at the pinned stand"+search.describe())}
	case 1:
		return nil
	default:
		return []Finding{mismatchFinding(check, fmt.Sprintf(
			"the reference is ambiguous: it is carried by %d registries (%s), but a pack exists exactly once",
			len(matches), strings.Join(matches, ", ")))}
	}
}
