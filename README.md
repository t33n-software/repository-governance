# repository-governance

`repository-governance` is the canonical home of the repository surface of
the fleet. It owns everything a repository **carries**: the reusable workflow
payloads (`reusable-ci-go`, `reusable-codeql-go`, `reusable-dependency-review`,
`reusable-release-config`), the composite actions (`setup-controlled-go`,
`verify-canonical-files`), the
canonical file family (`.gitattributes`, `.gitignore`, `lefthook.yml`,
`dependabot.yml`, `CODEOWNERS`, and the rule-sets conventions README render
theme), the hash-pinned caller contracts, and the
conformance verifier (`cmd/verify-canonical`) — the proof of carrying.

## Artifacts

| Artifact | Path | Role |
|---|---|---|
| Reusable payloads | `.github/workflows/reusable-*.yml` | The class-A execution logic, referenced by full-length SHA |
| Composite actions | `.github/actions/` | The controlled-Go setup and the verifier shell |
| Canonical callers | `hosting-platforms/github/workflows/callers/go/` | The thin, hash-pinned tenant adoption shape |
| Canonical files | `hosting-platforms/github/files/` | The class-C content family (byte-identical or rendered) |
| Conformance verifier | `cmd/verify-canonical/` | The fail-closed proof-of-carrying engine |
| Verifier schemas | `schemas/{repo-bindings,caller-hashes}/v1/` | The versioned binding-manifest and hash-record contracts |
| Conformance vectors | `conformance/{positive,negative}/` | The proof set for the binding-manifest decoder |

## Consumption

A tenant adopts the surface by carrying the thin, byte-identical callers that
reference the payloads by full-length commit SHA, the canonical files, the
tool pins (`tools/go.mod`), the schema-validated configuration seam
(`git-governance.quality.json`), and the binding manifest
(`repo-bindings.json`) — never copied logic. The "Canonical conformance"
check proves the bindings fail-closed on every pull request.

## Verification

The home verifies itself (dogfooding): its own callers are byte-identical to
the canonical masters, and the canonical gate set runs against this
repository. See `docs/development/VERIFICATION.md` for the full contract and
`hosting-platforms/github/workflows/CONTRACT.md` for the workflow family
contract.

## Release lifecycle

This home adopts the centralized release and hotfix lifecycle family owned by
`t33n-software/git-governance` as byte-identical, hash-pinned callers under
`.github/workflows/` — never by copy. The family contract lives at
`workflows/github/CONTRACT.md` in that home; the caller pins and hashes are
bound in `workflows/github/callers/release-lifecycle/caller-hashes.json`.

The bound delivery variant is `github-only` (repository variable
`GIT_GOVERNANCE_DELIVERY_VARIANT`): a signed immutable tag plus a GitHub
release with a provenance-attested source manifest. The broker-bound hotfix
lanes fail closed in this variant until their evidence path is migrated (the
named deferral of the family contract). The lanes build the governance CLI
from the pinned class-D tool in `tools/go.mod` — never from a source checkout.
