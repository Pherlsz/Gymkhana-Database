const TABLE_KINDS = ["people", "documents", "bills"] as const;
export type TableKind = (typeof TABLE_KINDS)[number];

function isTableKind(value: string): value is TableKind {
  return (TABLE_KINDS as readonly string[]).includes(value);
}

export const CADASTRO_MODES = ["manual", "xlsx", "forms"] as const;
export type CadastroMode = (typeof CADASTRO_MODES)[number];

export const GOOGLE_FORMS_TABS = ["sources", "history"] as const;
export type GoogleFormsTab = (typeof GOOGLE_FORMS_TABS)[number];

export const GOOGLE_FORMS_OAUTH_FLAGS = ["connected", "denied"] as const;
export type GoogleFormsOAuthFlag = (typeof GOOGLE_FORMS_OAUTH_FLAGS)[number];

const importIDPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export function isCadastroMode(value: unknown): value is CadastroMode {
  return typeof value === "string" && (CADASTRO_MODES as readonly string[]).includes(value);
}

export function isCadastroTypeId(value: string): boolean {
  return importIDPattern.test(value);
}

function isGoogleFormsTab(value: unknown): value is GoogleFormsTab {
  return typeof value === "string" && (GOOGLE_FORMS_TABS as readonly string[]).includes(value);
}

function isGoogleFormsOAuthFlag(value: unknown): value is GoogleFormsOAuthFlag {
  return (
    typeof value === "string" && (GOOGLE_FORMS_OAUTH_FLAGS as readonly string[]).includes(value)
  );
}

export type CadastroPageSearch = {
  table: TableKind;
  mode?: CadastroMode | undefined;
  import?: string | undefined;
  owner?: string | undefined;
  record?: string | undefined;
  type?: string | undefined;
  tab?: GoogleFormsTab | undefined;
  source?: string | undefined;
  google_forms?: GoogleFormsOAuthFlag | undefined;
};

export const CADASTRO_SEARCH_DEFAULTS: CadastroPageSearch = { table: "people" };

export function normalizeCadastroPageSearch(search: Record<string, unknown>): CadastroPageSearch {
  const table = isTableKind(String(search.table ?? "")) ? (search.table as TableKind) : "people";
  const mode = isCadastroMode(search.mode) ? search.mode : undefined;
  const rawImport = typeof search.import === "string" ? search.import.trim() : "";
  const importId = importIDPattern.test(rawImport) ? rawImport : undefined;
  const owner =
    typeof search.owner === "string" && search.owner.trim() ? search.owner.trim() : undefined;
  const rawRecord = typeof search.record === "string" ? search.record.trim() : "";
  const record = importIDPattern.test(rawRecord) ? rawRecord : undefined;
  const rawType = typeof search.type === "string" ? search.type.trim() : "";
  const typeId = importIDPattern.test(rawType) ? rawType : undefined;
  const result: CadastroPageSearch = { table };
  if (mode) result.mode = mode;
  if (importId) result.import = importId;
  if (owner) result.owner = owner;
  if (record) result.record = record;
  if (table !== "people" && typeId) result.type = typeId;
  if (mode === "forms") {
    if (isGoogleFormsTab(search.tab)) result.tab = search.tab;
    const rawSource = typeof search.source === "string" ? search.source.trim() : "";
    if (importIDPattern.test(rawSource)) result.source = rawSource;
    if (isGoogleFormsOAuthFlag(search.google_forms)) result.google_forms = search.google_forms;
  }
  return result;
}

function compactCadastroSearch(search: CadastroPageSearch): Partial<CadastroPageSearch> {
  const next: Partial<CadastroPageSearch> = {};
  if (search.table !== CADASTRO_SEARCH_DEFAULTS.table) next.table = search.table;
  if (search.mode) next.mode = search.mode;
  if (search.import) next.import = search.import;
  if (search.owner) next.owner = search.owner;
  if (search.record) next.record = search.record;
  if (search.type) next.type = search.type;
  if (search.mode === "forms") {
    if (search.tab) next.tab = search.tab;
    if (search.source) next.source = search.source;
    if (search.google_forms) next.google_forms = search.google_forms;
  }
  return next;
}

function serializeCadastroSearch(search: CadastroPageSearch): string {
  const compact = compactCadastroSearch(search);
  const params = new URLSearchParams();
  if (compact.table) params.set("table", compact.table);
  if (compact.mode) params.set("mode", compact.mode);
  if (compact.import) params.set("import", compact.import);
  if (compact.owner) params.set("owner", compact.owner);
  if (compact.record) params.set("record", compact.record);
  if (compact.type) params.set("type", compact.type);
  if (compact.tab) params.set("tab", compact.tab);
  if (compact.source) params.set("source", compact.source);
  if (compact.google_forms) params.set("google_forms", compact.google_forms);
  return params.toString();
}

export function cadastroWorkMode(search: CadastroPageSearch): CadastroMode {
  if (search.mode) return search.mode;
  return "manual";
}

export function cadastroHref(search: CadastroPageSearch): string {
  const query = serializeCadastroSearch(search);
  return query ? `/cadastro?${query}` : "/cadastro";
}

export function googleFormsReturnPath(
  href = typeof window === "undefined" ? "" : window.location.href,
): string {
  const current = href ? new URL(href) : new URL("https://local.test/cadastro");
  const tab = current.searchParams.get("tab");
  const source = current.searchParams.get("source");
  const rawTable = current.searchParams.get("table") ?? "";
  return cadastroHref({
    table: isTableKind(rawTable) ? rawTable : "people",
    mode: "forms",
    tab: isGoogleFormsTab(tab) ? tab : undefined,
    source: source && importIDPattern.test(source) ? source : undefined,
  });
}

export function moduleFromTable(table: TableKind) {
  if (table === "documents") return "DOCUMENTS" as const;
  if (table === "bills") return "BILLS" as const;
  return "PROFILES" as const;
}

export function tableFromModule(module: "PROFILES" | "DOCUMENTS" | "BILLS") {
  if (module === "DOCUMENTS") return "documents" as const;
  if (module === "BILLS") return "bills" as const;
  return "people" as const;
}

export function cadastroReviewHref(module: "PROFILES" | "DOCUMENTS" | "BILLS", importId: string) {
  const table = tableFromModule(module);
  return `/cadastro?table=${table}&mode=xlsx&import=${encodeURIComponent(importId)}`;
}
