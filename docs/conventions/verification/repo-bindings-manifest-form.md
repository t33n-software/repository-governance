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
territory's pinned registry.

## Selections, not facts

The binding manifest binds **selections**; it never restates **facts** whose
owner surface is an ecosystem package contract. A selection is a fact no other
governed document carries — the home pin, the fleet classes, the distribution
axis, the caller and file bindings, the ordered fragment lists, the per-artifact
provenance home, the category, the source roots. A fact with an ecosystem owner
— the package-manager identity, the toolchain directive, the location of the
workspace document — is derived from that owner surface and proven fail-closed
against it. Declaring such a fact in the manifest would create a second,
unproven authority beside the contract file that already owns it; the same
reasoning that forbids a third package-manager authority beside
`packageManager` and `devEngines.packageManager` forbids a fourth one here.

The derivation itself is territory-owned **data**, never a fleet-side constant
and never a per-ecosystem code file in this home: the pinned category registry
carries a `derivation` block (`contractFile`, `format`, `identity.field`,
`identity.form`, `workspaceFiles`, `baselineFile`, `tenantOwnedKeys`), and the
fleet carries only the readers. The interpreter is agnostic; the facts are
data; the folders do not exist. A second ecosystem joins through its own
territory home and one registry row — never through new fleet code, because the
only growing code axis is the format reader set, not the ecosystem count.

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
- A pinned registry whose instance carries no `derivation` block while the
  manifest binds a toolchain section → a fail-closed finding; the
  package-manager identity would have no declared owner surface.

## The proof families

| Family | Proof form |
|---|---|
| `tsconfig`, `vitest`, `tsdown`, `eslint` | byte identity against the pinned artifact of the provenance home and the bound hash |
| `vscodeignore` | the composition family: the governed region renders from the declared fragment list of its provenance home and proves byte-exact — the proof implementation lands with the fragment home (VEXG) and the multi-home read seam |
| The workspace document of the derived tool | the policy invariants proven key-by-key against the territory's fortress baseline; the tenant-owned keys keep their content; the path comes from the derivation block, never from the manifest |
| The delivery lane (`tsconfig.node.json`) | the lane's invariant guards plus the declared source-root coverage |

The category ID is the hierarchical kebab path whose shape equals the
territory folder path (`configs/<family>/<category-id>/`); the registry's
artifact mapping must equal the derived folder — declaration, folder, and
artifacts are one path without a mapping table. A lane without an implemented
guard set (the monorepo lane, the composition family today) fails closed with a
precise finding, never a skipped proof.

## Verification authority

- The whitebox tests over the decode, the derivation, and every proof path
  (`internal/canonical/toolchainconfig_test.go`,
  `internal/canonical/packagemanager_test.go`, and the bindings decode
  tests).
- The schema-decoder lockstep test proves the v3 family enum equals the
  decoder's closed family sets and the distribution enum equals the decoder's
  closed class set, in both directions.
- The conformance vectors (`conformance/negative/invalid-*.json` and
  `conformance/negative/toolchain-*.json`) prove that the decoder rejects
  every invalid section form fail-closed.
- The canonical quality gate of this home (exactly 100.0 percent statement
  coverage per executable package, race, vet, static analysis, govulncheck,
  boundary fuzzing).

## Do / Don't

- ✅ Do declare the category on both surfaces or on neither; the mixed form
  is a finding by design.
- ✅ Do bind the territory pin and the registry hash explicitly; the proofs
  run against the pinned tree, never against a remembered state.
- ❌ Don't fork the manifest per language; the form is one boundary.
- ❌ Don't declare an ecosystem-owned fact (package manager, toolchain
  directive, tool document location) in the manifest; derive it from the
  owner surface and prove it there.
- ❌ Don't place the derivation facts in fleet code or in a per-ecosystem
  folder tree; they are territory-owned data in the pinned registry, and the
  fleet carries only readers.
- ❌ Don't soften a missing guard set into a skipped proof; the guard set
  lands with the first binding of its lane.
