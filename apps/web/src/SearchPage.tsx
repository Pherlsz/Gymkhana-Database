import { Alert, Button, Inline, Page, Stack, StatusBadge, Surface } from "./ui";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { createColumnHelper } from "@tanstack/react-table";
import { useEffect, useMemo, useState } from "react";
import { DataGrid, DataGridPagination } from "./DataGrid";
import { normalizeProfileSearch } from "./ProfilesPage";
import { searchRoute } from "./App";
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

const moduleValues: SearchModule[] = [
  "profiles",
  "documents",
  "bills",
  "custom_data",
  "attachments",
];
const resultColumn = createColumnHelper<SearchResult>();

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
    () => new Map(catalog.data?.modules.map((module) => [module.key, module.label]) ?? []),
    [catalog.data?.modules],
  );
  const columns = useMemo(() => createSearchColumns(moduleLabels), [moduleLabels]);
  const totalPages = Math.max(
    1,
    Math.min(
      Math.floor(10_000 / search.limit) + 1,
      Math.ceil((results.data?.page.total ?? 0) / search.limit),
    ),
  );

  const updateSearch = (patch: Partial<GlobalSearchState>) => {
    void navigate({ search: (current) => ({ ...current, ...patch }) });
  };
  const submit = () => updateSearch({ q: draft, page: 1 });
  const toggleModule = (module: SearchModule, checked: boolean) => {
    const allModules = catalog.data?.modules.map((item) => item.key) ?? moduleValues;
    const current = selectedModules.length === 0 ? [...allModules] : [...selectedModules];
    const next = checked
      ? [...new Set([...current, module])]
      : current.filter((value) => value !== module);
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

  return (
    <Page.Root maxWidth="lg">
      <Page.Header>
        <Page.Eyebrow>M7 · Busca global</Page.Eyebrow>
        <Page.Title>Buscar dados autorizados</Page.Title>
        <Page.Description>
          Consulte campos lógicos de pessoas, documentos, contas, dados personalizados e metadados
          seguros de anexos. O conteúdo dos arquivos não faz parte desta busca.
        </Page.Description>
      </Page.Header>
      <Page.Content>
        <Stack gap="5">
          <Surface className="search-controls" tone="raised">
            <Stack gap="4">
              <label className="search-controls__terms">
                Termos — um por linha
                <textarea
                  aria-describedby="search-terms-help"
                  maxLength={650}
                  rows={3}
                  value={draft}
                  onChange={(event) => setDraft(event.target.value)}
                />
              </label>
              <span className="search-controls__help" id="search-terms-help">
                Até {catalog.data?.limits.maximum_terms ?? 5} sequências literais. Espaços, zeros à
                esquerda, letras, números e pontuação são preservados.
              </span>
              {catalog.isError ? (
                <Alert title="Não foi possível carregar o catálogo" tone="danger">
                  {searchErrorMessage(catalog.error)}
                </Alert>
              ) : null}
              {catalog.data ? (
                <fieldset className="search-modules">
                  <legend>Módulos</legend>
                  {catalog.data.modules.map((module) => (
                    <label key={module.key}>
                      <input
                        checked={
                          selectedModules.length === 0 || selectedModules.includes(module.key)
                        }
                        type="checkbox"
                        onChange={(event) => toggleModule(module.key, event.target.checked)}
                      />
                      {module.label}
                    </label>
                  ))}
                </fieldset>
              ) : null}
              <div className="search-options">
                <label>
                  Campos específicos
                  <select
                    multiple
                    size={8}
                    value={selectedFields}
                    onChange={(event) =>
                      updateSearch({
                        fields: serializeList(
                          [...event.currentTarget.selectedOptions].map((option) => option.value),
                        ),
                        page: 1,
                      })
                    }
                  >
                    {catalog.data?.fields
                      .filter(
                        (field) =>
                          selectedModules.length === 0 || selectedModules.includes(field.module),
                      )
                      .map((field) => (
                        <option key={field.key} value={field.key}>
                          {moduleLabels.get(field.module)} · {field.label}
                        </option>
                      ))}
                  </select>
                  <span className="search-controls__help">
                    Sem seleção, todos os campos permitidos.
                  </span>
                </label>
                <label>
                  Ordenação
                  <select
                    value={`${search.sort}:${search.order}`}
                    onChange={(event) => {
                      const [sort, order] = event.target.value.split(":") as [
                        GlobalSearchState["sort"],
                        GlobalSearchState["order"],
                      ];
                      updateSearch({ sort, order, page: 1 });
                    }}
                  >
                    <option value="relevance:desc">Maior relevância</option>
                    <option value="relevance:asc">Menor relevância</option>
                    <option value="updated_at:desc">Atualizados recentemente</option>
                    <option value="updated_at:asc">Atualizados há mais tempo</option>
                  </select>
                </label>
                <label>
                  Por página
                  <select
                    value={search.limit}
                    onChange={(event) =>
                      updateSearch({ limit: Number(event.target.value) as 25 | 50 | 100, page: 1 })
                    }
                  >
                    <option value={25}>25</option>
                    <option value={50}>50</option>
                    <option value={100}>100</option>
                  </select>
                </label>
              </div>
              <Inline>
                <Button disabled={draft.trim().length === 0 || !catalog.isSuccess} onClick={submit}>
                  Buscar
                </Button>
                <Button
                  onClick={() => {
                    setDraft("");
                    updateSearch({ q: "", modules: "", fields: "", page: 1 });
                  }}
                >
                  Limpar
                </Button>
              </Inline>
            </Stack>
          </Surface>

          {results.isError ? (
            <Alert title="Não foi possível executar a busca" tone="danger">
              {searchErrorMessage(results.error)}
            </Alert>
          ) : null}
          {terms.length === 0 ? (
            <Alert title="Informe o que deseja encontrar" tone="info">
              Use uma linha para cada parâmetro. Termos diferentes podem corresponder a campos e
              relações diferentes da mesma pessoa.
            </Alert>
          ) : null}
          <DataGrid
            caption="Resultados da busca global"
            className="search-results"
            columns={columns}
            data={results.data?.results ?? []}
            emptyLabel="Nenhum resultado autorizado corresponde aos termos informados."
            getRowId={(result) => `${result.module}:${result.entity_id}:${result.field_key}`}
            loading={results.isFetching}
            loadingLabel="Buscando dados autorizados..."
            renderCard={(result) => (
              <SearchResultCard
                key={`${result.module}:${result.entity_id}:${result.field_key}`}
                moduleLabel={moduleLabels.get(result.module) ?? result.module}
                result={result}
              />
            )}
          />
          {results.data ? (
            <DataGridPagination
              label="resultados"
              onPage={(page) => updateSearch({ page })}
              page={search.page}
              total={results.data.page.total}
              totalPages={totalPages}
            />
          ) : null}
        </Stack>
      </Page.Content>
    </Page.Root>
  );
}

