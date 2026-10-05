# provision-canonical

This document is the **mechanism** surface of the tenant-surface
provisioning: how a tenant's canonical surfaces are rendered and written
from the binding manifest, and how the provisioning relates to the
conformance verifier.

## The provisioning CLI

`cmd/provision-canonical` is the write exposure of the render core that
`cmd/verify-canonical` proves with — one core truth, two exposures:

```bash
go tool -modfile tools/go.mod provision-canonical --repo .
```

The CLI reads the tenant's `repo-bindings.json`, resolves the pinned home
tree (the explicit `--home` flag wins; without it, the home module is
resolved through the tenant's integrity-pinned tooling module), renders
every bound surface, proves each copied master and rendered region against
the manifest's recorded hashes fail-closed, and writes the proven
materializations. `--dry-run` previews the plan; the mutation requires
`--yes` in a non-interactive context or an explicit confirmation on a
terminal. The selection is reviewable, versioned manifest data; the CLI
carries no selection flags.

## The provisioned surfaces

| Surface | Mechanism |
|---|---|
| Bound callers | byte-identical copies of the canonical masters, hash-proven before the write |
| `lefthook.yml`, `.gitattributes`, `.github/dependabot.yml` | byte-identical copies of the home masters, hash-proven before the write |
| `.gitignore` | the governed region rendered from the bound fragment list at the bound home pin, with the tenant's free project block preserved below the mark; unmarked existing content is never overwritten |
| `.github/CODEOWNERS` | the render of the canonical template with the manifest's owner values |
| The rule-sets conventions README | the render of the canonical template where the manifest binds the family |
| The toolchain config artifacts | where the manifest binds the toolchain section: byte-identical copies of the pinned territory artifacts (`configs/<family>/<category-id>/<base>`), each hash-proven against the bound hash before the write; the territory tree resolution mirrors the verifier's — the explicit `--territory-home` flag wins, a bound toolchain section without the flag fails closed |
| The pnpm workspace document | where the toolchain section is bound: the composed re-render whose governed keys carry the exact fortress-baseline values of the pinned territory and whose tenant keys — the catalog above all — are preserved; a fresh tenant receives the baseline with an empty catalog |

Tenant-authored data is never written: the binding manifest, the config
seam, the tooling module, and the license family (owned by the license-hub
CLI) stay the tenant's reviewable inputs. The composed pnpm workspace
re-render preserves the tenant's mapping keys, not comment formatting; the
surface's proof form is the policy invariants, never a byte identity.

## Provisioning versus verification

The provisioning CLI never re-implements a proof the verifier owns: every
copied and rendered byte is proven against the manifest's recorded hashes
before any write, and a provisioned tenant passes the conformance verifier
byte for byte — the identity is proven by the contract tests
`TestProvisionedTenantPassesTheConformanceVerifier` and
`TestProvisionedToolchainTenantPassesTheConformanceVerifier` in
`internal/packaging`. CI never provisions: the required check proves, the
tenant provisions locally at onboarding or re-binding; the write exposure
and the verify exposure share the render core and never fork it.
