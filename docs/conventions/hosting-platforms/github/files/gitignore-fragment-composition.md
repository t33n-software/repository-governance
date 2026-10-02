# gitignore fragment composition

This document is the **mechanism** surface of the canonical file family's
gitignore topic: how the governed region is rendered, how the conformance
verifier proves it, how the binding manifest records it, and how changes roll
out.

## The fragment tree

The tree lives at `hosting-platforms/github/files/gitignore/` and follows the
naming grammar of the convention: the org core `core.gitignore` at the topic
root, an ecosystem area as `<area>/core.gitignore`, and a concern fragment as
`<area>/<concern>.gitignore`. The registered fragments are discoverable by the
single glob `files/gitignore/**/*.gitignore`; there are no in-tree version
directories.

## The render

`internal/canonical.RenderGitignoreGovernedRegion` composes the governed
region of a tenant file at bind time, reading every bound fragment from the
home tree:

1. The fragment list is validated: the org core first, every name in the
   fragment grammar, no duplicates.
2. Every fragment is read from the home tree; an unreadable fragment fails
   the render.
3. A fragment carrying the project-block mark fails the render.
4. A pattern line restated across fragments fails the render (the overlap
   guard).
5. The org core is proven: no negation lines, and every canonical
   secret-artifact family present (the home's projection of the
   push-protection registry).
6. The composed region carries the generated header naming the source
   fragments and the home pin
   (`# canonical: gitignore <fragments> @ <home-pin> — governed region, do not edit`),
   the org core content, each subsequent fragment with its generated source
   header (`# <area>/<concern>`), and exactly one project-block mark at the
   end.
7. The composed region is evaluated against the canonical license-family
   probes with the Git-compatible pattern engine; a match fails the render.

The render is deterministic: the same fragment list at the same home pin
produces the same bytes.

## The verifier proofs

On every tenant pull request, `verifyGitignore` proves fail-closed:

1. the bound fragment list renders from the pinned home tree;
2. the rendered governed region's SHA-256 equals the bound hash;
3. the tenant file carries the rendered governed region as a verbatim prefix
   (the free project block lives below the mark);
4. where the tenant binds the license-hub class, the composed tenant file is
   evaluated against the canonical license-family probes (`LICENSE`,
   `NOTICE`, `LICENSES/`, and a content path below it) and any ignore match
   fails the proof.

## The binding form

The tenant binding manifest records the topic at schema version 2:

```json
"gitignore": {
  "path": ".gitignore",
  "fragments": ["core", "opentofu/core", "opentofu/lockfiles-committed"],
  "sha256": "<hash of the rendered governed region>"
}
```

The list is order-bearing and explicit: the core is always the first
fragment, and a binding that omits it fails closed. The schema document lives
at `schemas/repo-bindings/v2/repo-bindings.schema.json`; earlier major
versions stay published and immutable.

## Rollout

A core or fragment change is a home release event. The golden renders under
`conformance/gitignore/` prove every registered fragment set against the real
home tree on every home change, so an invalid composition can never leave the
home. Tenants bind the home by pin and move only through a reviewed
re-binding pull request whose conformance lane re-proves the re-rendered
file; until then a tenant remains valid on its pin.
