#!/usr/bin/env python3
from pathlib import Path

go_generator = Path(__file__).with_name("generate_m4_bill_go.py")
text = go_generator.read_text(encoding="utf-8")
text = text.replace(
    "Já existe uma conta com esse identificador para a política configurada",
    "Já existe um billo com esse identificador para a política configurada",
)
text = text.replace(
    "O tipo de conta está inativo",
    "O tipo de billo está inativo",
)
go_generator.write_text(text, encoding="utf-8")

openapi_generator = Path(__file__).with_name("generate_m4_bill_openapi.py")
text = openapi_generator.read_text(encoding="utf-8")
text = text.replace(
    '"current_use": {"oneOf": [ref("BillCurrentUse"), {"type": "null"}]}',
    '"current_use": ref("BillCurrentUse")',
)
openapi_generator.write_text(text, encoding="utf-8")
