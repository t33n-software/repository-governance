# repository-governance

`repository-governance` is the canonical home of the repository surface of
the fleet. It owns everything a repository **carries**: the reusable workflow
payloads (`reusable-ci-go`, `reusable-codeql-go`, `reusable-dependency-review`),
the composite actions (`setup-controlled-go`, `verify-canonical-files`), the
canonical file family (`.gitattributes`, `.gitignore`, `lefthook.yml`,
`dependabot.yml`, `CODEOWNERS`), the hash-pinned caller contracts, and the
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
