# hosting-platforms/github

The GitHub area of the `repository-governance` home. Everything a repository
carries that GitHub resolves or requires physically lives here exactly once:
the reusable workflow payloads, the composite actions, the hash-pinned
canonical callers, and the canonical file family.

## Layout

| Path | Content |
|---|---|
| `workflows/CONTRACT.md` | The binding contract of the workflow family: trigger surface, permission model, check-context model, binding seams, pin lifecycle |
| `workflows/callers/go/` | The canonical, hash-pinned callers (`ci.yml`, `ci-full.yml`, `codeql.yml`, `dependency-review.yml`) plus their `caller-hashes.json` record |
| `actions/CONTRACT.md` | The binding contract of the composite action family |
| `files/gitattributes/.gitattributes` | The byte-identical line-ending contract |
| `files/gitignore/.gitignore` | The canonical core block plus the marked project-block convention |
| `files/lefthook/lefthook.yml` | The canonical hook core (calls only the Git CLI); bound blocks compose at render time |
| `files/dependabot/dependabot-go.yml` | The canonical Dependabot variant for the Go ecosystem class |
| `files/codeowners/CODEOWNERS.tmpl` | The ownership template; values render from the tenant's binding manifest |

The payloads and composite actions themselves live at the platform-enforced
execution locations (`.github/workflows/`, `.github/actions/`), because GitHub
reads no subfolders; the contract-test set in `internal/packaging` binds the
addresses to this contract.

## Adoption

A tenant adopts the surface by carrying the thin callers, the canonical files,
the tool pins, and the schema-validated configuration seam — never copied
logic. The binding manifest (`repo-bindings.json`) records the tenant's pins,
and the "Canonical conformance" check proves them fail-closed on every pull
request.
