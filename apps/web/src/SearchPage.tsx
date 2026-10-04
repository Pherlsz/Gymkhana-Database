import { Button, Pagination, Select } from "antd";
import { InlineStatus } from "./components/InlineStatus";
import { PageShell } from "./components/PageShell";
import { StateCard } from "./components/StateCard";
import { StatusBanner } from "./components/StatusBanner";
import "./search.css";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { getRouteApi } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";
import { SearchField } from "./components/SearchField";
import { useI18n } from "./i18n";
import {
  APIRequestError,
  executeSearch,
  getSearchCatalog,
  type SearchRequest,
} from "./lib/api/client";
import { queryKeys } from "./lib/api/queryKeys";
import {
  buildFieldFilterGroups,
  fieldOwnerModule,
  searchModuleChips,
} from "./lib/search/fieldFilterGroups";
import { buildProfileCards } from "./lib/search/groupResults";
import { parseList, serializeList } from "./lib/search/urlState";
import type { GlobalSearchState, SearchModule } from "./lib/search/types";
import { ProfileSearchCard } from "./lib/search/ProfileSearchCard";
import { searchErrorMessage } from "./lib/search/errors";
import { searchResultKey } from "./lib/search/searchKeys";
import { SEARCH_RESULT_CAP, searchResultWindow } from "./lib/search/searchWindow";

const searchRoute = getRouteApi("/search");

