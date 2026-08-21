# GitHub composite action contract

This document binds the composite action family of the home. Composite actions
are the reusable building blocks for tenant lanes that need a controlled
environment outside the full workflow payloads.

## The action family

| Action | Path | Purpose |
|---|---|---|
| `setup-controlled-go` | `.github/actions/setup-controlled-go/action.yml` | Checkout with `persist-credentials: false`, pinned toolchain provisioning from the tenant's `go.mod` toolchain directive, and the controlled-toolchain assertion (with optional GOOS/GOARCH assertions) |
| `verify-canonical-files` | `.github/actions/verify-canonical-files/action.yml` | The composite shell that provisions the controlled toolchain and runs `cmd/verify-canonical` against the calling repository |

## Binding rules

1. Every action reference inside a composite action is a full-length commit
   SHA with a version comment; the `setup-controlled-go` reference inside
   `verify-canonical-files` pins the home commit that introduced the action
   and bumps with a home release through a reviewed change.
2. The approved-endpoint binding (`APPROVED_GO_PROXY_ENDPOINT` /
   `APPROVED_GO_SUMDB`) is a deferred seam: it lands in
   `setup-controlled-go` when the organization's own registry exists, as a
   home change with evidence. Until then the actions do not bind an endpoint
   (the documented, time-bounded public-proxy exception of the workflow family
   contract applies).
3. Composite actions carry no organization name, endpoint, or credential;
   tenant and organization values bind through inputs, the configuration seam,
   and organization or repository variables.
4. A tenant wires the "Canonical conformance" check by calling
   `verify-canonical-files` from a thin workflow; the check becomes a required
   context only after it has been proven on a real pull request to the exact
   target line.
