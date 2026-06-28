# syntax=docker/dockerfile:1

# --- build stage ---
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static binary; the OpenAPI spec and Scalar bundle are embedded via go:embed.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# --- runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache wget && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/api /usr/local/bin/api
# Runtime assets (e.g. the label logo) — resolved by the default relative LABEL_LOGO_PATH=assets/...
COPY --from=build /src/assets /app/assets
USER app
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=5 \
  CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["api"]
