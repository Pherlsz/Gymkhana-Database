#!/usr/bin/env python3
from pathlib import Path

path = Path("apps/web/src/ProfileRecordsPanel.tsx")
text = path.read_text(encoding="utf-8")
text = text.replace(
    'onTypes={canAdministerTypes ? () => onSearch({ document_mode: "types" }) : undefined}',
    '{...(canAdministerTypes ? { onTypes: () => onSearch({ document_mode: "types" }) } : {})}',
)
text = text.replace(
    'onTypes={canAdministerTypes ? () => onSearch({ bill_mode: "types" }) : undefined}',
    '{...(canAdministerTypes ? { onTypes: () => onSearch({ bill_mode: "types" }) } : {})}',
)
text = text.replace("onTypes: (() => void) | undefined;", "onTypes?: () => void;")
path.write_text(text, encoding="utf-8")
