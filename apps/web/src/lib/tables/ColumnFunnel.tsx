import { memo, useCallback, useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import { ListFilter } from "lucide-react";
import { Button, Input, Popover, Select } from "antd";
import { ICON, ICON_STROKE } from "../../components/icons";
import { fieldPredicate, resolveFunnelKind } from "./bindPredicates";
import { opsForKind, predicateActive, type ColumnOp, type ColumnPredicate } from "./columnPredicate";
import type { ToolbarFilterField } from "./FilterControl";
import { funcsForKind, insertFormula, matchFormula, type FormulaSpec } from "./formulas/catalog";

export type ColumnFunnelCopy = {
  filter: string;
  operator: string;
  value: string;
  from: string;
  to: string;
  function: string;
  formula: string;
  insert: string;
  removeFormula: string;
  ops: Record<ColumnOp, string>;
};

function predicateKey(predicate: ColumnPredicate): string {
  return `${predicate.op}\0${predicate.values.join("\0")}`;
}

function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <label className="column-funnel__field">
      <span className="column-funnel__label">{label}</span>
      {children}
    </label>
  );
}

export const ColumnFunnel = memo(function ColumnFunnel({
  field,
  copy,
  formula,
  columns,
}: {
  field: ToolbarFilterField;
  copy: ColumnFunnelCopy;
  formula?:
    | {
        expression: string;
        onApply: (expression: string) => void;
        onRemove: () => void;
      }
    | undefined;
  columns: { key: string; label: string }[];
}) {
  const kind = resolveFunnelKind(field);
  const current = fieldPredicate(field);
  const surfaceId = useId();
  const draftRef = useRef(current);
  const fxRef = useRef(formula?.expression ?? "");
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(current);
  const [fx, setFx] = useState(formula?.expression ?? "");
  const [fxError, setFxError] = useState("");

  draftRef.current = draft;
  fxRef.current = fx;

  const ops = useMemo(() => opsForKind(kind), [kind]);
  const funcs = useMemo(() => funcsForKind(kind), [kind]);
  const other = useMemo(
    () => columns.find((item) => item.key !== field.key)?.label ?? columns[0]?.label ?? field.label,
    [columns, field.key, field.label],
  );
  const active = predicateActive(current) || Boolean(formula?.expression);
  const needsValue = draft.op !== "is_null" && draft.op !== "not_null";
  const pickedFn = matchFormula(fx)?.name ?? "";

  const opOptions = useMemo(
    () => ops.map((op) => ({ value: op, label: copy.ops[op] })),
    [copy.ops, ops],
  );
  const fnOptions = useMemo(
    () =>
      funcs.map((item) => ({
        value: item.name,
        label: `${item.name} — ${item.hint}`,
      })),
    [funcs],
  );
  const activeHint = useMemo(() => matchFormula(fx)?.hint, [fx]);

  const commitPredicate = useCallback(
    (next: ColumnPredicate) => {
      if (predicateKey(next) === predicateKey(fieldPredicate(field))) return;
      field.onPredicate?.(next);
    },
    [field],
  );

  const commitFx = useCallback(
    (expression: string) => {
      if (!formula) return;
      const next = expression.trim();
      if (next === (formula.expression ?? "").trim()) return;
      try {
        if (!next) formula.onRemove();
        else formula.onApply(next);
        setFxError("");
      } catch (error) {
        setFxError(error instanceof Error ? error.message : "expr");
      }
    },
    [formula],
  );

  useEffect(() => {
    if (!open) return;
    const next = fieldPredicate(field);
    setDraft(next);
    setFx(formula?.expression ?? "");
    setFxError("");
  }, [field, formula?.expression, open]);

  const close = useCallback(() => {
    commitPredicate(draftRef.current);
    if (formula) commitFx(fxRef.current);
    setOpen(false);
  }, [commitFx, commitPredicate, formula]);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Element)) return;
      if (
        target.closest(
          ".column-funnel__overlay, .column-funnel__trigger, .column-funnel__anchor, .ant-select-dropdown",
        )
      ) {
        return;
      }
      close();
    };
    document.addEventListener("pointerdown", onPointerDown, true);
    return () => document.removeEventListener("pointerdown", onPointerDown, true);
  }, [close, open]);

  const onOpenChange = useCallback(
    (next: boolean) => {
      if (next) {
        setOpen(true);
        return;
      }
      close();
    },
    [close],
  );

  const changeOp = useCallback(
    (op: ColumnOp) => {
      const next = { op, values: [] as string[] };
      setDraft(next);
      commitPredicate(next);
    },
    [commitPredicate],
  );

  const insertFn = useCallback(
    (name: string) => {
      if (!formula) return;
      if (!name) {
        setFx("");
        commitFx("");
        return;
      }
      const spec = funcs.find((item) => item.name === name) as FormulaSpec;
      const next = insertFormula(spec, field.label, other, kind);
      setFx(next);
      commitFx(next);
    },
    [commitFx, field.label, formula, funcs, kind, other],
  );

  const body = open ? (
    <div
      aria-label={copy.filter.replace("{label}", field.label)}
      className="column-funnel"
      id={surfaceId}
      role="dialog"
      onClick={(event) => event.stopPropagation()}
      onKeyDown={(event) => {
        if (event.key !== "Escape") return;
        event.stopPropagation();
        close();
      }}
    >
      <div className="column-funnel__row">
        <Field label={copy.operator}>
          <Select
            aria-label={copy.operator}
            options={opOptions}
            popupMatchSelectWidth={false}
            style={{ width: "100%" }}
            value={draft.op}
            onChange={(value) => changeOp(value as ColumnOp)}
          />
        </Field>
        {needsValue ? (
          draft.op === "between" ? (
            <div className="column-funnel__range">
              <Field label={copy.from}>
                <Input
                  aria-label={copy.from}
                  value={draft.values[0] ?? ""}
                  onChange={(event) =>
                    setDraft({ ...draft, values: [event.target.value, draft.values[1] ?? ""] })
                  }
                  onPressEnter={() => commitPredicate(draft)}
                />
              </Field>
              <Field label={copy.to}>
                <Input
                  aria-label={copy.to}
                  value={draft.values[1] ?? ""}
                  onChange={(event) =>
                    setDraft({ ...draft, values: [draft.values[0] ?? "", event.target.value] })
                  }
                  onPressEnter={() => commitPredicate(draft)}
                />
              </Field>
            </div>
          ) : draft.op === "in" && field.kind === "select" ? (
            <Field label={copy.value}>
              <Select
                aria-label={copy.value}
                mode="multiple"
                options={field.options.map((option) => ({
                  value: option.value,
                  label: option.label,
                }))}
                placeholder={field.allLabel}
                style={{ width: "100%" }}
                value={draft.values}
                onChange={(values) => {
                  const next = { ...draft, values };
                  draftRef.current = next;
                  setDraft(next);
                  commitPredicate(next);
                }}
              />
            </Field>
          ) : field.kind === "select" && (draft.op === "eq" || draft.op === "neq") ? (
            <Field label={copy.value}>
              <Select
                allowClear
                aria-label={copy.value}
                autoFocus
                options={field.options.map((option) => ({
                  value: option.value,
                  label: option.label,
                }))}
                placeholder={field.allLabel}
                style={{ width: "100%" }}
                value={draft.values[0] || undefined}
                onChange={(value) => {
                  const next = {
                    ...draft,
                    values: value ? [value] : [],
                  };
                  draftRef.current = next;
                  setDraft(next);
                  commitPredicate(next);
                }}
              />
            </Field>
          ) : (
            <Field label={copy.value}>
              <Input
                aria-label={copy.value}
                autoFocus
                value={draft.values[0] ?? ""}
                onChange={(event) => {
                  const next = { ...draft, values: [event.target.value] };
                  draftRef.current = next;
                  setDraft(next);
                }}
                onPressEnter={() => {
                  commitPredicate(draftRef.current);
                  close();
                }}
              />
            </Field>
          )
        ) : (
          <span className="column-funnel__spacer" />
        )}
      </div>

      {formula ? (
        <div className="column-funnel__fx">
          <Field label={copy.function}>
            <Select
              allowClear
              aria-label={copy.function}
              options={fnOptions}
              placeholder={copy.insert}
              popupMatchSelectWidth={false}
              style={{ width: "100%" }}
              value={pickedFn || undefined}
              onChange={(value) => insertFn(value ?? "")}
            />
          </Field>
          <Field label={copy.formula}>
            <Input
              aria-label={copy.formula}
              className="column-funnel__formula"
              spellCheck={false}
              value={fx}
              onChange={(event) => setFx(event.target.value)}
              onPressEnter={() => {
                commitFx(fx);
                close();
              }}
            />
          </Field>
          {activeHint ? <p className="column-funnel__hint">{activeHint}</p> : null}
          {fxError ? <p className="column-funnel__error">{fxError}</p> : null}
          {formula.expression ? (
            <Button
              className="column-funnel__ghost"
              size="small"
              type="link"
              onClick={() => {
                setFx("");
                commitFx("");
              }}
            >
              {copy.removeFormula}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  ) : null;

  return (
    <Popover
      arrow={false}
      classNames={{ root: "column-funnel__overlay" }}
      content={body}
      destroyOnHidden
      open={open}
      placement="bottomLeft"
      trigger={[]}
      onOpenChange={onOpenChange}
    >
      {/*
        Native button: Ant Design Button as a Popover child in every header cell
        hung the tables sheet (infinite update). No IconButton exists in this repo.
      */}
      <button
        aria-controls={open ? surfaceId : undefined}
        aria-expanded={open}
        aria-haspopup="dialog"
        aria-label={copy.filter.replace("{label}", field.label)}
        className={active || open ? "column-funnel__trigger is-active" : "column-funnel__trigger"}
        type="button"
        onClick={(event) => {
          event.stopPropagation();
          onOpenChange(!open);
        }}
      >
        <ListFilter aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
      </button>
    </Popover>
  );
});