function createSearchColumns(moduleLabels: Map<string, string>) {
  return [
    resultColumn.accessor("score", { header: "Relevância" }),
    resultColumn.accessor("entity_label", { header: "Entidade" }),
    resultColumn.accessor("field_label", {
      header: "Correspondência",
      cell: ({ row }) => (
        <Stack gap="1">
          <strong>{row.original.field_label}</strong>
          <span className="search-result__preview">{row.original.preview}</span>
        </Stack>
      ),
    }),
    resultColumn.accessor("module", {
      header: "Módulo",
      cell: ({ getValue }) => (
        <StatusBadge tone="info">{moduleLabels.get(getValue()) ?? getValue()}</StatusBadge>
      ),
    }),
    resultColumn.accessor("updated_at", {
      header: "Atualizado",
      cell: ({ getValue }) => formatDateTime(getValue()),
    }),
    resultColumn.display({
      id: "actions",
      header: "",
      cell: ({ row }) => <SearchTargetLink result={row.original} />,
    }),
  ];
}

function SearchResultCard({ result, moduleLabel }: { result: SearchResult; moduleLabel: string }) {
  return (
    <Surface className="search-result-card" tone="raised">
      <Stack gap="2">
        <Inline align="center">
          <StatusBadge tone="info">{moduleLabel}</StatusBadge>
          <span>Relevância {result.score}</span>
        </Inline>
        <strong>{result.entity_label}</strong>
        <span>{result.field_label}</span>
        <span className="search-result__preview">{result.preview}</span>
        <SearchTargetLink result={result} />
      </Stack>
    </Surface>
  );
}

function SearchTargetLink({ result }: { result: SearchResult }) {
  const profileSearch = profileSearchForResult(result);
  if (profileSearch) {
    return (
      <Link className="search-result__link" search={profileSearch} to="/profiles">
        Abrir registro
      </Link>
    );
  }
  return (
    <Link className="search-result__link" to="/custom-data">
      Abrir dados personalizados
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

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(
    new Date(value),
  );
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
