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

/**
 * List endpoints only understand contains, `^` (starts with) and `=` (equals)
 * on text params, and an exact value on select params. Anything else is
 * evaluated in the browser on the loaded page.
 */
export function serverAppliesPredicate(
  field: ToolbarFilterField,
  predicate: ColumnPredicate = fieldPredicate(field),
): boolean {
  if (field.local) return false;
  if (!predicateActive(predicate)) return true;
  if (field.kind === "select") return predicate.op === "eq";
  if (field.kind === "text") {
    return predicate.op === "contains" || predicate.op === "starts_with" || predicate.op === "eq";
  }
  return false;
}

export function filterAppliesLocally(field: ToolbarFilterField): boolean {
  return !serverAppliesPredicate(field);
}

/** Cells show option labels (Sim, Disponível, type names), not the stored values. */
export function clientPredicate(field: ToolbarFilterField): ColumnPredicate {
  const predicate = fieldPredicate(field);
  if (field.kind !== "select") return predicate;
  const options = [...field.options, ...(field.groups ?? []).flatMap((group) => group.options)];
  return {
    ...predicate,
    values: predicate.values.map(
      (value) => options.find((option) => option.value === value)?.label ?? value,
    ),
  };
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
          field.onChange(
            serverAppliesPredicate(field, next) || field.local
              ? encodePredicateValue(next, field.kind)
              : "",
          );
        }
      },
    };
  });
}
