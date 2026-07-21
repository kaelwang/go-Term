#!/usr/bin/env bash
# build.sh: build the frontend, embed it, and cross-compile the backend for
# linux/amd64. Equivalent to `make linux`.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "==> Building frontend"
cd frontend
npm ci
npm run build
cd "$ROOT_DIR"

echo "==> Embedding frontend assets"
rm -rf internal/static/dist
mkdir -p internal/static/dist
cp -r frontend/dist/. internal/static/dist/

echo "==> Cross-compiling backend (linux/amd64, CGO disabled)"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -tags embedstatic -o dist/go-Term ./cmd/server

echo "==> Done: dist/go-Term"
