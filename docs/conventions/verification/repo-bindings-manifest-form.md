# Repository bindings manifest form

**Status:** binding convention of the repository-governance home. This
document binds the mechanism surface of the tenant binding manifest
(`repo-bindings.json`) and the config-topic proofs the conformance verifier
carries.

## Rule

The binding manifest is the **one** trust boundary between the fleet and a
tenant. Its form is language-agnostic; its values are language-conditional.
The manifest already carries Go-coupled bindings — the `go.mod` toolchain
directive proof, the `tools/go.mod` tooling-module pin, the Go dependabot
master, and the `go/core` gitignore fragment. A Node/TypeScript tenant binds
its surfaces through the same form with its own values. A per-language
manifest fork is forbidden: it would split one trust boundary into two
documents and duplicate the schema, decoder, and conformance machinery.

The optional `toolchain` section is the language-territory binding surface:
its territory pin names the language territory explicitly, and it is present
only for tenants that bind a toolchain-config category through that
territory's pinned registry. The pnpm workspace binding lives inside this
section — the package-manager surface is a language-conditional value of the
one form, never a second schema.

## The declaration discipline

The category is declared on exactly two surfaces that must agree: the config
seam (`toolchain.category`) and the binding manifest's `toolchain` section.

- Neither surface declares a category → every config-topic proof is skipped;
  the absence form is valid.
- Exactly one surface declares a category → a fail-closed finding, never a
  silent state.
- The surfaces declare divergent categories → a fail-closed finding.
- The section binds the node-typescript territory's config families: the
  config seam's language must declare `node-typescript`; a divergent or
  missing language is a fail-closed finding.

## The proof families

| Family | Proof form |
|---|---|
| `tsconfig`, `vitest`, `tsdown` | byte identity against the pinned territory artifact and the bound hash |
| `pnpm` workspace | the policy invariants proven key-by-key against the territory's fortress baseline; the catalog content stays the tenant's own |
| The delivery lane (`tsconfig.node.json`) | the lane's invariant guards plus the declared source-root coverage |

The category ID is the hierarchical kebab path whose shape equals the
territory folder path (`configs/<family>/<category-id>/`); the registry's
artifact mapping must equal the derived folder — declaration, folder, and
artifacts are one path without a mapping table. A lane without an implemented
guard set (the monorepo lane today) fails closed with a precise finding,
never a skipped proof.

## Verification authority

- The whitebox tests over the decode and every proof path
  (`internal/canonical/toolchainconfig_test.go` and the bindings decode
  tests).
- The conformance vectors (`conformance/negative/invalid-toolchain-*.json`)
  prove that the decoder rejects every invalid section form fail-closed.
- The canonical quality gate of this home (exactly 100.0 percent statement
  coverage per executable package, race, vet, static analysis, govulncheck,
  boundary fuzzing).

## Do / Don't

- ✅ Do declare the category on both surfaces or on neither; the mixed form
  is a finding by design.
- ✅ Do bind the territory pin and the registry hash explicitly; the proofs
  run against the pinned tree, never against a remembered state.
- ❌ Don't fork the manifest per language; the form is one boundary.
- ❌ Don't carry a language surface such as the pnpm workspace outside the
  optional territory section; the section is the language-conditional form.
- ❌ Don't soften a missing guard set into a skipped proof; the guard set
  lands with the first binding of its lane.
