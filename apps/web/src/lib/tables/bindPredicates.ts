import type { ColumnOp, ColumnPredicate, FunnelKind } from "./columnPredicate";
import { defaultOp, funnelKindOf, predicateActive } from "./columnPredicate";
import type { ToolbarFilterField } from "./FilterControl";

export function resolveFunnelKind(field: ToolbarFilterField): FunnelKind {
  return field.funnelKind ?? funnelKindOf(field.kind, field.key);
}

export function decodeFieldValue(val: string, fallbackOp: ColumnOp): ColumnPredicate {
  if (!val) return { op: fallbackOp, values: [] };
  if (val.startsWith("^")) return { op: "starts_with", values: [val.slice(1)] };
  if (val.startsWith("=")) return { op: "eq", values: [val.slice(1)] };
  return { op: fallbackOp, values: [val] };
}

export function encodePredicateValue(
  predicate: ColumnPredicate,
  fieldKind?: "text" | "date" | "select",
): string {
  if (!predicateActive(predicate) || predicate.values.length === 0 || !predicate.values[0]) {
    return "";
  }
  const val = predicate.values[0];
  if (predicate.op === "starts_with") return `^${val}`;
  if (predicate.op === "eq" && fieldKind === "text") return `=${val}`;
  return val;
}

export function fieldPredicate(field: ToolbarFilterField): ColumnPredicate {
  if (field.predicate) return field.predicate;
  const kind = resolveFunnelKind(field);
  return decodeFieldValue(field.value, defaultOp(kind));
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
    const predicate = extra[field.key] ?? decodeFieldValue(field.value, defaultOp(kind));
    return {
      ...field,
      funnelKind: kind,
      predicate,
      onPredicate: (next: ColumnPredicate) => {
        const isActive = predicateActive(next);
        if (!isActive) {
          setExtra((current) => {
            const copy = { ...current };
            delete copy[field.key];
            return copy;
          });
          field.onChange("");
        } else {
          setExtra((current) => ({ ...current, [field.key]: next }));
          field.onChange(encodePredicateValue(next, field.kind));
        }
      },
    };
  });
}

/**
 * Like bindPredicates but caches `onPredicate` handlers by field key so that
 * callers using this inside a useMemo don't produce new object references
 * for every field on every render when only `extra` or `value` changed.
 */
export function bindPredicatesStable(
  fields: ToolbarFilterField[],
  extra: Record<string, ColumnPredicate>,
  setExtra: (
    updater: (current: Record<string, ColumnPredicate>) => Record<string, ColumnPredicate>,
  ) => void,
  handlerCache: Map<string, (next: ColumnPredicate) => void>,
): ToolbarFilterField[] {
  return fields.map((field) => {
    const kind = resolveFunnelKind(field);
    const predicate = extra[field.key] ?? decodeFieldValue(field.value, defaultOp(kind));

    // Reuse the same handler reference if it already exists for this key.
    // This prevents sheetColumns (which reads sheetFilters) from recalculating
    // just because bindPredicates ran again.
    let onPredicate = handlerCache.get(field.key);
    if (!onPredicate) {
      onPredicate = (next: ColumnPredicate) => {
        const isActive = predicateActive(next);
        if (!isActive) {
          setExtra((current) => {
            const copy = { ...current };
            delete copy[field.key];
            return copy;
          });
          field.onChange("");
        } else {
          setExtra((current) => ({ ...current, [field.key]: next }));
          field.onChange(encodePredicateValue(next, field.kind));
        }
      };
      handlerCache.set(field.key, onPredicate);
    }

    return { ...field, funnelKind: kind, predicate, onPredicate };
  });
}


