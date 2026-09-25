import type { ColumnPredicate, FunnelKind } from "./columnPredicate";
import { defaultOp, funnelKindOf, predicateActive } from "./columnPredicate";
import type { ToolbarFilterField } from "./FilterControl";

export function resolveFunnelKind(field: ToolbarFilterField): FunnelKind {
  return field.funnelKind ?? funnelKindOf(field.kind, field.key);
}

export function fieldPredicate(field: ToolbarFilterField): ColumnPredicate {
  if (field.predicate) return field.predicate;
  return { op: defaultOp(resolveFunnelKind(field)), values: field.value ? [field.value] : [] };
}

export function fieldFilterActive(field: ToolbarFilterField): boolean {
  return predicateActive(fieldPredicate(field));
}

export function bindPredicates(
  fields: ToolbarFilterField[],
  extra: Record<string, ColumnPredicate>,
  setExtra: (
    updater: (current: Record<string, ColumnPredicate>) => Record<string, ColumnPredicate>,
  ) => void,
): ToolbarFilterField[] {
  return fields.map((field) => {
    const kind = resolveFunnelKind(field);
    const predicate = extra[field.key] ?? {
      op: defaultOp(kind),
      values: field.value ? [field.value] : [],
    };
    return {
      ...field,
      funnelKind: kind,
      predicate,
      onPredicate: (next) => {
        const fallback = defaultOp(kind);
        if (!predicateActive(next) || (next.op === fallback && next.values.length <= 1)) {
          setExtra((current) => {
            const copy = { ...current };
            delete copy[field.key];
            return copy;
          });
          field.onChange(next.op === fallback ? (next.values[0] ?? "") : "");
          return;
        }
        setExtra((current) => ({ ...current, [field.key]: next }));
        field.onChange("");
      },
    };
  });
}
