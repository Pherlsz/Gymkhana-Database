/**
 * Fábrica centralizada e padronizada de chaves de query para TanStack Query.
 * Elimina o uso de strings soltas e inconsistências em invalidações de cache.
 */
export const queryKeys = {
  tables: {
    all: ["tables"] as const,
    profiles: (params?: unknown) =>
      params !== undefined
        ? (["tables", "profiles", params] as const)
        : (["tables", "profiles"] as const),
    documents: (params?: unknown, owner?: unknown) =>
      params !== undefined || owner !== undefined
        ? (["tables", "documents", params, owner] as const)
        : (["tables", "documents"] as const),
    bills: (params?: unknown, owner?: unknown) =>
      params !== undefined || owner !== undefined
        ? (["tables", "bills", params, owner] as const)
        : (["tables", "bills"] as const),
  },
  profiles: {
    all: ["profile"] as const,
    detail: (id?: string) =>
      id !== undefined ? (["profile", id] as const) : (["profile"] as const),
    lookup: (q?: string) => ["profiles-lookup", q] as const,
    cityOptions: (...filters: unknown[]) => ["profile-city-options", ...filters] as const,
  },
  types: {
    documents: ["document-types"] as const,
    bills: ["bill-types"] as const,
  },
  records: {
    documents: (ownerId?: string, search?: unknown) =>
      search !== undefined
        ? (["documents", ownerId, search] as const)
        : ownerId !== undefined
          ? (["documents", ownerId] as const)
          : (["documents"] as const),
    bills: (ownerId?: string, search?: unknown) =>
      search !== undefined
        ? (["bills", ownerId, search] as const)
        : ownerId !== undefined
          ? (["bills", ownerId] as const)
          : (["bills"] as const),
  },
  customData: {
    fields: (kind: string, id?: string) => ["custom-fields", kind, id ?? "global"] as const,
    values: (kind: string, id?: string) => ["custom-values", kind, id ?? "new"] as const,
    options: (fieldId: string) => ["custom-options", fieldId] as const,
  },
  search: {
    catalog: ["search-catalog"] as const,
    global: (request?: unknown) => ["global-search", request] as const,
    suggest: (grain?: string, hint?: string, q?: string) =>
      ["search-suggest", grain, hint, q] as const,
  },
  ocr: {
    capability: ["ocr-capability"] as const,
    job: (jobId?: string) => ["ocr-job", jobId] as const,
    suggestions: (jobId?: string) => ["ocr-suggestions", jobId] as const,
  },
  googleForms: {
    status: ["google-forms-status"] as const,
    sources: ["google-forms-sources"] as const,
    syncs: ["google-forms-syncs"] as const,
  },
  operations: {
    catalog: ["operations-catalog"] as const,
    import: (id?: string) => ["operation-import", id] as const,
    importReport: (id?: string) => ["operation-import-report", id] as const,
    importsList: ["operation-imports"] as const,
  },
  attachments: {
    enabled: ["attachments-enabled"] as const,
    byOwner: (owner: unknown, flag?: boolean) =>
      flag !== undefined
        ? (["attachments", owner, flag] as const)
        : (["attachments", owner] as const),
  },
  admin: {
    featureFlags: ["admin", "feature-flags"] as const,
  },
  cadastro: {
    minimumRequirement: (ownerId?: string) =>
      ownerId !== undefined
        ? (["cadastro-minimum-requirement", ownerId] as const)
        : (["cadastro-minimum-requirement"] as const),
    entryFormsSources: ["cadastro-entry-forms-sources"] as const,
    entryImports: ["cadastro-entry-imports"] as const,
  },
  home: {
    documentTypes: ["home", "overview", "document-types"] as const,
    billTypes: ["home", "overview", "bill-types"] as const,
    profiles: ["home", "overview", "profiles"] as const,
    documentsInUse: ["home", "overview", "documents-in-use"] as const,
    billsInUse: ["home", "overview", "bills-in-use"] as const,
  },
} as const;
