# syntax=docker/dockerfile:1
# Tooling Ansible ter-container (host tidak punya ansible). Versi di-pin.
FROM python:3.12-alpine
RUN apk add --no-cache git openssh-client \
 && pip install --no-cache-dir "ansible-core==2.17.6" "ansible-lint==24.9.2"
WORKDIR /work
