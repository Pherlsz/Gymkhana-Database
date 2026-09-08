import type { SearchCatalogResponse } from "../api/client";
import type { SearchModule } from "./types";

type CatalogField = SearchCatalogResponse["fields"][number];

export type FieldFilterOption = {
  value: string;
  label: string;
  title: string;
  scope?: string;
};

export type FieldFilterGroup = {
  key: string;
  label: string;
  owner: SearchModule;
  options: FieldFilterOption[];
};

const OWNER_ORDER: SearchModule[] = [
  "profiles",
  "documents",
  "bills",
  "custom_data",
  "attachments",
];

export function searchModuleChips(
  modules: SearchCatalogResponse["modules"] | undefined,
): SearchCatalogResponse["modules"] {
  return (modules ?? []).filter((module) => module.key !== "custom_data");
}

export function fieldOwnerModule(field: CatalogField): SearchModule {
  return (field.group ?? field.module) as SearchModule;
}

function optionFromField(field: CatalogField): FieldFilterOption {
  const separator = field.label.indexOf(" · ");
  if (separator === -1) {
    return { value: field.key, label: field.label, title: field.label };
  }
  return {
    value: field.key,
    label: field.label,
    title: field.label.slice(0, separator),
    scope: field.label.slice(separator + 3),
  };
}

export function fieldContextBucket(
  field: CatalogField,
  moduleLabels: Map<string, string>,
): { key: string; label: string; owner: SearchModule } {
  const owner = fieldOwnerModule(field);
  const option = optionFromField(field);
  if (field.key.startsWith("custom.") && option.scope && owner !== "profiles") {
    return { key: `${owner}:${option.scope}`, label: option.scope, owner };
  }
  return {
    key: owner,
    label: moduleLabels.get(owner) ?? owner,
    owner,
  };
}

export function buildFieldFilterGroups(
  fields: CatalogField[] | undefined,
  moduleLabels: Map<string, string>,
  selectedModules: SearchModule[],
  allModuleKeys: SearchModule[],
): FieldFilterGroup[] {
  const visible = selectedModules.length > 0 ? new Set(selectedModules) : new Set(allModuleKeys);
  const grouped = new Map<string, FieldFilterGroup>();

  for (const field of fields ?? []) {
    const bucket = fieldContextBucket(field, moduleLabels);
    if (!visible.has(bucket.owner)) continue;
    const existing = grouped.get(bucket.key);
    const option = optionFromField(field);
    if (existing) {
      existing.options.push(option);
      continue;
    }
    grouped.set(bucket.key, {
      key: bucket.key,
      label: bucket.label,
      owner: bucket.owner,
      options: [option],
    });
  }

  return [...grouped.values()].sort((left, right) => {
    const leftOwner = OWNER_ORDER.indexOf(left.owner);
    const rightOwner = OWNER_ORDER.indexOf(right.owner);
    if (leftOwner !== rightOwner) return leftOwner - rightOwner;
    const leftCanonical = left.key === left.owner ? 0 : 1;
    const rightCanonical = right.key === right.owner ? 0 : 1;
    if (leftCanonical !== rightCanonical) return leftCanonical - rightCanonical;
    return left.label.localeCompare(right.label, "pt-BR");
  });
}
