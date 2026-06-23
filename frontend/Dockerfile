# syntax=docker/dockerfile:1

# --- build stage ---
FROM node:24-alpine AS build
WORKDIR /app
RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY . .
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
