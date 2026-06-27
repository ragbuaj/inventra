# ADR-0004 — Rate limiting: Redis-backed limiter on auth + global throttle

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-26 |
| **Deciders** | Ragil (owner) |
| **Maps to** | Backlog #12 · PRD FR-1.8 |

## Context and problem statement

The PRD (FR-1.8) requires **rate limiting**: anti-brute-force on authentication and throttling of
sensitive endpoints. Redis is already a first-class dependency (token store, caching, scope cache), and
the system is meant to run as a bank-grade service that may scale to **multiple API instances** — so the
limiter must be consistent across instances, not per-process.

## Decision drivers

- Distributed correctness: limits must hold across N API replicas (shared counter).
- Reuse existing infrastructure (Redis) rather than add a new component.
- Protect auth first (`/auth/login`, `/auth/refresh`, password-reset), then a general per-IP throttle.
- Standard semantics: `429 Too Many Requests` + `Retry-After` / rate-limit headers.
- Graceful behavior if Redis is briefly unavailable (Redis is "complementary, not source of truth").

## Considered options

1. **In-memory token bucket** (`golang.org/x/time/rate`) — simple, fast, but **per-instance**; limits
   leak under horizontal scaling and reset on deploy. Inadequate as the primary control.
2. **Redis-backed limiter** (e.g. `go-redis/redis_rate`, GCRA) — distributed, atomic via Lua, pairs with
   the existing `go-redis` client; sliding-window/GCRA smoothing.
3. **Gateway/proxy-level** (nginx/Traefik/API gateway) — offloads from the app, but pushes
   account-aware logic (per-email login limits) out of the app where it can't see auth context.

## Decision outcome

**Chosen: Option 2 — Redis-backed limiter.**

- Use a Redis limiter (`github.com/go-redis/redis_rate/v10`, GCRA) on top of the existing `go-redis`
  client.
- **Auth endpoints:** strict limits keyed by **IP + account** (e.g. login attempts/min) to blunt
  brute-force and credential stuffing; password-reset/verification endpoints throttled per identifier.
- **Global:** a looser per-IP throttle as middleware on the API.
- Responses: `429` + `Retry-After` and `RateLimit-*` headers; log limit hits (ADR-0002) with
  `request_id`, **without** logging credentials.
- **Resilience (fail-open with caution):** if Redis errors/times out on the limiter path, allow the
  request but log a warning; pair with a small in-memory `x/time/rate` backstop on auth so protection
  degrades rather than disappears. A short Redis timeout keeps the hot path responsive.

Reject in-memory-only (not distributed) as the primary control; reject gateway-only (loses
account-aware auth limits). Either may complement this later.

## Consequences

- 👍 Consistent limits across instances; reuses Redis; standard 429 semantics; protects the most-abused
   endpoints first.
- 👍 Tunable bands (config, ADR-0003) without redeploy logic changes.
- 👎 Adds Redis to the auth hot path → mitigated by short timeouts + fail-open + in-memory backstop.
- 👎 Fail-open means a Redis outage weakens (not removes) protection → acceptable per the "Redis is
   complementary" stance; alert on limiter errors.

## Implementation notes

- Add the limiter middleware; make limits configurable (per-route bands) via config/`app_settings`.
- Order: limiter runs **before** `RequireAuth` for login; account-keyed limit applied post body-parse.
- Add tests (ADR-0001): unit for the limiter wrapper, integration (testcontainers Redis) for real counts.
