# Verification conventions

This area carries the binding conventions for the conformance verifier
(`cmd/verify-canonical`) and every governed proof component of this home:
how proofs obtain their inputs, how the module-cache boundary is managed,
and how the regression evidence for these guarantees is constructed.

## Conventions

| Document | Rule |
|---|---|
| `self-sufficient-module-resolution.md` | A governed verification component provisions every module it reads through the integrity-pinned tooling channel and never trusts a warm module cache. |
| `repo-bindings-manifest-form.md` | The tenant binding manifest is the one trust boundary between fleet and tenant: an agnostic form with language-conditional values, an optional territory section, and a two-surface declaration discipline. |
