# Cadastro — R2 activation runbook

Cloudflare R2 is required for attachments, XLSX import temp storage, digital document exemplars, and OCR source reads (Orchestration §11).

## Owner checklist

1. Create a **private** R2 bucket.
2. Configure CORS for the SPA origin (`AUTH_APPLICATION_URL`, e.g. `http://localhost:5173` in dev).
3. Store credentials in gitignored `.env` (never commit):

   | Variable               | Purpose                                      |
   | ---------------------- | -------------------------------------------- |
   | `R2_ENDPOINT`          | `https://<account>.r2.cloudflarestorage.com` |
   | `R2_BUCKET`            | Bucket name                                  |
   | `R2_ACCESS_KEY_ID`     | API token                                    |
   | `R2_SECRET_ACCESS_KEY` | Secret                                       |

   Product on/off for user-facing anexos is Administração → Funcionalidades (`attachments`).

4. Verify: `make check-config` reports attachments configured when credentials are complete.
5. Smoke: upload a PDF via **Anexos** on a document record; confirm appears in list.

## Local development (WSL)

```bash
make check-config
```

Until R2 credentials are set and the product flag is on, Cadastro **OCR** and **attachment upload** show a disabled state with reason in the UI. Manual and XLSX import (metadata-only staging) may still be tested where the worker has R2 for temp files.

## Worker dependency

`cmd/worker` requires complete R2 credentials for import parse jobs. Configure R2 before end-to-end XLSX cadastro tests.
