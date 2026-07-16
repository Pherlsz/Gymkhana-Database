#!/usr/bin/env python3
from pathlib import Path

path = Path("apps/web/src/ProfileRecordsPanel.test.tsx")
text = path.read_text(encoding="utf-8")
text = text.replace(
    'expect(screen.getAllByText("00123")).not.toHaveLength(0);',
    'expect(screen.getByDisplayValue("00123")).toBeInTheDocument();',
)
path.write_text(text, encoding="utf-8")
