import type { QueryCatalog, QueryFilterNode, QueryPlan, QuerySort } from "./api/query";

export const maximumQueryURLPlanCharacters = 16_384;

export type QuerySearch = {
  plan?: string;
  result_page: number;
};

export type RecoveredQueryPlan = {
  plan: QueryPlan;
  issues: string[];
  catalogChanged: boolean;
};

export function normalizeQuerySearch(search: Record<string, unknown>): QuerySearch {
  const encoded = typeof search.plan === "string" ? search.plan : undefined;
  const page = Number(search.result_page);
  return {
    ...(encoded && decodeQueryURLPlan(encoded) ? { plan: encoded } : {}),
    result_page: Number.isSafeInteger(page) && page > 0 && page <= 5 ? page : 1,
  };
}

export function encodeQueryURLPlan(plan: QueryPlan): string | undefined {
  const encoded = encodeBase64URL(JSON.stringify(plan));
  return encoded.length <= maximumQueryURLPlanCharacters ? encoded : undefined;
}

export function decodeQueryURLPlan(encoded: string | undefined): QueryPlan | undefined {
  if (!encoded || encoded.length > maximumQueryURLPlanCharacters) return undefined;
  try {
    const value: unknown = JSON.parse(decodeBase64URL(encoded));
    return isQueryPlan(value) ? value : undefined;
  } catch {
    return undefined;
  }
}

function encodeBase64URL(value: string): string {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  for (const byte of bytes) binary += String.fromCodePoint(byte);
  return btoa(binary).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

function decodeBase64URL(value: string): string {
  const normalized = value.replaceAll("-", "+").replaceAll("_", "/");
  const binary = atob(normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "="));
  const bytes = Uint8Array.from(binary, (character) => character.codePointAt(0) ?? 0);
  return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
}

export function recoverQueryURLPlan(
  source: QueryPlan | undefined,
  catalog: QueryCatalog,
): RecoveredQueryPlan | undefined {
  const root = catalog.entities.some((entity) => entity.key === source?.root_entity)
    ? source!.root_entity
    : (catalog.entities.find((entity) => entity.key === "profiles")?.key ??
      catalog.entities[0]?.key);
  if (!root) return undefined;

  const issues: string[] = [];
  if (source && source.root_entity !== root) issues.push("entidade raiz");
  const projectable = catalog.fields.filter((field) => field.entity === root && field.projectable);
  const permittedProjectionKeys = new Set(projectable.map((field) => field.key));
  const projections = unique(source?.projections ?? [])
    .filter((key) => permittedProjectionKeys.has(key))
    .slice(0, catalog.limits.maximum_projections);
  if (
    source &&
    (projections.length !== source.projections.length ||
      projections.some((key, index) => key !== source.projections[index]))
  ) {
    issues.push("colunas");
  }
  if (projections.length === 0 && (!source || source.projections.length > 0)) {
    projections.push(
      ...projectable.slice(0, Math.min(3, projectable.length)).map((field) => field.key),
    );
  }

  const sort = recoverSort(source?.sort ?? [], root, catalog, issues);
  const filterState = { nodes: 0, issues };
  const filter = source?.filter
    ? recoverFilter(source.filter, root, catalog, "filtro", 1, 0, filterState)
    : undefined;
  const maximumRows = Math.min(
    catalog.limits.maximum_rows,
    Math.max(1, source?.maximum_rows ?? 100),
  );
  if (source && maximumRows !== source.maximum_rows) issues.push("limite de linhas");

  return {
    plan: {
      version: "v1",
      catalog_version: catalog.version,
      root_entity: root,
      projections,
      ...(filter ? { filter } : {}),
      ...(sort.length > 0 ? { sort } : {}),
      maximum_rows: maximumRows,
    },
    issues: unique(issues),
    catalogChanged: Boolean(source && source.catalog_version !== catalog.version),
  };
}

function recoverSort(
  source: QuerySort[],
  root: string,
  catalog: QueryCatalog,
  issues: string[],
): QuerySort[] {
  const sortable = new Set(
    catalog.fields
      .filter((field) => field.entity === root && field.sortable)
      .map((field) => field.key),
  );
  const seen = new Set<string>();
  const recovered = source
    .filter((value) => {
      if (!sortable.has(value.field) || seen.has(value.field)) return false;
      seen.add(value.field);
      return true;
    })
    .slice(0, catalog.limits.maximum_sort_fields);
  if (recovered.length !== source.length) issues.push("ordenação");
  return recovered;
}

type FilterRecoveryState = {
  nodes: number;
  issues: string[];
};

