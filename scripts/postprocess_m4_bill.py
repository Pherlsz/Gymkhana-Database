#!/usr/bin/env python3
from pathlib import Path

for relative in (
    "internal/platform/httpserver/bill.go",
    "internal/platform/httpserver/bill_helpers.go",
    "internal/platform/httpserver/bill_test.go",
):
    path = Path(relative)
    text = path.read_text(encoding="utf-8")
    text = text.replace("Billos", "Contas/comprovantes")
    text = text.replace("Billo", "Conta/comprovante")
    text = text.replace("billos", "contas/comprovantes")
    text = text.replace("billo", "conta/comprovante")
    path.write_text(text, encoding="utf-8")
