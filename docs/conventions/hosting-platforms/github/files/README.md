# File-family conventions

This area carries the binding conventions for the canonical file family of
this home (`hosting-platforms/github/files/`): how composed topics are
rendered, proven, bound, and rolled out.

The model-level admission laws that bind what a fragment pattern may be and
how the fragment catalog may change live in the sibling area
[`../../../canonical-file-family/README.md`](../../../canonical-file-family/README.md);
the mechanics documents here satisfy those laws and do not define them.

## Conventions

| Document | Rule |
|---|---|
| `gitignore-fragment-composition.md` | The gitignore topic is composed at bind time from the bound fragment list and proven byte-exact by the conformance verifier. |
| `gitignore-segment-framing.md` | Every gitignore fragment segment is framed (border, symbol, title, description) and every single pattern line carries its own full-facet description line directly above it. |
| `provision-canonical.md` | The tenant surfaces are provisioned by the home's provisioning CLI from the bound manifest; the write exposure shares the render core with the verifier and never re-implements a proof. |
