# Architecture Decision Records (ADR)

This directory records **significant technical decisions** for Inventra — what was chosen, the
options weighed, and the trade-offs accepted. Format: lightweight [MADR](https://adr.github.io/madr/).

Decisions follow a deliberate principle for this project: **prefer industry standards & best
practice over portfolio-differentiation shortcuts** — use mature, battle-tested libraries unless the
domain genuinely needs custom, and document the trade-off either way.

## Conventions

- One decision per file: `NNNN-short-title.md` (zero-padded, monotonically increasing).
- **Status**: `Proposed` → `Accepted` → (`Superseded by ADR-XXXX` / `Deprecated`). Never edit an
  Accepted ADR's decision; supersede it with a new ADR and link both ways.
- Keep it short: context, options, decision, consequences. Link to PRD/DATABASE where relevant.

## Index

| ADR | Decision | Status | Maps to |
|---|---|---|---|
| [0001](0001-go-testing-stack.md) | Go testing: `testify` + `testcontainers-go` (keep stdlib runner) | Accepted | backlog #9 |
| [0002](0002-structured-logging.md) | Structured logging: stdlib `log/slog` + request-id correlation (FE↔BE) | Accepted | backlog #10 |
| [0003](0003-configuration.md) | Configuration: `caarlos0/env` + `.env` (not Viper) | Accepted | backlog #11 |
| [0004](0004-rate-limiting.md) | Rate limiting: Redis-backed limiter on auth + global throttle | Accepted | backlog #12 |
| [0005](0005-authorization-build-vs-buy.md) | Authorization: keep the custom 3-layer model (vs Casbin/OpenFGA) | Accepted | backlog #1 |
| [0006](0006-map-library.md) | Map: keep Leaflet + OSM (vs MapLibre/Google) | Accepted | backlog #2 |
| [0007](0007-frontend-api-composable-convention.md) | Frontend API composables: module subfolders + English snake_case DTOs | Accepted | discovered (frontend) |
| [0008](0008-backend-masterdata-module-convention.md) | Backend masterdata: 4-file split (dto/service/handler/routes) per resource | Accepted | backlog #3 |
| [0009](0009-third-party-signin.md) | Third-party sign-in: `oauth2` + `go-oidc` (not goth) | Accepted | backlog #4 |
| [0010](0010-background-job-execution.md) | Background job execution: staged adoption (contract-first, pluggable trigger) | Accepted | depreciation module |
| [0011](0011-observability.md) | Observability: self-hosted Prometheus/Grafana/Loki + Alertmanager→Telegram | Accepted | ops hardening — Phase 3 |
| [0012](0012-waf.md) | WAF: Coraza + OWASP CRS as a Caddy module (DetectionOnly → Blocking rollout) | Accepted | ops hardening — Phase 1 |
| [0013](0013-iac.md) | Infrastructure as Code: Ansible (`base`+`docker`+`app` roles, containerized tooling, Vault secrets) | Accepted | ops hardening — Phase 2 |
| [0014](0014-notification-delivery.md) | Notification delivery: transactional outbox (Postgres) + Redis Streams transport; supersedes PRD A1b (Redis is transport, not the notification store) | Accepted | notification module |

> ADRs are **decisions**, not implementation. Code lands in follow-up work; each ADR notes the libraries
> and the integration points. ADR-0007's refactor (folder regroup + field-key rename) is tracked as
> follow-up work — see PROGRESS.md.
