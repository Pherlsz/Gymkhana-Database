import { Alert, Card, Flex, Input, Layout, Pagination, Select } from "antd";
import { useQuery } from "@tanstack/react-query";
import { getRouteApi, Link } from "@tanstack/react-router";
import { Check, ChevronDown } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
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

type MatchRow = { fieldKey: string; fieldLabel: string; preview: string };

/** One row per matched field, grouped so the record is the unit of answer. */
export type ResultGroup = {
  key: string;
  module: SearchResult["module"];
  entityId: string;
  entityLabel: string;
  updatedAt: string;
  topScore: number;
  matches: MatchRow[];
};

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

/** Legacy-style cards: one card per profile; person fields emphasized and the
 * document/bill/custom hits that belong to that profile listed inside. */
export type ProfileCard = {
  key: string;
  profileId: string;
  profileLabel: string;
  updatedAt: string;
  topScore: number;
  profileMatches: MatchRow[];
  relatedGroups: ResultGroup[];
};

export function buildProfileCards(results: SearchResult[]): ProfileCard[] {
  const labelsByProfileId = new Map<string, { label: string; updatedAt: string }>();
  for (const result of results) {
    if (result.module === "profiles") {
      const existing = labelsByProfileId.get(result.entity_id);
      if (!existing || result.score > 0) {
        labelsByProfileId.set(result.entity_id, {
          label: result.entity_label,
          updatedAt: result.updated_at,
        });
      }
    }
  }

  const cards = new Map<string, ProfileCard>();
  for (const group of groupSearchResults(results)) {
    const anchorId =
      group.module === "profiles"
        ? group.entityId
        : (results.find(
            (result) => result.module !== "profiles" && result.entity_id === group.entityId,
          )?.profile_id ?? "");
    if (!anchorId) continue;
    const anchorMeta =
      labelsByProfileId.get(anchorId) ?? ({ label: anchorId, updatedAt: group.updatedAt } as const);
    let card = cards.get(anchorId);
    if (!card) {
      card = {
        key: anchorId,
        profileId: anchorId,
        profileLabel: anchorMeta.label,
        updatedAt: anchorMeta.updatedAt,
        topScore: group.topScore,
        profileMatches: [],
        relatedGroups: [],
      };
      cards.set(anchorId, card);
    }
    card.updatedAt = [card.updatedAt, anchorMeta.updatedAt].toSorted().at(-1) ?? card.updatedAt;
    card.topScore = Math.max(card.topScore, group.topScore);
    if (group.module === "profiles") {
      card.profileMatches.push(...group.matches);
    } else {
      card.relatedGroups.push(group);
    }
  }
  return [...cards.values()].toSorted((a, b) => b.topScore - a.topScore);
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

function groupAsResult(group: ResultGroup): SearchResult {
  const first = group.matches[0];
  return {
    module: group.module,
    // The API's own kind is preserved by callers through profile_id routing in
    // profileSearchForResult; this synthetic row only carries navigation data.
    entity_kind:
      group.module === "documents"
        ? "document"
        : group.module === "bills"
          ? "bill"
          : "custom_entity",
    entity_id: group.entityId,
    target_kind:
      group.module === "documents" ? "document" : group.module === "bills" ? "bill" : "profile",
    target_id: group.entityId,
    entity_label: group.entityLabel,
    field_key: first?.fieldKey ?? "",
    field_label: first?.fieldLabel ?? "",
    preview: first?.preview ?? "",
    score: group.topScore,
    updated_at: group.updatedAt,
  };
}

export function SearchPage() {
  const search = searchRoute.useSearch();
  const navigate = searchRoute.useNavigate();
  const { messages } = useI18n();
  const searchMessages = messages.search;
  const [draft, setDraft] = useState(search.q);
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
  const cards = useMemo(
    () => buildProfileCards(results.data?.results ?? []),
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

  const updateSearch = (patch: Partial<GlobalSearchState>) => {
    void navigate({
      search: (current) => ({ ...current, ...patch }),
    });
  };
  const submit = () => updateSearch({ q: draft, page: 1 });
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
        <Input.Search
          allowClear
          enterButton={searchMessages.submit}
          placeholder={searchMessages.inputPlaceholder}
          size="large"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          onSearch={() => submit()}
        />
        <div className="search-filters">
          <div className="search-modules" role="group" aria-label={searchMessages.modulesLegend}>
            {(catalog.data?.modules ?? []).map((module) => (
              <button
                key={module.key}
                aria-pressed={selectedModules.length === 0 || selectedModules.includes(module.key)}
                className="search-module-chip"
                type="button"
                onClick={() => toggleModule(module.key)}
              >
                {selectedModules.length === 0 || selectedModules.includes(module.key) ? (
                  <Check aria-hidden size={12} strokeWidth={2.5} />
                ) : null}
                {module.label}
              </button>
            ))}
          </div>
          <label className="search-filter">
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
          </label>
          <label className="search-filter">
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
        {!results.isFetching && terms.length > 0 && cards.length === 0 && !results.isError ? (
          <p className="search-results__status">{searchMessages.noResults}</p>
        ) : null}
        <div className="search-list">
          {cards.map((card) => (
            <ProfileResultCard card={card} key={card.key} moduleLabels={moduleLabels} />
          ))}
        </div>
      </section>
      {results.data && results.data.page.total > 0 ? (
        <Pagination
          current={search.page}
          pageSize={search.limit}
          showLessItems
          showSizeChanger={false}
          size="small"
          total={Math.min(
            results.data.page.total,
            totalPagesOf(results.data.page.total, search.limit) * search.limit,
          )}
          onChange={(page) => updateSearch({ page })}
        />
      ) : null}
    </Layout>
  );
}

const totalPagesOf = (total: number, limit: number) =>
  Math.max(1, Math.min(Math.floor(10_000 / limit) + 1, Math.ceil(total / limit)));

function ProfileResultCard({
  card,
  moduleLabels,
}: {
  card: ProfileCard;
  moduleLabels: Map<string, string>;
}) {
  const { messages } = useI18n();
  const searchMessages = messages.search;
  const [open, setOpen] = useState(false);
  const hasRelated = card.relatedGroups.length > 0;
  return (
    <Card className="profile-card" size="small">
      <Flex align="center" gap="0.5rem" wrap="wrap">
        <span className="profile-card__kind">{moduleLabels.get("profiles") ?? "Pessoas"}</span>
        <Link
          className="profile-card__name"
          {...tableLinkProps(normalizeProfileSearch({ selected: card.profileId, mode: "view" }))}
        >
          {card.profileLabel}
        </Link>
        <span className="profile-card__spacer" />
        <time className="profile-card__updated" dateTime={card.updatedAt}>
          {new Intl.DateTimeFormat("pt-BR", { dateStyle: "short" }).format(
            new Date(card.updatedAt),
          )}
        </time>
      </Flex>
      {card.profileMatches.slice(0, 3).map((match) => (
        <p key={match.fieldKey} className="profile-card__match">
          <span className="profile-card__match-label">{match.fieldLabel}</span>
          <strong className="profile-card__match-value">{match.preview}</strong>
        </p>
      ))}
      {hasRelated ? (
        <>
          <button
            aria-expanded={open}
            className="profile-card__toggle"
            type="button"
            onClick={() => setOpen(!open)}
          >
            <ChevronDown aria-hidden className="profile-card__chevron" size={14} strokeWidth={2} />
            {open
              ? searchMessages.hideRelated
              : `${searchMessages.showRelated}: ${card.relatedGroups.length}`}
          </button>
          {open ? (
            <ul className="profile-card__related">
              {card.relatedGroups.map((group) => (
                <li key={group.key} className="profile-card__related-row">
                  <span className="profile-card__related-kind">
                    {moduleLabels.get(group.module) ?? group.module}
                  </span>
                  <span className="profile-card__related-label">{group.entityLabel}</span>
                  <span className="profile-card__related-preview">{group.matches[0]?.preview}</span>
                  <ProfileSearchLink result={groupAsResult(group)} />
                </li>
              ))}
            </ul>
          ) : null}
        </>
      ) : null}
    </Card>
  );
}

function ProfileSearchLink({ result }: { result: SearchResult }) {
  const { messages } = useI18n();
  const profileSearch = profileSearchForResult(result);
  if (!profileSearch) return null;
  return (
    <Link className="profile-card__open" {...tableLinkProps(profileSearch)}>
      {messages.search.openRecord}
    </Link>
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
