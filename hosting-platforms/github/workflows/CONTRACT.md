# GitHub workflow family contract

This document is the binding contract of the reusable workflow family and its
callers. It owns the trigger surface, the permission model, the check-context
model, the binding seams, and the pin lifecycle. The payloads live at the
platform-enforced execution location `.github/workflows/`; the hash-pinned
canonical callers live at `hosting-platforms/github/workflows/callers/go/`.

## The payload family

| Payload | Capability | Typed inputs |
|---|---|---|
| `.github/workflows/reusable-ci-go.yml` | The canonical Go quality lane | `quality_class` (`linux-only` or `full`) |
| `.github/workflows/reusable-codeql-go.yml` | The canonical Go CodeQL analysis lane | none |
| `.github/workflows/reusable-dependency-review.yml` | The dependency admission review | none |

Every payload carries exclusively `on: workflow_call` and never triggers
itself. Every action reference inside a payload is a full-length commit SHA
with a version comment; a bump is a home release event with evidence, followed
by a fleet pin bump through a reviewed pull request. No payload carries an
organization name, an endpoint, a credential, workflow-level `GOFLAGS`,
`cache: true`, or a `pull_request_target` trigger.

## The trigger surface

The callers trigger on push and pull request to `main`, `develop`,
`release/**`, and `support/**`, plus a schedule and `workflow_dispatch`
(`dependency-review.yml` is pull-request-native). Trigger completeness is a
precondition for the first release cut: the shared-line rulesets are
pre-positioned and bind every future matching ref from its first commit, so a
missing trigger family would block the first pull request to that line
fail-closed.

## The permission model

The caller carries `permissions: {}` at workflow level (default deny) and
grants the callee's least-privilege set explicitly on the calling job. A called
workflow's token is capped by the calling job's permissions and can never be
elevated beyond it; a caller that grants nothing fails the callee's run. The
bound grants:

| Caller | Calling-job grant | Callee declares |
|---|---|---|
| `ci.yml` / `ci-full.yml` | `contents: read` | `contents: read` |
| `codeql.yml` | `actions: read`, `contents: read`, `security-events: write` | the same three |
| `dependency-review.yml` | `contents: read` | `contents: read` |

## The check-context model

A reusable-workflow call surfaces its checks under the composite context
`<caller job name> / <callee job name>`. The naming law of the composed era:
the caller job carries the lane identity (what the lane is), and the callee
job carries the gate or variant identity (what the gate does). The bound names
are contract, because the shared-line rulesets bind the exact strings:

| Caller | Caller job name (lane) | Callee job name (gate) | Emitted contexts |
|---|---|---|---|
| `ci.yml` | `Quality gates` | the matrix variant | `Quality gates / linux-amd64` |
| `ci-full.yml` | `Quality gates` | the matrix variant | `Quality gates / linux-amd64`, `Quality gates / macos-arm64`, `Quality gates / windows-amd64` |
| `codeql.yml` | `CodeQL` | `CodeQL (go)` | `CodeQL / CodeQL (go)` |
| `dependency-review.yml` | `Dependency review` | `Dependency admission review` | `Dependency review / Dependency admission review` |

The CodeQL merge gate consumes the SARIF result through the `code_scanning`
ruleset rule (tool-bound, not context-bound); the status-check contexts of the
other lanes are bound in the organization rulesets. A ruleset binds a context
only after the lane has proven it on a real pull request to the exact target
line — the activation order is validated sequencing, never assumption.

## The binding seams

- **Toolchain version.** The payloads and the controlled-Go setup action
  provision the pinned toolchain from the tenant's `go.mod` — the Go-native
  selector that `actions/setup-go` reads through `go-version-file: go.mod`
  (the `toolchain` directive, with the `go` directive as the platform
  fallback). The configuration seam's `toolchain.goVersion` remains the
  assertion authority: the quality gate cross-checks the running toolchain
  against it fail-closed, and the conformance verifier proves the tenant's
  `go.mod` carries the directive. The toolchain identity is tenant data at
  its native place — never a workflow edit, never a JSON extraction shim.
- **Quality gate.** The CI payload runs `go tool -modfile tools/go.mod
  quality-gate`; the tool pin resolves through the tenant's tooling module
  (class-D consumption).
- **Approved Go endpoint (deferred).** The fail-closed
  `APPROVED_GO_PROXY_ENDPOINT` / `APPROVED_GO_SUMDB` binding step is deferred
  until the organization's own registry exists. Until then the lanes resolve
  modules through the public proxy as a documented, time-bounded exception;
  this deferral is the only sanctioned deviation and it ends with the
  registry's go-live, when the bind step lands in the payloads as a home
  change with evidence.
- **Builder image (evolution path).** The governed, digest-pinned builder
  image supersedes runner-based provisioning when the builder authority lane
  is live; the toolchain assertion step then proves the image-provided
  toolchain.

## The caller contract

A tenant adopts a lane by carrying the byte-identical caller from
`hosting-platforms/github/workflows/callers/go/`. The caller references the
payload by full-length commit SHA; the version comment lands with the first
home release pin. Any deviation from the canonical caller is a hash mismatch
and blocks fail-closed in the "Canonical conformance" check. A tenant whose
class disables a capability carries no caller for it — the state is a named
class in the fleet registry, never a renamed or disabled file.

## Dogfooding

This home is a tenant of its own callers: `.github/workflows/ci.yml`,
`codeql.yml`, and `dependency-review.yml` are byte-identical to the canonical
masters. The contract-test set in `internal/packaging` proves the identity on
every home change.
