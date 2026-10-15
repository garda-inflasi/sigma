#!/usr/bin/env bash
# build.sh — Build all services
# Usage: bash scripts/build.sh [api|web|all]

set -euo pipefail

TARGET=${1:-all}

case $TARGET in
  api)
    echo "Building Go API..."
    cd apps/api
    go build -o bin/api ./cmd/api
    echo "✅ API built at apps/api/bin/api"
    ;;
  web)
    echo "Building Next.js..."
    cd apps/web
    pnpm build
    echo "✅ Web built"
    ;;
  all)
    echo "Building all services..."
    cd apps/api && go build -o bin/api ./cmd/api && echo "✅ API built"
    cd ../..
    cd apps/web && pnpm build && echo "✅ Web built"
    ;;
  *)
    echo "Usage: $0 [api|web|all]"
    exit 1
    ;;
esac
