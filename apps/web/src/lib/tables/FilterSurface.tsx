import { Button, Empty, Input, Select } from "antd";
import { Info, Plus, X } from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { FilterControl, type ToolbarFilterField } from "./FilterControl";
import { ICON, ICON_STROKE } from "../../components/icons";

type PendingRow = { id: string; fieldKey: string | null };

function hasValue(field: ToolbarFilterField): boolean {
  return field.value.trim().length > 0;
}

type FilterRowProps = {
  rowId: string;
  isApplied: boolean;
  fieldKey: string | null;
  field: ToolbarFilterField | undefined;
  appliedRow: boolean;
  canRemove: boolean;
  fieldOptions: { value: string; label: string }[];
  chooseFieldLabel: string;
  chooseValueLabel: string;
  removeFilterLabel: string;
  onPickField: (
    rowId: string,
    isApplied: boolean,
    field: ToolbarFilterField | undefined,
    nextKey: string | null,
  ) => void;
  onRemove: (rowId: string, isApplied: boolean, field: ToolbarFilterField | undefined) => void;
};

const FilterRow = memo(function FilterRow({
  rowId,
  isApplied,
  fieldKey,
  field,
  appliedRow,
  canRemove,
  fieldOptions,
  chooseFieldLabel,
  chooseValueLabel,
  removeFilterLabel,
  onPickField,
  onRemove,
}: FilterRowProps) {
  const handleFieldChange = useCallback(
    (value: string | null | undefined) => {
      onPickField(rowId, isApplied, field, value ?? null);
    },
    [onPickField, rowId, isApplied, field],
  );

  const handleRemoveClick = useCallback(() => {
    onRemove(rowId, isApplied, field);
  }, [onRemove, rowId, isApplied, field]);

  return (
    <div
      className={appliedRow ? "filter-surface__row is-applied" : "filter-surface__row"}
      key={rowId}
    >
      <Select
        allowClear
        aria-label={chooseFieldLabel}
        className="filter-surface__field-select"
        optionFilterProp="label"
        options={fieldOptions}
        placeholder={chooseFieldLabel}
        popupMatchSelectWidth
        showSearch
        value={fieldKey ?? undefined}
        onChange={handleFieldChange}
      />
      <div className="filter-surface__control">
        {field ? (
          <FilterControl key={field.key} autoFocus={!hasValue(field)} field={field} />
        ) : (
          <Input allowClear disabled placeholder={chooseValueLabel} style={{ width: "100%" }} />
        )}
      </div>
      <Button
        aria-label={removeFilterLabel}
        className="filter-surface__remove"
        disabled={!canRemove}
        icon={<X aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />}
        type="text"
        onClick={handleRemoveClick}
      />
    </div>
  );
});

