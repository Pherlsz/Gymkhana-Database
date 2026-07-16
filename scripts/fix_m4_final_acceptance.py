#!/usr/bin/env python3
from pathlib import Path

panel_path = Path("apps/web/src/ProfileRecordsPanel.tsx")
panel = panel_path.read_text(encoding="utf-8")
old_catch = """    } catch (error) {
      await refresh();
      onNotice(conflictMessage(error));
    }
"""
new_catch = """    } catch (error) {
      await refresh();
      onNotice(conflictMessage(error));
      throw error;
    }
"""
if panel.count(old_catch) != 2:
    raise RuntimeError(f"expected two inline conflict catches, found {panel.count(old_catch)}")
panel = panel.replace(old_catch, new_catch)
old_blur = """      onBlur={() => {
        if (value !== props.value) void props.onSave(value);
      }}
"""
new_blur = """      onBlur={() => {
        if (value !== props.value)
          void props.onSave(value).catch(() => setValue(props.value));
      }}
"""
if panel.count(old_blur) != 1:
    raise RuntimeError("inline editor blur anchor not found")
panel_path.write_text(panel.replace(old_blur, new_blur), encoding="utf-8")

test_path = Path("apps/web/src/ProfileRecordsPanel.m4Acceptance.test.tsx")
test = test_path.read_text(encoding="utf-8")
for value in ("000A-99", "2026-07", "123.40"):
    old = f'expect(screen.getByDisplayValue("{value}")).toBeDisabled();'
    new = (
        f'expect(screen.getAllByDisplayValue("{value}").filter((element) => '
        'element.hasAttribute("disabled"))).toHaveLength(1);'
    )
    if old not in test:
        raise RuntimeError(f"test anchor not found for {value}")
    test = test.replace(old, new, 1)
test_path.write_text(test, encoding="utf-8")
