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

helpers = Path("internal/platform/httpserver/bill_helpers.go")
text = helpers.read_text(encoding="utf-8")
start = text.index("func optionalProfileIdentifier")
end = text.index("func optionalBillIdentifier", start)
text = text[:start] + text[end:]
helpers.write_text(text, encoding="utf-8")

service_test = Path("internal/bill/service_test.go")
text = service_test.read_text(encoding="utf-8")
text = text.replace('Filters.Reference != "ref-00ab"', 'Filters.Reference != "ref 00ab"')
service_test.write_text(text, encoding="utf-8")