function recoverFilter(
  node: QueryFilterNode,
  entity: string,
  catalog: QueryCatalog,
  path: string,
  depth: number,
  relationDepth: number,
  state: FilterRecoveryState,
): QueryFilterNode | undefined {
  state.nodes += 1;
  if (
    state.nodes > catalog.limits.maximum_filter_nodes ||
    depth > catalog.limits.maximum_filter_depth
  ) {
    state.issues.push(path);
    return undefined;
  }
  if (node.kind === "predicate") {
    const field = catalog.fields.find(
      (value) => value.key === node.field && value.entity === entity && value.filterable,
    );
    const operator = catalog.operators.find(
      (value) => value.key === node.operator && field?.operators.includes(value.key),
    );
    const values = node.values ?? [];
    const options = new Set(field?.options?.map((option) => option.key) ?? []);
    const hasInvalidOption =
      options.size > 0 && values.some((value) => value !== "" && !options.has(value));
    if (
      !field ||
      !operator ||
      values.length < operator.minimum_values ||
      values.length > operator.maximum_values ||
      hasInvalidOption
    ) {
      state.issues.push(path);
      return undefined;
    }
    return { kind: "predicate", field: field.key, operator: operator.key, values };
  }
  if (node.kind === "group") {
    const children = (node.children ?? []).flatMap((child, index) => {
      const recovered = recoverFilter(
        child,
        entity,
        catalog,
        `${path}.${index + 1}`,
        depth + 1,
        relationDepth,
        state,
      );
      return recovered ? [recovered] : [];
    });
    if (children.length === 0) {
      state.issues.push(path);
      return undefined;
    }
    return {
      kind: "group",
      conjunction: node.conjunction === "OR" ? "OR" : "AND",
      children,
    };
  }
  if (node.kind === "not") {
    const child = node.children?.[0];
    const recovered = child
      ? recoverFilter(child, entity, catalog, `${path}.negação`, depth + 1, relationDepth, state)
      : undefined;
    if (!recovered) {
      state.issues.push(path);
      return undefined;
    }
    return { kind: "not", children: [recovered] };
  }
  const relation = catalog.relations.find(
    (value) => value.key === node.relation && value.from_entity === entity,
  );
  if (!relation || relationDepth >= catalog.limits.maximum_relation_depth) {
    state.issues.push(path);
    return undefined;
  }
  const child = node.children?.[0];
  const recovered = child
    ? recoverFilter(
        child,
        relation.to_entity,
        catalog,
        `${path}.${relation.label}`,
        depth + 1,
        relationDepth + 1,
        state,
      )
    : undefined;
  if (!recovered) {
    state.issues.push(path);
    return undefined;
  }
  return { kind: "relation", relation: relation.key, children: [recovered] };
}

function isQueryPlan(value: unknown): value is QueryPlan {
  if (
    !isRecord(value) ||
    !exactKeys(value, [
      "version",
      "catalog_version",
      "root_entity",
      "projections",
      "filter",
      "sort",
      "maximum_rows",
    ])
  ) {
    return false;
  }
  if (
    value.version !== "v1" ||
    typeof value.catalog_version !== "string" ||
    !/^[a-f0-9]{64}$/.test(value.catalog_version) ||
    !isLogicalKey(value.root_entity) ||
    !Array.isArray(value.projections) ||
    value.projections.length > 20 ||
    !value.projections.every(isLogicalKey) ||
    unique(value.projections).length !== value.projections.length ||
    !Number.isSafeInteger(value.maximum_rows) ||
    Number(value.maximum_rows) < 1 ||
    Number(value.maximum_rows) > 500
  ) {
    return false;
  }
  if (value.sort !== undefined && !isSort(value.sort)) return false;
  if (value.filter !== undefined) {
    const state = { nodes: 0 };
    if (!isFilter(value.filter, 1, 0, state)) return false;
  }
  return true;
}

function isSort(value: unknown): value is QuerySort[] {
  return (
    Array.isArray(value) &&
    value.length <= 3 &&
    value.every(
      (item) =>
        isRecord(item) &&
        exactKeys(item, ["field", "direction"]) &&
        isLogicalKey(item.field) &&
        (item.direction === "asc" || item.direction === "desc"),
    ) &&
    unique(value.map((item: QuerySort) => item.field)).length === value.length
  );
}

function isFilter(
  value: unknown,
  depth: number,
  relationDepth: number,
  state: { nodes: number },
): value is QueryFilterNode {
  if (!isRecord(value) || depth > 6) return false;
  state.nodes += 1;
  if (state.nodes > 40) return false;
  if (value.kind === "predicate") {
    return (
      exactKeys(value, ["kind", "field", "operator", "values"]) &&
      isLogicalKey(value.field) &&
      isOperator(value.operator) &&
      Array.isArray(value.values) &&
      value.values.length <= 25 &&
      value.values.every(
        (item) =>
          typeof item === "string" && item.length <= 500 && !hasUnsafeControlCharacter(item),
      )
    );
  }
  if (value.kind === "group") {
    return (
      exactKeys(value, ["kind", "conjunction", "children"]) &&
      (value.conjunction === "AND" || value.conjunction === "OR") &&
      Array.isArray(value.children) &&
      value.children.length >= 1 &&
      value.children.length <= 40 &&
      value.children.every((child) => isFilter(child, depth + 1, relationDepth, state))
    );
  }
  if (value.kind === "not") {
    return (
      exactKeys(value, ["kind", "children"]) &&
      Array.isArray(value.children) &&
      value.children.length === 1 &&
      isFilter(value.children[0], depth + 1, relationDepth, state)
    );
  }
  return (
    value.kind === "relation" &&
    relationDepth < 3 &&
    exactKeys(value, ["kind", "relation", "children"]) &&
    isLogicalKey(value.relation) &&
    Array.isArray(value.children) &&
    value.children.length === 1 &&
    isFilter(value.children[0], depth + 1, relationDepth + 1, state)
  );
}

function isOperator(value: unknown): boolean {
  return [
    "eq",
    "neq",
    "contains",
    "starts_with",
    "gt",
    "gte",
    "lt",
    "lte",
    "between",
    "in",
    "is_null",
    "not_null",
  ].includes(String(value));
}

function hasUnsafeControlCharacter(value: string): boolean {
  for (const character of value) {
    const code = character.codePointAt(0) ?? 0;
    if (code < 32 || code === 127) return true;
  }
  return false;
}

function isLogicalKey(value: unknown): value is string {
  return typeof value === "string" && /^[a-z][A-Za-z0-9_.-]{1,199}$/.test(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function exactKeys(value: Record<string, unknown>, allowed: string[]): boolean {
  return Object.keys(value).every((key) => allowed.includes(key));
}

function unique<T>(values: T[]): T[] {
  return [...new Set(values)];
}