export function SearchPage() {
  const search = searchRoute.useSearch();
  const navigate = searchRoute.useNavigate();
  const { messages } = useI18n();
  const searchMessages = messages.search;
  const [draft, setDraft] = useState(search.q);
  const [prevSearchQ, setPrevSearchQ] = useState(search.q);
  if (search.q !== prevSearchQ) {
    setPrevSearchQ(search.q);
    setDraft(search.q);
  }

  const catalog = useQuery({
    queryKey: queryKeys.search.catalog,
    queryFn: ({ signal }) => getSearchCatalog(signal),
    staleTime: 60_000,
  });
  const selectedModules = useMemo(
    () => parseList(search.modules) as SearchModule[],
    [search.modules],
  );
  const selectedFields = useMemo(() => parseList(search.fields), [search.fields]);
  const request = useMemo<SearchRequest>(
    () => ({
      q: search.q,
      ...(selectedModules.length > 0 ? { modules: selectedModules } : {}),
      ...(selectedFields.length > 0 ? { fields: selectedFields } : {}),
      limit: search.limit,
      offset: (search.page - 1) * search.limit,
      sort: search.sort,
      order: search.order,
    }),
    [
      search.limit,
      search.order,
      search.page,
      search.q,
      search.sort,
      selectedFields,
      selectedModules,
    ],
  );
  const results = useQuery({
    queryKey: queryKeys.search.global(request),
    queryFn: ({ signal }) => executeSearch(request, signal),
    enabled: search.q.trim().length > 0,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
    gcTime: 5 * 60_000,
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
  const fieldOptions = useMemo(
    () =>
      buildFieldFilterGroups(
        catalog.data?.fields,
        moduleLabels,
        selectedModules,
        (catalog.data?.modules.map((module) => module.key) ?? []) as SearchModule[],
      ).map((group) => ({
        label: group.label,
        options: group.options,
      })),
    [catalog.data?.fields, catalog.data?.modules, moduleLabels, selectedModules],
  );
  const moduleChips = useMemo(
    () => searchModuleChips(catalog.data?.modules),
    [catalog.data?.modules],
  );

  const updateSearch = (patch: Partial<GlobalSearchState>) => {
    void navigate({
      search: (current) => ({ ...current, ...patch }),
    });
  };
  const applyFilters = (patch: Partial<GlobalSearchState>) => {
    updateSearch({ q: draft, page: 1, ...patch });
  };
  const pruneFieldsForModules = (modules: SearchModule[]) => {
    const allowedFields = new Set(
      catalog.data?.fields
        .filter((field) => {
          const owner = fieldOwnerModule(field);
          return modules.length === 0 || modules.includes(owner);
        })
        .map((field) => field.key) ?? [],
    );
    return serializeList(selectedFields.filter((field) => allowedFields.has(field)));
  };
  const selectModule = (module: SearchModule | null) => {
    if (!module) {
      applyFilters({
        modules: "",
        fields: pruneFieldsForModules([]),
      });
      return;
    }
    const allModules = (catalog.data?.modules ?? []).map((m) => m.key as SearchModule);
    const current = selectedModules.length === 0 ? allModules : selectedModules;
    const next = current.includes(module)
      ? current.filter((value) => value !== module)
      : [...new Set([...current, module])];
    const canonical = next.length === allModules.length ? [] : next;
    applyFilters({
      modules: serializeList(canonical),
      fields: pruneFieldsForModules(canonical),
    });
  };
  const togglePreview = (profileId: string) => {
    updateSearch({ preview: search.preview === profileId ? "" : profileId });
  };
  const sortValue = `${search.sort}:${search.order}` as const;
  const resultTotal = results.data?.page.total ?? 0;
  const hasQuery = search.q.trim().length > 0;
  const resultWindow = searchResultWindow(resultTotal, search.limit);
  const showCount = hasQuery && !results.isError && results.data != null;
  const listPending = Boolean(results.isFetching && results.isPlaceholderData);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const cardKey = cards.map((card) => card.key).join("|");
  const [activeIndex, setActiveIndex] = useState(0);
  const [prevCardKey, setPrevCardKey] = useState(cardKey);
  const [keyboardFocus, setKeyboardFocus] = useState(false);
  if (cardKey !== prevCardKey) {
    setPrevCardKey(cardKey);
    setActiveIndex(0);
    setKeyboardFocus(false);
  }
  useEffect(() => {
    if (!keyboardFocus) return;
    listRef.current?.querySelector<HTMLElement>("[data-active='true'] .search-card__header")?.focus();
  }, [activeIndex, keyboardFocus]);
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target;
      const typing =
        target instanceof HTMLElement &&
        (target.isContentEditable ||
          target.tagName === "INPUT" ||
          target.tagName === "TEXTAREA" ||
          target.tagName === "SELECT");
      if (event.key === "Enter" && target instanceof Element && target.closest(".search-list")) return;
      const action = searchResultKey(event.key, activeIndex, hasQuery ? cards.length : 0, typing);
      if (action == null || event.altKey || event.ctrlKey || event.metaKey) return;
      if (action === "focus") {
        event.preventDefault();
        searchInputRef.current?.focus();
        return;
      }
      event.preventDefault();
      if (action === "toggle") {
        const card = cards[activeIndex];
        if (card) togglePreview(card.profileId);
        return;
      }
      setKeyboardFocus(true);
      setActiveIndex(action);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [activeIndex, cards, hasQuery]);

  return (
    <PageShell className="search-page" title={searchMessages.title}>
      <section className="search-toolbar">
        <SearchField
          inputRef={searchInputRef}
          label={searchMessages.title}
          mode="suggest"
          placeholder={searchMessages.inputPlaceholder}
          value={draft}
          onChange={setDraft}
          onSubmit={(value) => {
            if (value === search.q) return;
            updateSearch({ q: value, page: 1, preview: "" });
          }}
        />
        <div className="search-filters-bar">
          <div className="search-modules" role="group" aria-label={searchMessages.modulesLegend}>
            <Button
              aria-pressed={selectedModules.length === 0}
              className="search-module-chip"
              onClick={() => selectModule(null)}
              shape="round"
              size="small"
              type={selectedModules.length === 0 ? "primary" : "default"}
            >
              {searchMessages.modulesAll}
            </Button>
            {(moduleChips ?? []).map((module) => {
              const isSelected =
                selectedModules.length === 0 || selectedModules.includes(module.key);
              return (
                <Button
                  aria-pressed={isSelected}
                  className="search-module-chip"
                  key={module.key}
                  onClick={() => selectModule(module.key)}
                  shape="round"
                  size="small"
                  type={isSelected ? "primary" : "default"}
                >
                  {module.label}
                </Button>
              );
            })}
          </div>
          <div className="search-filter-controls">
            <label className="search-filter search-filter--fields">
              <span className="visually-hidden">{searchMessages.fieldsLabel}</span>
              <Select
                allowClear
                loading={catalog.isLoading}
                maxTagCount="responsive"
                mode="multiple"
                notFoundContent={searchMessages.fieldsEmptyState}
                optionFilterProp="label"
                optionRender={(option) => {
                  const data = option.data as { title?: string; scope?: string } | undefined;
                  const title = data?.title ?? String(option.label ?? "");
                  const scope = data?.scope;
                  return (
                    <span className="search-field-option">
                      <span className="search-field-option__title">{title}</span>
                      {scope ? <span className="search-field-option__scope">{scope}</span> : null}
                    </span>
                  );
                }}
                options={fieldOptions}
                placeholder={searchMessages.fieldsSearchPlaceholder}
                popupMatchSelectWidth={false}
                showSearch
                size="small"
                styles={{ popup: { root: { minWidth: "18rem" } } }}
                value={selectedFields}
                onChange={(values) => applyFilters({ fields: serializeList(values ?? []) })}
              />
            </label>
            <label className="search-filter search-filter--sort">
              <span className="visually-hidden">{searchMessages.sortLabel}</span>
              <Select
                size="small"
                value={sortValue}
                onChange={(value) => {
                  const [sort, order] = value.split(":") as [
                    GlobalSearchState["sort"],
                    GlobalSearchState["order"],
                  ];
                  applyFilters({ sort, order });
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
        </div>
      </section>

      {results.isError ? (
        <StatusBanner
          description={searchErrorMessage(results.error)}
          title={searchMessages.resultsErrorTitle}
          tone="error"
        />
      ) : null}
      {catalog.isError ? (
        <StatusBanner
          description={searchErrorMessage(catalog.error)}
          title={searchMessages.catalogErrorTitle}
          tone="error"
        />
      ) : null}

      <section aria-label={searchMessages.resultsCaption} className="search-results">
        {results.isFetching ? (
          <InlineStatus kind="loading" label={searchMessages.loadingResults} />
        ) : null}
        {!hasQuery ? (
          <StateCard
            compact
            description={searchMessages.emptyQueryDescription}
            kind="empty"
            title={searchMessages.emptyQueryTitle}
          />
        ) : null}
        {showCount && !results.isFetching && cards.length === 0 ? (
          <InlineStatus kind="empty" label={searchMessages.noResults} showIcon={false} />
        ) : null}
        {showCount && !listPending && cards.length > 0 ? (
          <div className="search-results__count">
            <span>{searchMessages.resultCount({ count: resultTotal })}</span>
            {resultWindow.truncated ? (
              <span className="search-results__truncated">
                {searchMessages.truncated({ count: SEARCH_RESULT_CAP })}
              </span>
            ) : null}
          </div>
        ) : null}
        <div className={listPending ? "search-list search-list--pending" : "search-list"} ref={listRef}>
          {(hasQuery ? cards : []).map((card, index) => (
            <ProfileSearchCard
              active={index === activeIndex}
              card={card}
              key={card.key}
              moduleLabels={moduleLabels}
              open={search.preview === card.profileId}
              query={search.q}
              showUpdatedAt={search.sort === "updated_at"}
              onToggle={togglePreview}
            />
          ))}
        </div>
      </section>
      {results.data && !listPending && results.data.page.total > 0 ? (
        <Pagination
          current={search.page}
          pageSize={search.limit}
          showLessItems
          showSizeChanger={false}
          size="small"
          total={searchResultWindow(results.data.page.total, search.limit).shown}
          onChange={(page) => updateSearch({ page, preview: "" })}
        />
      ) : null}
    </PageShell>
  );
}

