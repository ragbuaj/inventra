# syntax=docker/dockerfile:1

# --- build stage ---
FROM node:24-alpine AS build
WORKDIR /app
RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .

# Browser-facing API base. This MUST be set at BUILD time, not at container
# runtime: route `/` is prerendered into a static HTML file (see
# nuxt.config.ts -> nitro.prerender) so the service worker can precache the app
# shell, and that static file carries the resolved runtimeConfig with it. An env
# var set when the container starts no longer reaches the landing page — it would
# only reach the routes Nitro still renders live, which is worse than not setting
# it at all. CD passes the production value via build-args (.github/workflows/deploy.yml).
ARG NUXT_PUBLIC_API_BASE=http://localhost:8080/api/v1
ENV NUXT_PUBLIC_API_BASE=$NUXT_PUBLIC_API_BASE

# Nuxt/Nitro build is memory-hungry; raise V8's heap limit above the ~2GB default.
RUN NODE_OPTIONS=--max-old-space-size=4096 pnpm build

# --- runtime stage ---
FROM node:24-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=3000
COPY --from=build /app/.output ./.output
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
