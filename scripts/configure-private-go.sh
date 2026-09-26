#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${GYMKHANA_REPOSITORY_TOKEN:-}" ]]; then
  echo "::error::GYMKHANA_REPOSITORY_TOKEN is required to download the private Gymkhana Core module."
  exit 1
fi

git config --local --unset-all http.https://github.com/.extraheader 2>/dev/null || true
git config --global url."https://x-access-token:${GYMKHANA_REPOSITORY_TOKEN}@github.com/".insteadOf "https://github.com/"
go env -w GOPRIVATE=github.com/Pherlsz/Gymkhana-Core
go env -w GONOSUMDB=github.com/Pherlsz/Gymkhana-Core
