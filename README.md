# repository-governance

`repository-governance` is the canonical home of the repository surface of
the fleet: the reusable workflow family (`reusable-ci-go`,
`reusable-codeql-go`, `reusable-dependency-review`), the composite actions,
the canonical files (`.gitattributes`, `.gitignore`, `lefthook.yml`,
`dependabot.yml`, `CODEOWNERS`), the hash-pinned callers, and the
conformance verifier (`cmd/verify-canonical`).

## Consumption

Tenant repositories carry only the thin, hash-verified callers that reference
the reusable workflows of this home by full-length commit SHA, the canonical
files, and the schema-validated configuration seam.

## Status

This repository is being established; the initial content lands through the
governed ticket workflow.
