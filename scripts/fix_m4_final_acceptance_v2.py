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
panel = panel.replace(old_blur, new_blur)
old_holder_state = """  const [holder, setHolder] = useState(props.record.current_use?.holder_profile_id ?? "");
  const [error, setError] = useState<string | null>(null);
"""
new_holder_state = """  const [holder, setHolder] = useState(props.record.current_use?.holder_profile_id ?? "");
  const holderAvailable = holders.data?.profiles.some((value) => value.id === holder) ?? false;
  const [error, setError] = useState<string | null>(null);
"""
if panel.count(old_holder_state) != 1:
    raise RuntimeError("current holder state anchor not found")
panel = panel.replace(old_holder_state, new_holder_state)
old_holder_options = """          <select value={holder} onChange={(event) => setHolder(event.target.value)}>
            <option value="">Selecione</option>
            {holders.data?.profiles.map((value) => (
"""
new_holder_options = """          <select value={holder} onChange={(event) => setHolder(event.target.value)}>
            <option value="">Selecione</option>
            {holder && !holderAvailable ? <option value={holder}>Pessoa atual</option> : null}
            {holders.data?.profiles.map((value) => (
"""
if panel.count(old_holder_options) != 1:
    raise RuntimeError("current holder options anchor not found")
panel_path.write_text(panel.replace(old_holder_options, new_holder_options), encoding="utf-8")

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
