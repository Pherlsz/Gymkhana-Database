#!/usr/bin/env bash
set -euo pipefail
cd /home/pherls/projects/product/Gymkhana-Database
echo "=== cwd ==="
pwd
echo "=== env files ==="
ls -la .env .env.example 2>/dev/null || true
echo "=== node_modules ==="
if [ -d node_modules ]; then echo exists; else echo missing; fi
echo "=== versions ==="
go version
node -v
pnpm -v || true
docker --version
docker compose version
echo "=== compose ==="
docker compose ps
echo "=== listening ==="
ss -tln | grep -E ':8080|:5173|:5432' || true
