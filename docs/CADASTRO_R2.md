# Cadastro — R2 activation runbook

Cloudflare R2 is required for attachments, XLSX import temp storage, digital document exemplars, and OCR source reads (Orchestration §11).

## Owner checklist

1. Create a **private** R2 bucket.
2. Configure CORS for the SPA origin (`AUTH_APPLICATION_URL`, e.g. `http://localhost:5173` in dev).
3. Store credentials in lokeys profile `gymkhana` (never commit):

   | Variable               | Purpose                                      |
   | ---------------------- | -------------------------------------------- |
   | `R2_ENABLED`           | `true`                                       |
   | `R2_ENDPOINT`          | `https://<account>.r2.cloudflarestorage.com` |
   | `R2_BUCKET`            | Bucket name                                  |
   | `R2_ACCESS_KEY_ID`     | API token                                    |
   | `R2_SECRET_ACCESS_KEY` | Secret                                       |

4. Verify: `make check-config` reports `attachments_enabled=true`.
5. Smoke: upload a PDF via **Anexos** on a document record; confirm appears in list.

## Local development (WSL)

lokeys is DPAPI-backed on Windows. From PowerShell:

```powershell
$env:LOKEYS_AGENT = "1"
lokeys run -p gymkhana --env dev -- make check-config
```

Until R2 is enabled, Cadastro **OCR** and **attachment upload** show a disabled state with reason in the UI. Manual and XLSX import (metadata-only staging) may still be tested where the worker has R2 for temp files.

## Worker dependency

`cmd/worker` requires `R2_ENABLED=true` for import parse jobs. Enable R2 before end-to-end XLSX cadastro tests.
