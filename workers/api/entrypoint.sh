#!/bin/sh
set -eu
echo "api-container boot APP_ENV=${APP_ENV:-} AUTH_ENABLED=${AUTH_ENABLED:-} R2=${R2_ENABLED:-} OCR=${OCR_ENABLED:-} DB_LEN=${#DATABASE_URL} OAUTH_ID_LEN=${#GOOGLE_OAUTH_CLIENT_ID} OAUTH_SECRET_LEN=${#GOOGLE_OAUTH_CLIENT_SECRET} REDIRECT_LEN=${#GOOGLE_OAUTH_REDIRECT_URL} APP_URL_LEN=${#AUTH_APPLICATION_URL}"
# River worker processes OCR/import/matching jobs; API only enqueues.
/worker &
WORKER_PID=$!
cleanup() {
  kill "$WORKER_PID" 2>/dev/null || true
  wait "$WORKER_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM
exec /api
