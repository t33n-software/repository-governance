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

## Core capabilities

| Capability | Status | Verification |
|---|---|---|
| Reusable workflow payloads | VERIFIED | `reusable-ci-go`, `reusable-codeql-go`, and `reusable-dependency-review` carry only `on: workflow_call`, full-length SHA-pinned actions, the permission matrix, bounded execution, the gate job names, and Go-native toolchain provisioning from the tenant `go.mod`; contract-test set |
| Composite actions | VERIFIED | `setup-controlled-go` and `verify-canonical-files` carry the controlled-toolchain and verifier shells; contract-test set |
| Canonical callers | VERIFIED | the four hash-pinned masters cover every shared line with the exact job names and grants; byte-identity with the home's own callers is proven by the contract-test set |
| Canonical file family | VERIFIED | `files/{gitattributes,gitignore,lefthook,dependabot,codeowners}` carry the canonical content; contract-test set |
| Conformance verifier | VERIFIED | `cmd/verify-canonical` proves caller hashes and pins, canonical files, CODEOWNERS materialization, config-seam conformance, the `go.mod` toolchain-directive guard, tool-pin admission against the identity-asserted canonical catalog, and license-lane wiring fail-closed; the flag parser accepts both the space-separated and the assignment forms; a regression guard binds every verifier home path to the real home layout; the module resolution provisions through the integrity-pinned download before the directory query, with the cold-cache order proven over the exec seam; same-package whitebox tests |
| Verifier schemas | VERIFIED | `schemas/repo-bindings/v1/` and `schemas/caller-hashes/v1/` strictly decoded; conformance vectors prove every acceptance and rejection |
| Boundary fuzzing | VERIFIED | `FuzzDecodeBindings` fuzzes the binding-manifest decoder |
| Dogfooding | VERIFIED | the home's own callers are byte-identical to the masters; `go tool -modfile tools/go.mod quality-gate` runs the canonical gate set against this repository |
| Release lifecycle adoption | VERIFIED | the seven lifecycle callers under `.github/workflows/` are byte-identical to the canonical masters of `t33n-software/git-governance` and hash-match `caller-hashes.json` (LF-normalized) |
| Governance CLI tool channel | VERIFIED | `tools/go.mod` pins `github.com/t33n-software/git-governance/cmd/git-governance`; `go build -modfile tools/go.mod` proves the lane build |
