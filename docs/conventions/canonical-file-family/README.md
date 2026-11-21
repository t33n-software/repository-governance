# Canonical file family conventions

This area carries the **model-level** conventions of the canonical file
family: the laws about what a fragment pattern may be and how the catalog of
fragments may change. These laws are ecosystem-neutral (they never define a
rule around one toolchain's artifacts) and channel-neutral (they bind
whatever hosting-platform realization renders the family).

The sibling areas split the remaining responsibilities:

- [`../hosting-platforms/`](../hosting-platforms/) carries the per-platform
  mechanics — how this organization's GitHub lane renders, binds, proves,
  and rolls out the composed governed regions.
- [`../verification/`](../verification/) carries the proof and drift
  architecture the composed regions are verified against.

## Conventions

| Document | Rule |
|---|---|
| [`fragment-catalog-discipline.md`](fragment-catalog-discipline.md) | Every fragment pattern stays inside the closed dialect-safe form set (brace expansion is forbidden), and every catalog change adds capability rows instead of deleting them — an adoption default is a preference, never a deletion. |

## Boundary

This area owns model-level admission law and nothing else. Composition
mechanics, verifier behavior, schema shapes, and per-ecosystem fragment
content are owned by their respective surfaces and never restated here. The
mechanics documents satisfy these laws; they do not define them.

The composition document under `../hosting-platforms/github/files/` carries
some layer-model narrative alongside its mechanics today. Narrowing it to
pure mechanics, with layer-model law read from this area, is a named
convergence candidate; it is not part of the current ticket, and no content
is relocated here as a side effect.
