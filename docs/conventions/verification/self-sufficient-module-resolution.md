# Self-sufficient module resolution

**Status:** binding convention of the repository-governance home.
**Contract:** `REPOSITORY_GOVERNANCE_HOME_CONTRACT_001` section 7 binds the
environment guarantee of the conformance verifier; this document binds the
mechanism.

## Rule

A governed verification component MUST provision every module it reads
through the tenant's integrity-pinned tooling channel (`tools/go.mod` +
`tools/go.sum`) and MUST NOT trust a warm module cache. A cold cache — the
default state of a CI lane — is a normal input, never a failure mode.

The verifier's only external surface is the tenant's tooling module. It never
dereferences document references (the `$schema` identity is asserted, never
fetched), and it never requires the caller, the lane, or the operator to
pre-stage the module cache.

## Bound mechanism

`internal/canonical/production.go` — `ResolveModuleDir`:

1. `go mod download <module>` — populates the module cache through the
   sum-pinned channel; a checksum divergence fails closed.
2. `go list -m -f {{.Dir}} <module>` — resolves the cache directory only
   after the download succeeded.

The download MUST precede the directory query: `go list -m` reports an empty
`Dir` for a required module that is not yet present in the cache. The order
is proven by the whitebox regression tests in
`internal/canonical/production_test.go` (the sequence proof, the
download-failure fail-closed path, the list-failure path, and the
empty-directory rejection).

## Proven instance

The DA-8 pilot conformance lane (the tenant pull request CI) failed closed
with `go list -m ... returned no directory`: the lane builds only the
verifier tool, so the catalog module was absent from the cold runner cache,
while the local verification had a warm cache and passed. An
environment-dependent proof is not a proof; this convention eliminates that
class.

## Verification authority

- The whitebox regression tests over the exec seam (the call order and every
  failure mode).
- The canonical quality gate of this home (exactly 100.0 percent statement
  coverage, race, vet, staticcheck, govulncheck, boundary fuzzing).
- The tenant conformance lanes, which run the verifier in cold-cache CI
  environments on every pull request.

## Do / Don't

- ✅ Do provision every module the verifier reads through `go mod download`
  before resolving its directory.
- ✅ Do keep the resolution inside the verifier's seam so every caller and
  lane inherits the guarantee.
- ❌ Don't pre-stage the module cache in a workflow, action, or operator
  step as a substitute for the verifier's own provisioning.
- ❌ Don't resolve a module directory with `go list -m` before its download
  completed.
