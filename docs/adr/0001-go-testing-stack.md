# ADR-0001 — Go testing stack: testify + testcontainers-go

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-26 |
| **Deciders** | Ragil (owner) |
| **Maps to** | Backlog #9 |

## Context and problem statement

The backend currently tests with the stdlib `testing` package only. As feature modules grow (asset
core, approval chains, dual-basis depreciation, scope enforcement), we need (a) **readable assertions**
to keep table-driven tests legible, and (b) **integration tests against real Postgres & Redis** —
scope/field-permission enforcement and sqlc queries are exactly the things mocks hide bugs in. CLAUDE.md
already mandates broad, real-behavior test coverage; the tooling should support that.

## Decision drivers

- Idiomatic Go (don't import a foreign testing paradigm).
- Real dependencies for integration (data-scope, partial-unique indexes, triggers behave like prod).
- CI already runs Docker (the e2e job), so containers are available.
- Keep unit tests fast; isolate slow integration tests.

## Considered options

1. **Stdlib `testing` only** — zero deps; but verbose assertions and hand-rolled DB fixtures, or
   mock-heavy tests that don't exercise real SQL/scope behavior.
2. **`testify` + `testcontainers-go`** — `require`/`assert` for assertions; ephemeral real
   Postgres/Redis containers for integration.
3. **Ginkgo + Gomega (BDD)** — expressive, but a non-idiomatic DSL that diverges from standard Go
   tooling and raises the contributor learning curve.

## Decision outcome

**Chosen: Option 2.** Keep the stdlib `go test` runner and table-driven style; add:

- **`github.com/stretchr/testify`** — use `require` (fatal) for preconditions and `assert` for
  follow-on checks. Use `testify/mock` **sparingly**, only for true external boundaries.
- **`github.com/testcontainers/testcontainers-go`** (+ `modules/postgres`, `modules/redis`) — spin
  real Postgres/Redis for integration tests; run migrations against the container, exercise sqlc
  queries and scope/field-permission paths end-to-end.

Integration tests are gated behind a build tag (`//go:build integration`) so the default `go test ./...`
stays fast; CI runs an explicit integration pass. Reuse a single container per package (suite setup) to
amortize startup.

Reject Ginkgo/Gomega: BDD value doesn't justify leaving idiomatic Go for this team/codebase.

## Consequences

- 👍 Legible assertions; integration tests catch scope/SQL/index bugs that mocks miss.
- 👍 No paradigm shift — still `go test`, still table-driven.
- 👎 testcontainers needs a Docker daemon and adds seconds of startup → mitigated by the `integration`
  build tag, container reuse, and parallelizing where safe.
- 👎 One more way to write tests (testify mock vs interfaces) → guidance: prefer real deps via
  testcontainers over mocks; mock only external services.

## Implementation notes

- Add deps; create a `internal/testsupport` helper for container bootstrap + migration apply.
- CI: a dedicated job (or step) runs `go test -tags=integration ./...`; unit job stays untagged.
- Keep CLAUDE.md's "assert real behavior" rule — no hollow assertions.
