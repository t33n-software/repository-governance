# Verification

This document is the verification contract of the `repository-governance`
home. Every gate is fail-closed: a missing or failing check is never a pass.

## The quality lane

The home is a tenant of the fleet quality lane. The canonical gate set runs
through the pinned `quality-gate` tool — identically locally, in the Lefthook
pre-push lane, and in CI through the home's own caller:

```bash
go tool -modfile tools/go.mod quality-gate
```

The gate set covers the controlled toolchain assertion, module and tool
verification, formatting, lint, typecheck, the unit and contract tests, the
race detector, static analysis, vulnerability analysis, the boundary fuzz
lane (`FuzzDecodeBindings`), the Lefthook configuration validation, the
build-and-smoke of `cmd/verify-canonical`, and the in-process exact
100.0-percent statement-coverage gate.

## The conformance verifier

`cmd/verify-canonical` is the home's proof-of-carrying engine, consumed by
tenants as a tool pin and wired as the required check "Canonical
conformance":

```bash
go tool -modfile tools/go.mod verify-canonical --repo .
```

The flag parser accepts both the space-separated form shown above and the
`--repo=<path>` assignment form; a flag without a value is a usage error.

The verifier is environment-self-sufficient: it provisions every module it
reads (its own home module and the canonical catalog module) through the
tenant's integrity-pinned tooling channel and never trusts a warm module
cache, so it runs identically in cold-cache CI lanes. The bound mechanism
and its regression evidence are the convention
`docs/conventions/verification/self-sufficient-module-resolution.md`.

On a tenant pull request it proves, fail-closed:

1. every declared caller file's SHA-256 equals the bound hash, the canonical
   master's hash, and the home's published caller-hashes record, and the
   caller references the payload by the bound full-length home SHA;
2. the tenant's `git-governance.quality.json` strictly decodes and carries the
   pinned schema version (the semantic gate rules are owned by the producer
   home that executes the gate);
3. the tenant's `go.mod` carries an explicit, well-formed `toolchain`
   directive — the Go-native selector the payloads and the controlled-Go
   setup action resolve fail-closed and install exactly through
   `go-version` (never the `go` directive, never the latest patch);
4. the canonical file family matches the bound hashes — byte-identical for
   `lefthook.yml`, `.gitattributes`, and `dependabot.yml`, and the canonical
   core as a verbatim prefix for `.gitignore`;
5. `.github/CODEOWNERS` is the exact materialization of the canonical template
   with the manifest's values;
6. the canonical tool catalog carries its bound `$schema` identity — asserted
   against the canonical catalog schema identity, never dereferenced — and
   every tool pin in the tenant's tooling module is admitted by the catalog or
   is the home's own verifier tool;
7. where the license-hub family is bound, the license binding values and lock
   exist and decode as JSON documents, and the verifier orchestrates the
   tenant-pinned `license` CLI through the tenant's tooling module: the locked
   template is resolved inside the pinned license-hub module directory, and
   the pinned tool proves pin integrity, drift freedom, and completeness of
   the committed instance fail-closed — the verification semantics live
   exactly once in the license hub and are never re-implemented by the
   verifier.

The tenant's binding manifest (`repo-bindings.json`) is strictly decoded
against `schemas/repo-bindings/v1/repo-bindings.schema.json`; the home's
published caller hashes follow
`schemas/caller-hashes/v1/caller-hashes.schema.json`; the
`conformance/{positive,negative}/` vectors prove every acceptance and every
rejection of the manifest decoder. Every schema document carries the
`.schema.json` suffix — the canonical naming convention for schema files.

## The contract-test set

`internal/packaging/` carries the one canonical contract-test set. On every
home change it proves: the payloads carry only `on: workflow_call`, every
action reference is a full-length commit SHA with a version comment, the
permission matrix per lane, the absence of forbidden patterns
(`pull_request_target`, workflow-level `GOFLAGS`, `cache: true`), the
exact pinned toolchain provisioning (the fail-closed resolution step reads
the `toolchain` directive of the tenant's `go.mod` and `go-version` installs
exactly that version — never `go-version-file`, no JSON extraction shim) and
the gate job names of the payloads, the callers'
four-shared-line trigger coverage and exact job names, the byte identity
between the home's own callers and the canonical masters, the caller-hashes
record against the recomputed master content, the canonical file family, the
CODEOWNERS template and its materialization, the schema conformance, the
conformance vectors, the home's own binding manifest self-consistency, the
home's own `go.mod` toolchain directive, and the absence of legacy artifacts
(`_BAK` files, JSON payload copies in `docs/`).

## Whitebox testing

Every production path carries a direct same-package whitebox test for its
invariants, branches, state transitions, errors, and cleanup paths. The
binding-manifest decoder carries the boundary fuzz lane
(`FuzzDecodeBindings`).