export function FilterSurface({
  addFilterLabel,
  chooseFieldLabel,
  chooseValueLabel,
  removeFilterLabel,
  noFieldsLabel,
  appliedLabel,
  clearLabel,
  localHint,
  fields,
}: {
  addFilterLabel: string;
  chooseFieldLabel: string;
  chooseValueLabel: string;
  removeFilterLabel: string;
  noFieldsLabel: string;
  appliedLabel: string;
  clearLabel: string;
  localHint: string;
  fields: ToolbarFilterField[];
}) {
  const draftSeq = useRef(0);
  const nextDraftId = () => {
    draftSeq.current += 1;
    return `draft-${draftSeq.current}`;
  };

  const [pending, setPending] = useState<PendingRow[]>(() =>
    fields.some(hasValue) ? [] : [{ id: "draft-0", fieldKey: null }],
  );

  const applied = useMemo(() => fields.filter(hasValue), [fields]);

  useEffect(() => {
    const appliedKeys = new Set(applied.map((field) => field.key));
    setPending((current) => {
      const pruned = current.filter(
        (row) => row.fieldKey === null || !appliedKeys.has(row.fieldKey),
      );
      if (applied.length === 0 && pruned.length === 0) {
        return [{ id: nextDraftId(), fieldKey: null }];
      }
      return pruned.length === current.length ? current : pruned;
    });
  }, [applied]);

  const pendingShown = pending.filter(
    (row) => row.fieldKey === null || !applied.some((field) => field.key === row.fieldKey),
  );
  const usedKeys = useMemo(
    () =>
      new Set([
        ...applied.map((field) => field.key),
        ...pendingShown.flatMap((row) => (row.fieldKey ? [row.fieldKey] : [])),
      ]),
    [applied, pendingShown],
  );
  const unusedCount = fields.filter((field) => !usedKeys.has(field.key)).length;
  const hasEmptyPending = pendingShown.some((row) => row.fieldKey === null);
  const canAdd = unusedCount > 0 && !hasEmptyPending;
  const showsLocalHint = fields.some(
    (field) =>
      field.local && (hasValue(field) || pendingShown.some((row) => row.fieldKey === field.key)),
  );

  const fieldOptionsFor = useCallback(
    (currentKey: string | null) =>
      fields
        .filter((field) => field.key === currentKey || !usedKeys.has(field.key))
        .map((field) => ({ value: field.key, label: field.label })),
    [fields, usedKeys],
  );

  const clearField = useCallback(
    (key: string | null) => {
      if (!key) return;
      const field = fields.find((item) => item.key === key);
      if (field && hasValue(field)) field.onChange("");
    },
    [fields],
  );

  const pickAppliedField = useCallback((field: ToolbarFilterField, nextKey: string | null) => {
    field.onChange("");
    setPending((current) => [...current, { id: nextDraftId(), fieldKey: nextKey }]);
  }, []);

  const pickPendingField = useCallback(
    (row: PendingRow, nextKey: string | null) => {
      if (row.fieldKey && row.fieldKey !== nextKey) clearField(row.fieldKey);
      setPending((current) =>
        current.map((item) => (item.id === row.id ? { ...item, fieldKey: nextKey } : item)),
      );
    },
    [clearField],
  );

  const removePending = useCallback(
    (row: PendingRow) => {
      clearField(row.fieldKey);
      setPending((current) => {
        const next = current.filter((item) => item.id !== row.id);
        if (applied.length === 0 && next.length === 0) {
          return [{ id: nextDraftId(), fieldKey: null }];
        }
        return next;
      });
    },
    [applied.length, clearField],
  );

  const handleClearAll = useCallback(() => {
    for (const field of fields) {
      if (hasValue(field)) field.onChange("");
    }
    setPending([{ id: nextDraftId(), fieldKey: null }]);
  }, [fields]);

  const handlePickField = useCallback(
    (
      rowId: string,
      isApplied: boolean,
      field: ToolbarFilterField | undefined,
      nextKey: string | null,
    ) => {
      if (isApplied && field) {
        pickAppliedField(field, nextKey);
      } else {
        const pendingRow = pending.find((p) => p.id === rowId);
        if (pendingRow) pickPendingField(pendingRow, nextKey);
      }
    },
    [pickAppliedField, pickPendingField, pending],
  );

  const handleRemove = useCallback(
    (rowId: string, isApplied: boolean, field: ToolbarFilterField | undefined) => {
      if (isApplied && field) {
        field.onChange("");
      } else {
        const pendingRow = pending.find((p) => p.id === rowId);
        if (pendingRow) removePending(pendingRow);
      }
    },
    [removePending, pending],
  );

  const rows = [
    ...applied.map((field) => ({ kind: "applied" as const, id: field.key, field })),
    ...pendingShown.map((row) => ({ kind: "pending" as const, ...row })),
  ];
  const canRemoveRow = (fieldKey: string | null) =>
    applied.length + pendingShown.length > 1 || Boolean(fieldKey);

  return (
    <div className="filter-surface">
      <div className="filter-surface__header">
        <div className="filter-surface__header-title">
          <span>{appliedLabel}</span>
          <span className={`filter-surface__header-badge ${applied.length > 0 ? "is-active" : ""}`}>
            {applied.length > 0
              ? `${applied.length} ativo${applied.length > 1 ? "s" : ""}`
              : "Nenhum ativo"}
          </span>
        </div>
        {applied.length > 0 ? (
          <Button
            className="filter-surface__clear-btn"
            size="small"
            type="text"
            onClick={handleClearAll}
          >
            {clearLabel}
          </Button>
        ) : null}
      </div>

      {rows.length === 0 ? (
        <Empty description={noFieldsLabel} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <div aria-label={appliedLabel} className="filter-surface__list" role="group">
          {rows.map((row) => {
            const fieldKey = row.kind === "applied" ? row.field.key : row.fieldKey;
            const field =
              row.kind === "applied" ? row.field : fields.find((item) => item.key === row.fieldKey);
            const appliedRow = Boolean(field && hasValue(field));
            const isApplied = row.kind === "applied";
            return (
              <FilterRow
                appliedRow={appliedRow}
                canRemove={canRemoveRow(fieldKey)}
                chooseFieldLabel={chooseFieldLabel}
                chooseValueLabel={chooseValueLabel}
                field={field}
                fieldKey={fieldKey}
                fieldOptions={fieldOptionsFor(fieldKey)}
                isApplied={isApplied}
                key={row.id}
                removeFilterLabel={removeFilterLabel}
                rowId={row.id}
                onPickField={handlePickField}
                onRemove={handleRemove}
              />
            );
          })}
        </div>
      )}

      <div className="filter-surface__footer">
        <Button
          className="filter-surface__add-btn"
          disabled={!canAdd}
          icon={<Plus aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />}
          type="dashed"
          onClick={() =>
            setPending((current) => [...current, { id: nextDraftId(), fieldKey: null }])
          }
        >
          {addFilterLabel}
        </Button>
        {showsLocalHint ? (
          <span className="filter-surface__hint-pill">
            <Info aria-hidden size={ICON.badge} strokeWidth={ICON_STROKE} />
            {localHint}
          </span>
        ) : null}
      </div>
    </div>
  );
}
