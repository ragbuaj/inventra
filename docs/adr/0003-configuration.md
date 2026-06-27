# ADR-0003 — Configuration: caarlos0/env + .env (not Viper)

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-06-26 |
| **Deciders** | Ragil (owner) |
| **Maps to** | Backlog #11 |

## Context and problem statement

Configuration is sourced from environment variables (12-factor): `DATABASE_URL`, Redis, MinIO, JWT,
Google OAuth, `NUXT_PUBLIC_API_BASE`, etc., with `.env` for local dev and Docker env in containers.
The question raised was whether to adopt **Viper**. We want a typed, validated config without
over-engineering.

## Decision drivers

- 12-factor: env-first, works cleanly in Docker/CI.
- Typed config struct, validated at startup, **fail fast** on missing/invalid values.
- Minimal surface — we do **not** need multi-format files, remote config stores, or live reload.

## Considered options

1. **Hand-rolled `os.Getenv` parsing** (current `internal/config`) — no deps, but boilerplate for
   defaults, required-checks, and type parsing as config grows.
2. **`caarlos0/env`** — struct-tag binding of env → typed fields, with defaults/required, plus
   `joho/godotenv` to load `.env` in dev.
3. **Viper** — powerful (files, env, flags, remote, watch) but heavy for an env-only app; pulls a large
   dependency tree and config indirection we won't use.

## Decision outcome

**Chosen: Option 2 — `caarlos0/env` (+ `joho/godotenv`).**

- Keep a single typed `Config` struct; bind via `env:"..."` tags with `envDefault` and `required`.
- Load `.env` in local/dev only (guarded), then parse env over it.
- **Validate at startup** and fail fast with a clear message listing missing/invalid keys.
- Frontend keeps using Nuxt `runtimeConfig` (`NUXT_PUBLIC_API_BASE`) — unchanged.

Reject Viper: its strengths (layered files, hot-reload, remote KV) are non-goals here; it would add
weight and indirection without payoff. Revisit only if we later need layered file-based config.

## Consequences

- 👍 Minimal, typed, 12-factor; less boilerplate than hand-rolled; obvious failure on misconfig.
- 👍 Small dependency footprint.
- 👎 No live reload / file layering → not needed; if that changes, supersede this ADR.
- 👎 Struct tags are a (mild) form of magic → mitigated by keeping one well-documented Config struct.

## Implementation notes

- Refactor `internal/config` to the struct-tag approach; centralize defaults and required keys.
- Document every key in `.env.example` (already present) — keep it the source of truth for ops.
- Never commit real secrets; `.env` stays git-ignored.
