#!/usr/bin/env bash
set -euo pipefail

umask 077

required=(GCP_PROJECT_ID GCP_REGION API_SERVICE_NAME API_BASE_URL EXPECTED_REVISION CONFIRM_PRODUCTION_ROLLBACK)
for name in "${required[@]}"; do
  [[ -n "${!name:-}" ]] || { echo "$name is required" >&2; exit 2; }
done
[[ "$CONFIRM_PRODUCTION_ROLLBACK" == "YES" ]] || {
  echo "CONFIRM_PRODUCTION_ROLLBACK must be YES" >&2
  exit 2
}
[[ "$EXPECTED_REVISION" =~ ^[a-f0-9]{12}$ ]] || {
  echo "EXPECTED_REVISION must be 12 lowercase hexadecimal characters" >&2
  exit 2
}
recovery="${1:?usage: scripts/rollback-cloud-run.sh recovery-private/deployments/TIMESTAMP}"
[[ -d "$recovery" ]] || { echo "recovery directory does not exist: $recovery" >&2; exit 2; }
case "$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).resolve())' "$recovery")" in
  "$(python3 -c 'import pathlib; print(pathlib.Path("recovery-private/deployments").resolve())')"/*) ;;
  *) echo "recovery directory must be under recovery-private/deployments" >&2; exit 2 ;;
esac

if [[ -f "$recovery/api.service.yaml" ]]; then
  gcloud run services replace "$recovery/api.service.yaml" \
    --project "$GCP_PROJECT_ID" \
    --region "$GCP_REGION" \
    --quiet
else
  echo "previous API service configuration was not captured" >&2
  exit 1
fi
if [[ -f "$recovery/worker.job.yaml" ]]; then
  gcloud run jobs replace "$recovery/worker.job.yaml" \
    --project "$GCP_PROJECT_ID" \
    --region "$GCP_REGION" \
    --quiet
fi

go run ./cmd/launch-smoke \
  --api-url "$API_BASE_URL" \
  --expected-revision "$EXPECTED_REVISION"

cat <<'NOTICE'
Runtime rollback completed.
Database schema and migrated data were not rolled back. Use a verified recovery point only through the separate restore procedure and an explicit data-loss decision.
NOTICE
