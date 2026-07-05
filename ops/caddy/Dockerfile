# syntax=docker/dockerfile:1

# --- build stage: Caddy + modul Coraza WAF ---
FROM caddy:2-builder-alpine AS build
RUN xcaddy build --with github.com/corazawaf/coraza-caddy/v2

# --- runtime ---
FROM caddy:2-alpine
COPY --from=build /usr/bin/caddy /usr/bin/caddy
