# Inventra API Documentation

**Spec-first OpenAPI 3.1.** The contract is the source of truth.

## Files
| Path | Purpose |
|---|---|
| `backend/api/openapi.yaml` | Canonical OpenAPI 3.1 spec (embedded & served by the API) |
| `backend/api/scalar.standalone.js` | Vendored Scalar viewer (self-hosted, no CDN) |
| `docs/api/bruno/` | Bruno collection for manual testing (git-tracked) |
| `.spectral.yaml` | Spectral lint ruleset |

## View the docs
Run the backend, then open:
- **Interactive docs (Scalar):** http://localhost:8080/docs
- **Raw spec:** http://localhost:8080/openapi.yaml

## Lint the spec
```bash
npx @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml
```
CI runs this on every push/PR (job `api-docs`).

## Manual testing with Bruno
1. Install [Bruno](https://www.usebruno.com/).
2. Open collection folder `docs/api/bruno/`.
3. Select the **Local** environment (`baseUrl = http://localhost:8080`).
4. Run **Auth › Login** — it stores `accessToken` / `refreshToken` as env vars; the
   other Auth requests reuse them automatically.

Seed a user first (dev): `go run ./cmd/createadmin -email admin@inventra.local -password admin12345` (from `backend/`).

## Definition of Done (per endpoint/module)
Every new or changed endpoint MUST:
1. Update `backend/api/openapi.yaml` (path + reusable schema in `components`).
2. Keep `spectral lint` green.
3. Add/adjust a Bruno request under `docs/api/bruno/`.
