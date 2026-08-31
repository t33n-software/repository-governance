# Product Acceptance Matrix

This matrix is the repository-local source of truth for delivery status. It
does not rely on any external governance repository or unpublished rule set.

## Status legend

- `IMPLEMENTED`: source code exists.
- `VERIFIED`: automated tests or an actual local execution succeeded.
- `IN_PROGRESS`: a confirmed gap is actively being remediated.
- `PENDING`: intentionally planned but not yet delivered.
- `BLOCKED`: cannot be verified because an external prerequisite is absent.

## Verified baseline

| Item | Status | Evidence |
|---|---|---|
| Local repository | VERIFIED | `main` is initialized; every audit and release gate begins by checking the current Git status |
| Go module | VERIFIED | `github.com/t33n-software/repository-governance`, language Go 1.26 and pinned toolchain Go 1.26.6 |
| License instance | VERIFIED | the tenant values `license.values.json` and the digest-pinned lock `license.lock.json` (template `license-hub/templates/custom/norepublish/NoRepublish-1.0.0.hbs`, version 1.0.0) render `LICENSE` and `LICENSES/LicenseRef-repository-governance-NoRepublish-1.0.txt`; the committed instance is proven byte-identical against the canonical render through the license-hub CLI (`verify` reports the instance matches the canonical render) |

## Core capabilities

| Capability | Status | Verification |
|---|---|---|
| Reusable workflow payloads | VERIFIED | `reusable-ci-go`, `reusable-codeql-go`, `reusable-dependency-review`, `reusable-release-config`, and `reusable-canonical-conformance` carry only `on: workflow_call`, full-length SHA-pinned actions, the permission matrix, bounded execution, and the gate job names; the Go payloads carry exact pinned toolchain provisioning — a fail-closed resolution step reads the `toolchain` directive of the tenant `go.mod` and setup-go installs exactly that version through `go-version` (never the `go` directive, never the latest patch); the CI payload carries the constant-size capability-pack provisioning seam (exactly one `quality-gate provision` step before exactly one gate step, never a per-capability step); the conformance payload references the home's verify-canonical-files action at the canonical home pin (bound by a regression guard); contract-test set |
| Composite actions | VERIFIED | `setup-controlled-go` and `verify-canonical-files` carry the controlled-toolchain and verifier shells; the verifier action's internal `setup-controlled-go` reference is bound to the canonical home pin by a regression guard; contract-test set |
| Canonical callers | VERIFIED | the five hash-pinned masters cover every shared line with the exact job names and grants; the canonical home pin is re-issued at the revision whose payloads and composite action provision exactly the pinned toolchain (the fail-closed resolution step and the exact `go-version` installation), and the home's tool channel pins the provision-capable orchestrator with the fail-closed format proof; byte-identity with the home's own callers is proven by the contract-test set |
| Canonical file family | VERIFIED | `files/{gitattributes,gitignore,lefthook,dependabot,codeowners,conventions}` carry the canonical content; the conventions theme renders the tenant's rule-sets README from the value-free template with the manifest's values and the class-derived platform sentence; contract-test set |
| Conformance verifier | VERIFIED | `cmd/verify-canonical` proves caller hashes and pins, canonical files, CODEOWNERS materialization, config-seam conformance at schema version 4 (the language-keyed toolchain identity and the `extends` declaration), the `extends` resolution against the capability-pack registry at the tenant's pinned tool stand (the two-tier registry union through the integrity-pinned tooling channel, the working-tree registries of the homes, the pinned reference grammar, and the descriptor identity binding — an unknown, unpinned, or ambiguous reference fails closed), the directory-only registry enumeration that never treats the anchor package files at the registry root as capability areas on any platform, the `go.mod` toolchain-directive guard, tool-pin admission against the identity-asserted canonical catalog, and the license content proof fail-closed — where the license-hub family is bound, the verifier orchestrates the tenant-pinned `license` CLI (the locked template resolved inside the pinned hub module directory) and surfaces the tool's own violation output, never re-implementing the proof; the conventions README materialization where the manifest binds the conventions family (the exact render of the value-free template with the manifest's values and the class-derived platform sentence; a class without a canonical render fails closed); the flag parser accepts both the space-separated and the assignment forms; a regression guard binds every verifier home path to the real home layout; the module resolution provisions through the integrity-pinned download before the directory query, with the cold-cache order proven over the exec seam; same-package whitebox tests |
| Verifier schemas | VERIFIED | `schemas/repo-bindings/v1/` (the config-seam pin is `const: 4`) and `schemas/caller-hashes/v1/` strictly decoded; conformance vectors prove every acceptance and rejection, including the hard cut from the v3 seam |
| Boundary fuzzing | VERIFIED | `FuzzDecodeBindings` fuzzes the binding-manifest decoder |
| Dogfooding | VERIFIED | the home's own callers are byte-identical to the masters; `go tool -modfile tools/go.mod quality-gate` runs the canonical gate set against this repository |
| Release lifecycle adoption | VERIFIED | the seven lifecycle callers under `.github/workflows/` are byte-identical to the canonical masters of `t33n-software/git-governance` and hash-match `caller-hashes.json` (LF-normalized) |
| Governance CLI tool channel | VERIFIED | `tools/go.mod` pins `github.com/t33n-software/git-governance/cmd/git-governance`; `go build -modfile tools/go.mod` proves the lane build |
