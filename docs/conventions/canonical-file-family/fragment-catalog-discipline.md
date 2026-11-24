# Fragment catalog discipline

This document is the **admission** surface of the canonical file family: the
two model-level laws that bind every fragment pattern and every catalog
change. The laws are ecosystem-neutral and channel-neutral. The composition
mechanics — bind-time rendering, verifier proofs, binding form, rollout —
live in
`../hosting-platforms/github/files/gitignore-fragment-composition.md` and
are not restated here.

## Why the family separates admission from mechanics

An ignore surface fails open. A pattern that does not match what its author
intended does not error; it silently commits exactly the artifact it was
written to guard. A catalog row that disappears removes a capability for
every consumer that still binds it, and no render step can detect that. Both
failures happen **before** composition: they are defects of what was
admitted, not of how the admission was rendered. The family therefore binds
admission separately from mechanics.

## Law 1 — Pattern portability (closed dialect-safe form set)

The grammar the governed region is written in is the version-control ignore
grammar: Git evaluates it, and the home's verifier proves the composed
region with a Git-compatible pattern engine. A fragment pattern is lawful
only when it uses a form that grammar defines, chosen so its meaning cannot
drift in any other ignore-style dialect the shared vocabulary feeds. The
closed form set:

1. **Path class** — a path or directory prefix, optionally with `/**`:
   `scripts/**`, `node_modules/`. The rule is expressed by where the
   artifact lives, not by what its filename ends with. This is the portable
   form for "every build surface of every stack under this path", which is
   why the tooling directories need no extension naming at all.
2. **Starred contract name** — one contract filename and one star:
   `rolldown.config.*`. The star covers every extension the contract may
   carry; the extension never enters the rule.
3. **Extension class** — one extension as the entire rule: `*.out`,
   `**/*.ts`. Lawful where the extension itself is the artifact family.
4. **Exact name** — one literal: `.npmrc`, `auth.ini`, `crash.log`.
5. **Character class inside a name** — one bracketed letter set:
   `*.py[cod]`. Lawful because the grammar defines bracket classes
   identically: it enumerates single letters within one name, not complete
   filename alternatives.

**Forbidden absolutely: brace expansion.** A pattern shaped `*.{js,mjs}` or
`{dist,build}/` does not enter any fragment. The version-control ignore
grammar has no brace alternation: it matches the braces literally, against a
file whose name contains them. The intended exclusion silently matches
nothing — a fail-open guard indistinguishable from working — while glob
dialects that do expand the same text give it a different meaning. A pattern
whose reading depends on which consumer parses it breaks the family's core
promise, that the governed region is one proven artifact across every
surface its vocabulary is shared with.

Where a brace pattern seems needed, a closed form already covers the same
set: the files differ only by extension under one directory → path class;
one contract filename in any extension → starred contract name; genuinely
different filenames → separate exact-name or starred-contract lines. The
choice is made without naming any ecosystem: the rule is a path or a
contract, never an extension list.

## Law 2 — Capability completeness (adoption-agnostic catalogs)

A fragment catalog enumerates what a consumer may build with. It is
capability-complete and adoption-agnostic: the organization's current
default for new projects is an adoption posture recorded in the
dependency-selection layer, and an adoption posture is never a deletion.

1. **A default change adds a row; it never deletes or substitutes one.**
   When the organization adopts a new engine, the catalog gains that
   engine's concern fragment; the previous engine's fragment stays listed,
   because existing consumers still bind it and their governed regions still
   prove byte-exact against the pinned home stand. Replacing one
   capability's row with another row silently breaks every binding that
   still names the removed row.
2. **A row leaves the catalog only through a supersession decision.**
   Removal requires a reviewed decision recording that the capability itself
   ended — the artifact class no longer occurs, or the last consumer moved —
   with the binding list re-proven empty of the row. "The organization
   stopped adopting it" is not a supersession reason.
3. **The precedent governs the pattern.** The package-manager axis of the
   catalog carries one concern row per supported manager while exactly one
   manager is the organization default. The engine axis follows the same
   shape, and so does every future adoption axis: rows enumerate capability,
   the default records preference, and neither operation touches the other.

## Enforcement mapping

The render and the verifier already fail closed on the **structural**
composition invariants: unknown or repeated fragments, a mark inside a
fragment, a pattern restated across fragments, a negation in the org core, a
missing secret-artifact family, a license-family match. The two laws above
bind **content** and **catalog lifecycle** before composition; they are
reviewed admission — every diff into the fragment tree and every catalog
change is checked against this document. A mechanical brace detector in the
render is a possible future guard; it is not claimed here, and its absence
does not weaken the admission rule.

## Author checklist for a fragment or catalog change

1. Does every new pattern fit one of the five closed forms — no braces, no
   extension list where a path class or a starred contract name covers the
   same files?
2. Does every new pattern line carry its full facet description under the
   framing convention?
3. Does a changed default add a row and leave every surviving capability's
   row intact?
4. Is every row removal backed by a recorded supersession decision with the
   binding list re-proven empty of the row?
5. Do the structural composition proofs still pass — admission is reviewed
   here, composition is proven by the render?

## Boundary

This document owns the admission law for patterns and catalog lifecycle. It
owns no render mechanics, no schema, no verifier behavior, no per-ecosystem
fragment content, and no adoption preference itself — the preference belongs
to the dependency-selection layer. Where this document references a
mechanics surface it re-references the owning document and never restates
it.
