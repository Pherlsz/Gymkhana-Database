import { Alert, Button, Flex, Layout, Select } from "antd";
import { useQuery } from "@tanstack/react-query";
import { getRouteApi, Link } from "@tanstack/react-router";
import { Search, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { DataGridPagination } from "./DataGrid";
import { normalizeProfileSearch } from "./ProfilesPage";
import { tableLinkProps } from "./lib/tables/tableRoutes";
import { useI18n } from "./i18n";
import {
  APIRequestError,
  executeSearch,
  getSearchCatalog,
  type ProfileListSearch,
  type SearchRequest,
  type SearchResult,
} from "./lib/api/client";

type SearchModule = NonNullable<SearchRequest["modules"]>[number];

export type GlobalSearchState = {
  q: string;
  modules: string;
  fields: string;
  page: number;
  limit: 25 | 50 | 100;
  sort: "relevance" | "updated_at";
  order: "asc" | "desc";
};

const searchRoute = getRouteApi("/search");

const moduleValues: SearchModule[] = [
  "profiles",
  "documents",
  "bills",
  "custom_data",
  "attachments",
];

type ResultGroup = {
  key: string;
  module: SearchResult["module"];
  entityId: string;
  entityLabel: string;
  updatedAt: string;
  topScore: number;
  matches: Array<{ fieldKey: string; fieldLabel: string; preview: string }>;
};

/** §13: the API returns one row per matched field; the surface groups them so
 * the record is the unit of answer (Option B). Order inside the page is kept:
 * groups appear by their best score, matches in API order. */
export function groupSearchResults(results: SearchResult[]): ResultGroup[] {
  const groups = new Map<string, ResultGroup>();
  for (const result of results) {
    const key = `${result.module}:${result.entity_id}`;
    const existing = groups.get(key);
    if (existing) {
      existing.matches.push({
        fieldKey: result.field_key,
        fieldLabel: result.field_label,
        preview: result.preview,
      });
      existing.topScore = Math.max(existing.topScore, result.score);
      continue;
    }
    groups.set(key, {
      key,
      module: result.module,
      entityId: result.entity_id,
      entityLabel: result.entity_label,
      updatedAt: result.updated_at,
      topScore: result.score,
      matches: [
        { fieldKey: result.field_key, fieldLabel: result.field_label, preview: result.preview },
      ],
    });
  }
  return [...groups.values()].toSorted((a, b) => b.topScore - a.topScore);
}

export function normalizeGlobalSearch(search: Record<string, unknown>): GlobalSearchState {
  const requestedLimit = Number(search.limit);
  const limit = requestedLimit === 25 || requestedLimit === 100 ? requestedLimit : 50;
  return {
    q: typeof search.q === "string" ? search.q : "",
    modules: normalizeListParameter(search.modules, (value) =>
      moduleValues.includes(value as SearchModule),
    ),
    fields: normalizeListParameter(search.fields, (value) => value.length > 0),
    page: positiveInteger(search.page, 1),
    limit,
    sort: search.sort === "updated_at" ? "updated_at" : "relevance",
    order: search.order === "asc" ? "asc" : "desc",
  };
}

export const GLOBAL_SEARCH_DEFAULTS = normalizeGlobalSearch({});

export function termsFromSearch(value: string): string[] {
  return value
    .split("\n")
    .map((term) => term.trim())
    .filter(Boolean);
}

export function profileSearchForResult(result: SearchResult): ProfileListSearch | null {
  if (result.target_kind === "profile") {
    return normalizeProfileSearch({ selected: result.target_id, mode: "view" });
  }
  if (!result.profile_id) return null;
  if (result.target_kind === "document") {
    return normalizeProfileSearch({
      selected: result.profile_id,
      mode: "view",
      section: "documents",
      document_selected: result.target_id,
      document_mode: "view",
    });
  }
  if (result.target_kind === "bill") {
    return normalizeProfileSearch({
      selected: result.profile_id,
      mode: "view",
      section: "bills",
      bill_selected: result.target_id,
      bill_mode: "view",
    });
  }
  return normalizeProfileSearch({ selected: result.profile_id, mode: "view" });
}

export function SearchPage() {
  const search = searchRoute.useSearch();
  const navigate = searchRoute.useNavigate();
  const { messages } = useI18n();
  const searchMessages = messages.search;
  const [draft, setDraft] = useState(search.q);
  const inputRef = useRef<HTMLInputElement>(null);
  useEffect(() => setDraft(search.q), [search.q]);

  const catalog = useQuery({
    queryKey: ["search-catalog"],
    queryFn: ({ signal }) => getSearchCatalog(signal),
    staleTime: 60_000,
  });
  const terms = useMemo(() => termsFromSearch(search.q), [search.q]);
  const selectedModules = useMemo(
    () => parseList(search.modules) as SearchModule[],
    [search.modules],
  );
  const selectedFields = useMemo(() => parseList(search.fields), [search.fields]);
  const request = useMemo<SearchRequest>(
    () => ({
      terms,
      ...(selectedModules.length > 0 ? { modules: selectedModules } : {}),
      ...(selectedFields.length > 0 ? { fields: selectedFields } : {}),
      limit: search.limit,
      offset: (search.page - 1) * search.limit,
      sort: search.sort,
      order: search.order,
    }),
    [search.limit, search.order, search.page, search.sort, selectedFields, selectedModules, terms],
  );
  const results = useQuery({
    queryKey: ["global-search", request],
    queryFn: ({ signal }) => executeSearch(request, signal),
    enabled: terms.length > 0 && catalog.isSuccess,
    retry: (attempt, error) =>
      !(error instanceof APIRequestError && [403, 422, 429].includes(error.status)) && attempt < 1,
  });
  const moduleLabels = useMemo(
    () =>
      new Map<string, string>(
        catalog.data?.modules.map((module) => [module.key, module.label] as const) ?? [],
      ),
    [catalog.data?.modules],
  );
  const groups = useMemo(
    () => groupSearchResults(results.data?.results ?? []),
    [results.data?.results],
  );
  const fieldOptions = useMemo(() => {
    const visibleModules =
      selectedModules.length > 0
        ? new Set(selectedModules)
        : new Set(catalog.data?.modules.map((module) => module.key));
    const grouped = new Map<string, Array<{ value: string; label: string }>>();
    for (const field of catalog.data?.fields ?? []) {
      if (!visibleModules.has(field.module)) continue;
      const bucket = grouped.get(field.module) ?? [];
      bucket.push({ value: field.key, label: field.label });
      grouped.set(field.module, bucket);
    }
    return [...grouped.entries()].map(([moduleKey, options]) => ({
      label: moduleLabels.get(moduleKey) ?? moduleKey,
      options,
    }));
  }, [catalog.data?.fields, catalog.data?.modules, moduleLabels, selectedModules]);
  const totalPages = Math.max(
    1,
    Math.min(
      Math.floor(10_000 / search.limit) + 1,
      Math.ceil((results.data?.page.total ?? 0) / search.limit),
    ),
  );

  const updateSearch = (patch: Partial<GlobalSearchState>) => {
    void navigate({
      search: (current) => ({ ...current, ...patch }),
    });
  };
  const submit = () => {
    if (draft.trim().length === 0) {
      inputRef.current?.focus();
      return;
    }
    updateSearch({ q: draft, page: 1 });
  };
  const clear = () => {
    setDraft("");
    inputRef.current?.focus();
    updateSearch({ q: "", modules: "", fields: "", page: 1 });
  };
  const toggleModule = (module: SearchModule) => {
    const allModules = catalog.data?.modules.map((item) => item.key) ?? moduleValues;
    const current = selectedModules.length === 0 ? [...allModules] : [...selectedModules];
    const next = current.includes(module)
      ? current.filter((value) => value !== module)
      : [...new Set([...current, module])];
    const canonicalModules = next.length === allModules.length ? [] : next;
    const allowedFields = new Set(
      catalog.data?.fields
        .filter((field) => canonicalModules.length === 0 || canonicalModules.includes(field.module))
        .map((field) => field.key) ?? [],
    );
    updateSearch({
      modules: serializeList(canonicalModules),
      fields: serializeList(selectedFields.filter((field) => allowedFields.has(field))),
      page: 1,
    });
  };

  const sortValue = `${search.sort}:${search.order}` as const;

  return (
    <Layout className="search-page">
      <header className="search-page__header">
        <h1 className="page-title">{searchMessages.title}</h1>
      </header>
      <section className="search-toolbar">
        <form
          className="search-bar"
          role="search"
          onSubmit={(event) => {
            event.preventDefault();
            submit();
          }}
        >
          <Search aria-hidden size={16} strokeWidth={2} />
          <input
            aria-label={searchMessages.title}
            autoComplete="off"
            placeholder={searchMessages.inputPlaceholder}
            ref={inputRef}
            spellCheck={false}
            type="text"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
          />
          {draft.length > 0 || search.q.length > 0 ? (
            <button
              aria-label={searchMessages.clear}
              className="search-bar__clear"
              type="button"
              onClick={clear}
            >
              <X aria-hidden size={16} strokeWidth={2} />
            </button>
          ) : null}
          <Button disabled={draft.trim().length === 0 || !catalog.isSuccess} htmlType="submit">
            {searchMessages.submit}
          </Button>
        </form>
        <div className="search-modules" role="group" aria-label={searchMessages.modulesLegend}>
          {(catalog.data?.modules ?? []).map((module) => (
            <button
              key={module.key}
              aria-pressed={selectedModules.length === 0 || selectedModules.includes(module.key)}
              className="search-module-chip"
              type="button"
              onClick={() => toggleModule(module.key)}
            >
              {module.label}
            </button>
          ))}
        </div>
        <div className="search-options">
          <label className="search-options__fields">
            <span>{searchMessages.fieldsLabel}</span>
            <Select
              allowClear
              loading={catalog.isLoading}
              maxTagCount="responsive"
              notFoundContent={searchMessages.fieldsEmptyState}
              options={fieldOptions}
              placeholder={searchMessages.fieldsSearchPlaceholder}
              showSearch
              value={selectedFields}
              onChange={(values) =>
                updateSearch({
                  fields: serializeList((values ?? []) as string[]),
                  page: 1,
                })
              }
            />
            <span className="search-options__hint">{searchMessages.fieldsAllAllowed}</span>
          </label>
          <label>
            <span>{searchMessages.sortLabel}</span>
            <Select
              value={sortValue}
              onChange={(value) => {
                const [sort, order] = value.split(":") as [
                  GlobalSearchState["sort"],
                  GlobalSearchState["order"],
                ];
                updateSearch({ sort, order, page: 1 });
              }}
              options={[
                { value: "relevance:desc", label: searchMessages.sortRelevanceDesc },
                { value: "relevance:asc", label: searchMessages.sortRelevanceAsc },
                { value: "updated_at:desc", label: searchMessages.sortUpdatedDesc },
                { value: "updated_at:asc", label: searchMessages.sortUpdatedAsc },
              ]}
            />
          </label>
          <label>
            <span>{searchMessages.perPageLabel}</span>
            <Select
              value={search.limit}
              onChange={(value) => updateSearch({ limit: value as 25 | 50 | 100, page: 1 })}
              options={[
                { value: 25, label: "25" },
                { value: 50, label: "50" },
                { value: 100, label: "100" },
              ]}
            />
          </label>
        </div>
      </section>

      {results.isError ? (
        <Alert
          message={searchMessages.resultsErrorTitle}
          type="error"
          showIcon
          description={<>{searchErrorMessage(results.error)}</>}
        />
      ) : null}
      {catalog.isError ? (
        <Alert
          message={searchMessages.catalogErrorTitle}
          type="error"
          showIcon
          description={<>{searchErrorMessage(catalog.error)}</>}
        />
      ) : null}

      <section aria-label={searchMessages.resultsCaption} className="search-results">
        {results.isFetching ? (
          <p className="search-results__status">{searchMessages.loadingResults}</p>
        ) : null}
        {!results.isFetching && terms.length > 0 && groups.length === 0 && !results.isError ? (
          <p className="search-results__status">{searchMessages.noResults}</p>
        ) : null}
        {groups.length > 0 ? (
          <div className="search-grid">
            {groups.map((group) => (
              <SearchResultCard
                key={group.key}
                group={group}
                moduleLabel={moduleLabels.get(group.module) ?? group.module}
              />
            ))}
          </div>
        ) : null}
      </section>
      {results.data ? (
        <Flex justify="center">
          <DataGridPagination
            label={searchMessages.resultsUnit}
            onPage={(page) => updateSearch({ page })}
            page={search.page}
            total={results.data.page.total}
            totalPages={totalPages}
          />
        </Flex>
      ) : null}
    </Layout>
  );
}

function SearchResultCard({ group, moduleLabel }: { group: ResultGroup; moduleLabel: string }) {
  const { messages } = useI18n();
  const searchMessages = messages.search;
  const profileSearch = profileSearchForResult(group as unknown as SearchResult);
  return (
    <article className="search-card">
      <header className="search-card__head">
        <span className="search-card__kind">{moduleLabel}</span>
        <span className="search-card__entity">{group.entityLabel}</span>
        <span className="search-card__score">
          {searchMessages.relevanceWord} {group.topScore}
        </span>
      </header>
      <ul className="search-card__matches">
        {group.matches.map((match) => (
          <li key={match.fieldKey} className="search-card__match">
            <span className="search-card__field">{match.fieldLabel}</span>
            <span className="search-card__preview">{match.preview}</span>
          </li>
        ))}
      </ul>
      <footer className="search-card__foot">
        {profileSearch ? (
          <Link className="search-result__link" {...tableLinkProps(profileSearch)}>
            {searchMessages.openRecord}
          </Link>
        ) : null}
        <span className="search-card__updated">
          {new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(
            new Date(group.updatedAt),
          )}
        </span>
      </footer>
    </article>
  );
}

function normalizeListParameter(value: unknown, allowed: (value: string) => boolean): string {
  if (typeof value !== "string") return "";
  return serializeList(parseList(value).filter(allowed));
}

function parseList(value: string): string[] {
  return [
    ...new Set(
      value
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  ];
}

function serializeList(values: readonly string[]): string {
  return [...new Set(values)].toSorted().join(",");
}

function positiveInteger(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : fallback;
}

export function searchErrorMessage(error: unknown): string {
  if (error instanceof APIRequestError) {
    switch (error.code) {
      case "rate_limited":
        return "O limite de buscas foi atingido. Aguarde um minuto e tente novamente.";
      case "query_too_costly":
        return "A busca ficou ampla demais. Reduza módulos, campos, termos ou a página.";
      case "result_set_too_large":
        return "A busca encontrou candidatos demais. Use módulos, campos ou termos mais específicos.";
      case "search_timeout":
        return "A busca excedeu o tempo seguro. Tente usar filtros mais específicos.";
      case "forbidden":
        return "Você não possui permissão para consultar este escopo.";
      default:
        if (error.fieldErrors.length > 0)
          return error.fieldErrors.map((field) => field.message).join(" · ");
        return error.message;
    }
  }
  return error instanceof Error ? error.message : "Erro inesperado.";
}
