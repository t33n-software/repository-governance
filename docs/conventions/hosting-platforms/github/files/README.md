# File-family conventions

This area carries the binding conventions for the canonical file family of
this home (`hosting-platforms/github/files/`): how composed topics are
rendered, proven, bound, and rolled out.

## Conventions

| Document | Rule |
|---|---|
| `gitignore-fragment-composition.md` | The gitignore topic is composed at bind time from the bound fragment list and proven byte-exact by the conformance verifier; the rule is owned by the knowledge plane. |
| `provision-canonical.md` | The tenant surfaces are provisioned by the home's provisioning CLI from the bound manifest; the write exposure shares the render core with the verifier and never re-implements a proof. |
