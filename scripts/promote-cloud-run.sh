#!/usr/bin/env bash
set -euo pipefail

umask 077

required=(
  GCP_PROJECT_ID GCP_REGION API_SERVICE_NAME WORKER_JOB_NAME MIGRATION_JOB_NAME
  API_BASE_URL EXPECTED_REVISION CONFIRM_PRODUCTION_PROMOTION
)
for name in "${required[@]}"; do
  [[ -n "${!name:-}" ]] || { echo "$name is required" >&2; exit 2; }
done
[[ "$CONFIRM_PRODUCTION_PROMOTION" == "YES" ]] || {
  echo "CONFIRM_PRODUCTION_PROMOTION must be YES" >&2
  exit 2
}
[[ "$EXPECTED_REVISION" =~ ^[a-f0-9]{12}$ ]] || {
  echo "EXPECTED_REVISION must be 12 lowercase hexadecimal characters" >&2
  exit 2
}
for command in gcloud python3 go; do
  command -v "$command" >/dev/null || { echo "$command is required" >&2; exit 2; }
done

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
rendered=".tmp/cloud-run-${stamp}"
recovery="recovery-private/deployments/${stamp}"
mkdir -p "$rendered" "$recovery"
python3 scripts/render-cloud-run.py --output-directory "$rendered"

if gcloud run services describe "$API_SERVICE_NAME" --project "$GCP_PROJECT_ID" --region "$GCP_REGION" >/dev/null 2>&1; then
  gcloud run services describe "$API_SERVICE_NAME" \
    --project "$GCP_PROJECT_ID" \
    --region "$GCP_REGION" \
    --format export > "$recovery/api.service.yaml"
fi
if gcloud run jobs describe "$WORKER_JOB_NAME" --project "$GCP_PROJECT_ID" --region "$GCP_REGION" >/dev/null 2>&1; then
  gcloud run jobs describe "$WORKER_JOB_NAME" \
    --project "$GCP_PROJECT_ID" \
    --region "$GCP_REGION" \
    --format export > "$recovery/worker.job.yaml"
fi

# Validate the service specification before making any change.
gcloud run services replace "$rendered/api.service.yaml" \
  --project "$GCP_PROJECT_ID" \
  --region "$GCP_REGION" \
  --dry-run >/dev/null

# Schema changes are isolated from API/worker startup and must finish first.
gcloud run jobs replace "$rendered/migrate.job.yaml" \
  --project "$GCP_PROJECT_ID" \
  --region "$GCP_REGION" \
  --quiet
gcloud run jobs execute "$MIGRATION_JOB_NAME" \
  --project "$GCP_PROJECT_ID" \
  --region "$GCP_REGION" \
  --wait

# API and worker are promoted only after the migration job succeeds.
gcloud run services replace "$rendered/api.service.yaml" \
  --project "$GCP_PROJECT_ID" \
  --region "$GCP_REGION" \
  --quiet
gcloud run jobs replace "$rendered/worker.job.yaml" \
  --project "$GCP_PROJECT_ID" \
  --region "$GCP_REGION" \
  --quiet

if [[ "${ALLOW_PUBLIC_API_INGRESS:-NO}" == "YES" ]]; then
  gcloud run services add-iam-policy-binding "$API_SERVICE_NAME" \
    --project "$GCP_PROJECT_ID" \
    --region "$GCP_REGION" \
    --member=allUsers \
    --role=roles/run.invoker \
    --quiet >/dev/null
fi

go run ./cmd/launch-smoke \
  --api-url "$API_BASE_URL" \
  --expected-revision "$EXPECTED_REVISION"

PROMOTION_RECEIPT="$recovery/receipt.json" \
PROMOTION_STAMP="$stamp" \
PROMOTION_API_IMAGE="$API_IMAGE_DIGEST" \
PROMOTION_WORKER_IMAGE="$WORKER_IMAGE_DIGEST" \
PROMOTION_MIGRATION_IMAGE="$MIGRATION_IMAGE_DIGEST" \
PROMOTION_REVISION="$EXPECTED_REVISION" \
python3 - <<'PY'
import json
import os
from pathlib import Path

path = Path(os.environ["PROMOTION_RECEIPT"])
payload = {
    "format": "cloud-run-promotion-v1",
    "promoted_at": os.environ["PROMOTION_STAMP"],
    "revision": os.environ["PROMOTION_REVISION"],
    "images": {
        "api": os.environ["PROMOTION_API_IMAGE"],
        "worker": os.environ["PROMOTION_WORKER_IMAGE"],
        "migrate": os.environ["PROMOTION_MIGRATION_IMAGE"],
    },
    "previous_api_exported": (path.parent / "api.service.yaml").exists(),
    "previous_worker_exported": (path.parent / "worker.job.yaml").exists(),
}
path.write_text(json.dumps(payload, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
PY

printf 'promotion completed: revision=%s recovery=%s\n' "$EXPECTED_REVISION" "$recovery"
