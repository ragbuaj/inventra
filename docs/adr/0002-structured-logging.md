# ADR-0002 — Structured logging: log/slog + request-id correlation

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-26 |
| **Deciders** | Ragil (owner) |
| **Maps to** | Backlog #10 |

## Context and problem statement

Logging is currently Gin's default request logger. The PRD non-functional requirements call for
**structured logging** and basic observability. A bank-grade system also needs **traceable** logs:
correlating a frontend action to its backend request and downstream effects, without leaking sensitive
fields. We need a logging approach for both backend and frontend.

## Decision drivers

- Structured (JSON) output, queryable; stable field schema.
- Request correlation **FE ↔ BE** (one id threads through a request's logs).
- Minimal dependencies; future-proof toward OpenTelemetry without committing to it now.
- Never log secrets (`password_hash`, tokens, Google ids) — aligns with field-permission sensitivity.

## Considered options

1. **stdlib `log/slog`** (Go 1.21+; project is on 1.25) — structured, handler-swappable, zero deps.
2. **`rs/zerolog`** — very fast, ergonomic, but an external dep and its own API.
3. **`uber-go/zap`** — powerful/fast, heavier API and config surface; external dep.

## Decision outcome

**Chosen: Option 1 — `log/slog`.**

- **Backend:** a JSON handler in production, a human-readable text handler in dev (chosen via config,
  ADR-0003). A `slog.Logger` is created at startup and injected; handlers/services log via a logger
  pulled from `context.Context`.
- **Request correlation:** middleware reads an inbound `X-Request-ID` (propagated by the frontend) or
  generates one, stores it on the context, echoes it in the response header, and binds it as a `slog`
  attribute so **every** log line for that request carries `request_id`. Include `user_id`/`role_id`
  (from `RequireAuth`) where available — but **never** sensitive values.
- **Frontend:** a thin `useLogger` composable that generates/propagates the `X-Request-ID` per API call
  and ships client errors to a backend log endpoint (or console in dev). No heavy client logging lib.

Reject zerolog/zap: `slog` is the standard, dependency-free, and sufficient; its handler interface lets
us bridge to OpenTelemetry later without changing call sites.

## Consequences

- 👍 Zero new backend deps; standard API; swappable handler (text→JSON→OTel) without touching call sites.
- 👍 End-to-end correlation via a single `request_id`.
- 👎 `slog` is slightly less ergonomic than zerolog and not the fastest → acceptable; logging is not the
   hot path, and the JSON handler is fine for our volume.
- 👎 Requires discipline to never log sensitive fields → enforce via a small `safeAttrs` helper / review.

## Implementation notes

- Replace Gin's default logger with a slog-backed middleware (method, path, status, latency, request_id).
- Define a stable attribute set; add a redaction helper for sensitive keys.
- Keep `/health` noise low (skip or sample).
