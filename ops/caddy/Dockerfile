# syntax=docker/dockerfile:1

# --- build stage: Caddy + modul Coraza WAF ---
# Versi dipin demi reproducibility (ADR-0012): Caddy 2.11.4 (base image, tag
# terverifikasi ada di Docker Hub) + coraza-caddy v2.5.0 (rilis stabil
# terbaru saat ini, terverifikasi via `git ls-remote --tags`).
FROM caddy:2.11.4-builder-alpine AS build
RUN xcaddy build --with github.com/corazawaf/coraza-caddy/v2@v2.5.0

# --- runtime ---
FROM caddy:2.11.4-alpine
COPY --from=build /usr/bin/caddy /usr/bin/caddy
