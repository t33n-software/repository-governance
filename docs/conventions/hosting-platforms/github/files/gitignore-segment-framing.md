# gitignore segment framing

This document is the **design** surface of the canonical file family's
gitignore topic: how every fragment segment is framed, described, and read.
The composition mechanics (render, verifier proofs, binding form, rollout)
are owned by `gitignore-fragment-composition.md` and are not restated here;
this document binds only the human- and agent-readable shape of the fragment
content itself.

## Why framing exists

A governed region is committed into every tenant repository and is read by
people who never saw the home. A bare pattern line answers `what is ignored`
but not `why`, `what produces it`, or `what consumes it`. The frame makes
every segment self-explanatory at the reading site, so a reviewer can decide
about a pattern without opening an issue, and an agent can decide whether a
pattern belongs to its ecosystem without guessing. The layout is the
agent-facing chain-of-thought frame design applied to file content: the same
framed header, the same one-clause-per-line discipline, the same
heading/description separation.

## The frame anatomy

Every segment of every fragment file carries exactly this unit, in this
order, each part on its own physical line, every line a gitignore comment:

```text
# ╭───═══◎◎◎═══───
# <symbol> **<Segment title>**
# <Segment description: one sentence naming the topic and its shared reason.>
# ╰───═══◎◎◎═══───
#
# <pattern-a> — what: …; produced by: …; used by: …; why ignored: …
<pattern-a>
# <pattern-b> — what: …; produced by: …; used by: …; why ignored: …
<pattern-b>
```

1. **Frame top border:** the exact separator `╭───═══◎◎◎═══───`, prefixed by
   `# `. It marks the visible beginning of a segment and is never merged with
   the title, never replaced by `---`, and never carries content inside the
   border line itself.
2. **Lead line:** the segment's assigned symbol, then the segment title in
   Markdown bold markers. The symbol is unique per topic within the segment
   symbol table below.
3. **Description line:** one sentence stating what the segment groups and why
   its members share the ignore decision.
4. **Frame bottom border:** the exact separator `╰───═══◎◎◎═══───`, prefixed by
   `# `. It closes the framed header.
5. **Spacer line:** exactly one empty comment line (`#`) below the header.
6. **Pattern entries:** every single pattern line is immediately preceded by
   its own description comment line, and the description carries every facet:
   `what:` the artifact or surface is, `produced by:` which tool or command
   creates it, `used by:` which consumer reads it, `why ignored:` the reason
   it never becomes committed truth.

Two shapes are forbidden absolutely: a comment block followed by a pattern
block (grouped descriptions), and any compression that lets one description
stand for several patterns — even inside a homogeneous family. The reader
must see the full rubric at every single element, so the description and its
pattern interleave strictly one-to-one.

One pattern per line, no trailing whitespace, and no pattern without its own
description line directly above it. A pattern whose facets cannot be answered
is not yet ready to enter a fragment: the description is the admission
evidence.

## Segment rules

- Every fragment file — the org core, every `<area>/core`, and every
  `<area>/<concern>` — is built from framed segments only. Unframed pattern
  runs are forbidden.
- A segment groups one topic; a fragment carries as many segments as it has
  distinct topics. Segment order inside a fragment is stable and reads from
  general to specific.
- The generated region header and the generated `# <area>/<concern>` source
  lines come from the render and stay outside the fragment files; a fragment
  therefore begins directly with its first frame top border.
- The project-block mark never appears inside a fragment, and the frame
  borders are not the mark; both stay enforced by the render.
- Comments carry no ignore semantics, so framing cannot change what is
  matched; the composition invariants keep operating on pattern lines only.

## Segment symbol table

Symbols here are **file-content markers** of the gitignore segment plane.
They are not chain-of-thought lead symbols and are outside the COT
lead-symbol registry and its icon-uniqueness duty; uniqueness is required
within this table only.

| Symbol | Segment topic |
|---|---|
| 🏗️ | build and test outputs |
| 📤 | universal toolchain output suffix |
| 🖥️ | editor and operating-system state |
| 🔐 | secret-artifact guard |
| 🧩 | node toolchain artifacts |
| 📜 | auth-local registry credentials |
| 🚀 | VS Code extension delivery surfaces |
| ⚙️ | go toolchain artifacts |
| 🐍 | Python toolchain artifacts |
| 🧯 | OpenTofu engine-local surfaces |
| 🗄️ | infrastructure state |
| 🧷 | instance bindings |
| 📦 | lockfile policy statements |

A new segment introduces its symbol in the same change that adds the segment,
and the symbol is chosen once per topic: two segments never share one symbol.

## Rollout

Reframing an existing fragment changes its bytes and therefore every golden
render that binds it, so it is a home release event under the composition
document's rollout rule; tenants move through reviewed re-binding pull
requests at the new pin. The pattern set itself is unchanged by framing, so
no tenant gains or loses an ignore decision from a reframing release.
