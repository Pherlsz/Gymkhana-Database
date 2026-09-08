#!/usr/bin/env bash
set -euo pipefail

required=(
  GCP_PROJECT_ID GCP_REGION WORKER_JOB_NAME WORKER_SCHEDULER_NAME
  WORKER_SCHEDULE SCHEDULER_TIME_ZONE SCHEDULER_SERVICE_ACCOUNT
  CONFIRM_PRODUCTION_SCHEDULE
)
for name in "${required[@]}"; do
  [[ -n "${!name:-}" ]] || { echo "$name is required" >&2; exit 2; }
done
[[ "$CONFIRM_PRODUCTION_SCHEDULE" == "YES" ]] || {
  echo "CONFIRM_PRODUCTION_SCHEDULE must be YES" >&2
  exit 2
}

uri="https://run.googleapis.com/v2/projects/${GCP_PROJECT_ID}/locations/${GCP_REGION}/jobs/${WORKER_JOB_NAME}:run"
common=(
  --project "$GCP_PROJECT_ID"
  --location "$GCP_REGION"
  --schedule "$WORKER_SCHEDULE"
  --time-zone "$SCHEDULER_TIME_ZONE"
  --uri "$uri"
  --http-method POST
  --headers Content-Type=application/json
  --message-body '{}'
  --oauth-service-account-email "$SCHEDULER_SERVICE_ACCOUNT"
  --oauth-token-scope https://www.googleapis.com/auth/cloud-platform
  --attempt-deadline 30s
  --quiet
)

if gcloud scheduler jobs describe "$WORKER_SCHEDULER_NAME" \
  --project "$GCP_PROJECT_ID" \
  --location "$GCP_REGION" >/dev/null 2>&1; then
  gcloud scheduler jobs update http "$WORKER_SCHEDULER_NAME" "${common[@]}"
else
  gcloud scheduler jobs create http "$WORKER_SCHEDULER_NAME" "${common[@]}"
fi

printf 'worker schedule configured: %s %s (%s)\n' "$WORKER_SCHEDULER_NAME" "$WORKER_SCHEDULE" "$SCHEDULER_TIME_ZONE"
