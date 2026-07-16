#!/usr/bin/env python3
from pathlib import Path

path = Path(__file__).with_name("generate_m4_bill_go.py")
text = path.read_text(encoding="utf-8")
text = text.replace(
    "Já existe uma conta com esse identificador para a política configurada",
    "Já existe um billo com esse identificador para a política configurada",
)
text = text.replace(
    "O tipo de conta está inativo",
    "O tipo de billo está inativo",
)
path.write_text(text, encoding="utf-8")
