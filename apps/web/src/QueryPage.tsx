import { Alert, Button, Inline, Page, Stack, StatusBadge, Surface } from "@pherlsz/gymkhana-ui";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { createColumnHelper, type ColumnDef } from "@tanstack/react-table";
import { useEffect, useMemo, useRef, useState } from "react";
import { queryRoute } from "./App";
import { DataGrid, DataGridPagination } from "./DataGrid";
import { normalizeProfileSearch } from "./ProfilesPage";
import { APIRequestError } from "./lib/api/client";
import {
  executeQueryPlan,
  getQueryCatalog,
  getQueryResult,
  validateQueryPlan,
  type PlanEstimate,
  type QueryCatalog,
  type QueryField,
  type QueryFilterNode,
  type QueryOperator,
  type QueryPlan,
  type QueryRelation,
  type QueryResultCell,
  type QueryResultPage,
  type QueryResultRow,
  type QuerySort,
} from "./lib/api/query";
import { decodeQueryURLPlan, encodeQueryURLPlan, recoverQueryURLPlan } from "./lib/queryState";

type DraftPredicate = {
  id: string;
  kind: "predicate";
  field: string;
  operator: QueryOperator;
  values: string[];
};
type DraftGroup = {
  id: string;
  kind: "group";
  conjunction: "AND" | "OR";
  children: DraftNode[];
};
type DraftRelation = {
  id: string;
  kind: "relation";
  relation: string;
  child: DraftGroup;
};
type DraftNot = { id: string; kind: "not"; child: DraftGroup };
type DraftNode = DraftPredicate | DraftGroup | DraftRelation | DraftNot;

let draftSequence = 0;
function draftID(): string {
  draftSequence += 1;
  return `query-node-${draftSequence}`;
}

function emptyGroup(): DraftGroup {
  return { id: draftID(), kind: "group", conjunction: "AND", children: [] };
}

export function QueryPage() {
  const search = queryRoute.useSearch();
  const navigate = queryRoute.useNavigate();
  const queryClient = useQueryClient();
  const catalog = useQuery({
    queryKey: ["query-catalog"],
    queryFn: ({ signal }) => getQueryCatalog(signal),
    staleTime: 30_000,
  });
  const [rootEntity, setRootEntity] = useState("");
  const [projections, setProjections] = useState<string[]>([]);
  const [filter, setFilter] = useState<DraftGroup>(emptyGroup);
  const [sort, setSort] = useState<QuerySort[]>([]);
  const [maximumRows, setMaximumRows] = useState(100);
  const [hydratedCatalogVersion, setHydratedCatalogVersion] = useState("");
  const [recoveryIssues, setRecoveryIssues] = useState<string[]>([]);
  const [invalidURLPlan] = useState(hasInvalidQueryURLPlan);
  const resultPage = search.result_page;
  const setResultPage = (page: number) => {
    void navigate({
      replace: true,
      search: (current) => ({ ...current, result_page: page }),
    });
  };

  const entities = catalog.data?.entities ?? [];
  const rootFields = useMemo(
    () => catalog.data?.fields.filter((field) => field.entity === rootEntity) ?? [],
    [catalog.data?.fields, rootEntity],
  );
  const projectableFields = useMemo(
    () => rootFields.filter((field) => field.projectable),
    [rootFields],
  );
  const sortableFields = useMemo(() => rootFields.filter((field) => field.sortable), [rootFields]);

  const plan = useMemo<QueryPlan | null>(() => {
    if (!catalog.data || !rootEntity || projections.length === 0) return null;
    return {
      version: "v1",
      catalog_version: catalog.data.version,
      root_entity: rootEntity,
      projections,
      ...(filter.children.length > 0
        ? { filter: draftFilterToPlan(filter, catalog.data, rootEntity) }
        : {}),
      ...(sort.length > 0 ? { sort } : {}),
      maximum_rows: maximumRows,
    };
  }, [catalog.data, filter, maximumRows, projections, rootEntity, sort]);

  const refreshStaleCatalog = (error: Error) => {
    if (error instanceof APIRequestError && error.code === "query_catalog_stale") {
      void queryClient.invalidateQueries({ queryKey: ["query-catalog"] });
    }
  };
  const validation = useMutation({
    mutationFn: (value: QueryPlan) => validateQueryPlan(value),
    onError: refreshStaleCatalog,
  });
  const idempotency = useRef<{ plan: string; key: string } | null>(null);
  const execution = useMutation({
    mutationFn: (value: QueryPlan) => {
      const serialized = JSON.stringify(value);
      if (idempotency.current?.plan !== serialized) {
        idempotency.current = { plan: serialized, key: newQueryIdempotencyKey() };
      }
      return executeQueryPlan(value, idempotency.current.key);
    },
    onSuccess: () => setResultPage(1),
    onError: refreshStaleCatalog,
  });

  useEffect(() => {
    if (!catalog.data || hydratedCatalogVersion === catalog.data.version) return;
    const source = hydratedCatalogVersion
      ? {
          version: "v1" as const,
          catalog_version: hydratedCatalogVersion,
          root_entity: rootEntity,
          projections,
          ...(filter.children.length > 0
            ? { filter: draftFilterToPlan(filter, catalog.data, rootEntity) }
            : {}),
          ...(sort.length > 0 ? { sort } : {}),
          maximum_rows: maximumRows,
        }
      : decodeQueryURLPlan(search.plan);
    const recovered = recoverQueryURLPlan(source, catalog.data);
    if (!recovered) return;
    setRootEntity(recovered.plan.root_entity);
    setProjections(recovered.plan.projections);
    setFilter(recovered.plan.filter ? planFilterToDraftGroup(recovered.plan.filter) : emptyGroup());
    setSort(recovered.plan.sort ?? []);
    setMaximumRows(recovered.plan.maximum_rows);
    setRecoveryIssues(
      recovered.catalogChanged ? [...recovered.issues, "versão do catálogo"] : recovered.issues,
    );
    setHydratedCatalogVersion(catalog.data.version);
    validation.reset();
    execution.reset();
    idempotency.current = null;
    setResultPage(1);
  }, [
    catalog.data,
    execution,
    filter,
    hydratedCatalogVersion,
    maximumRows,
    projections,
    rootEntity,
    search.plan,
    sort,
    validation,
  ]);

  const urlPlan = useMemo<QueryPlan | null>(() => {
    if (!catalog.data || !rootEntity) return null;
    return {
      version: "v1",
      catalog_version: catalog.data.version,
      root_entity: rootEntity,
      projections,
      ...(filter.children.length > 0
        ? { filter: draftFilterToPlan(filter, catalog.data, rootEntity) }
        : {}),
      ...(sort.length > 0 ? { sort } : {}),
      maximum_rows: maximumRows,
    };
  }, [catalog.data, filter, maximumRows, projections, rootEntity, sort]);
  const encodedURLPlan = urlPlan ? encodeQueryURLPlan(urlPlan) : undefined;
  useEffect(() => {
    if (
      !catalog.data ||
      hydratedCatalogVersion !== catalog.data.version ||
      !encodedURLPlan ||
      search.plan === encodedURLPlan
    ) {
      return;
    }
    void navigate({
      replace: true,
      search: (current) => ({
        ...current,
        plan: encodedURLPlan,
        result_page: current.result_page,
      }),
    });
  }, [catalog.data, encodedURLPlan, hydratedCatalogVersion, navigate, search.plan]);

  const resultLimit = 100;
  const result = useQuery({
    queryKey: ["query-result", execution.data?.id, resultPage],
    queryFn: ({ signal }) =>
      getQueryResult(execution.data!.id, resultLimit, (resultPage - 1) * resultLimit, signal),
    enabled: execution.data?.state === "COMPLETED" || execution.data?.state === "RUNNING",
    refetchInterval: (query) =>
      execution.data?.state === "RUNNING" && !query.state.data && query.state.fetchFailureCount < 30
        ? 1_000
        : false,
    retry: (attempt, error) =>
      !(error instanceof APIRequestError && [403, 404, 409, 410].includes(error.status)) &&
      attempt < 1,
  });
  const waitingForResult =
    execution.data?.state === "RUNNING" && !result.data && result.failureCount < 30;

  const changeRoot = (value: string) => {
    setRootEntity(value);
    const nextFields = catalog.data?.fields.filter(
      (field) => field.entity === value && field.projectable,
    );
    setProjections(nextFields?.slice(0, 3).map((field) => field.key) ?? []);
    setFilter(emptyGroup());
    setSort([]);
    setRecoveryIssues([]);
    setResultPage(1);
    execution.reset();
    validation.reset();
    idempotency.current = null;
  };
  const changePlan = () => {
    setResultPage(1);
    execution.reset();
    validation.reset();
    idempotency.current = null;
  };

  return (
    <Page.Root maxWidth="lg">
      <Page.Header>
        <Page.Eyebrow>M10 · Query Engine</Page.Eyebrow>
        <Page.Title>Construtor visual de consultas</Page.Title>
        <Page.Description>
          Combine campos e relações do catálogo autorizado. O servidor valida o plano lógico,
          parametriza os valores e executa somente leitura com limites estritos.
        </Page.Description>
      </Page.Header>
      <Page.Content>
        <Stack gap="5">
          {catalog.isError ? (
            <Alert title="Não foi possível carregar o catálogo" tone="danger">
              {queryErrorMessage(catalog.error)}
            </Alert>
          ) : null}
          {catalog.isPending ? <Alert title="Carregando catálogo" tone="info" /> : null}
          {invalidURLPlan ? (
            <Alert title="Plano da URL ignorado" tone="warning">
              O estado recebido excede os limites ou contém uma estrutura desconhecida. Um plano
              seguro foi iniciado.
            </Alert>
          ) : null}
          {recoveryIssues.length > 0 ? (
            <Alert title="Plano recuperado com o catálogo atual" tone="warning">
              Os itens ainda permitidos foram preservados. Revise: {recoveryIssues.join(", ")}.
            </Alert>
          ) : null}
          {urlPlan && !encodedURLPlan ? (
            <Alert title="O plano excede o limite da URL" tone="warning">
              Reduza a quantidade ou o tamanho dos valores antes de validar e executar.
            </Alert>
          ) : null}
          {catalog.data ? (
            <>
              <Surface className="query-builder" tone="raised">
                <Stack gap="5">
                  <div className="query-builder__heading">
                    <div>
                      <h2>1. Origem e colunas</h2>
                      <p>
                        Selecione uma entidade raiz e até {catalog.data.limits.maximum_projections}{" "}
                        colunas.
                      </p>
                    </div>
                    <StatusBadge tone="info">
                      Catálogo {catalog.data.version.slice(0, 8)}
                    </StatusBadge>
                  </div>
                  <label className="query-control">
                    Entidade raiz
                    <select
                      aria-label="Entidade raiz"
                      value={rootEntity}
                      onChange={(event) => changeRoot(event.target.value)}
                    >
                      {entities.map((entity) => (
                        <option key={entity.key} value={entity.key}>
                          {entity.label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <fieldset className="query-projections">
                    <legend>Colunas do resultado</legend>
                    <div className="query-projections__grid">
                      {projectableFields.map((field) => (
                        <label key={field.key}>
                          <input
                            checked={projections.includes(field.key)}
                            disabled={
                              !projections.includes(field.key) &&
                              projections.length >= catalog.data.limits.maximum_projections
                            }
                            type="checkbox"
                            onChange={(event) => {
                              setProjections((current) =>
                                event.target.checked
                                  ? [...current, field.key]
                                  : current.filter((key) => key !== field.key),
                              );
                              changePlan();
                            }}
                          />
                          <span>{field.label}</span>
                          <small>{valueKindLabel(field.kind)}</small>
                        </label>
                      ))}
                    </div>
                  </fieldset>
                </Stack>
              </Surface>

              <Surface className="query-builder" tone="raised">
                <Stack gap="4">
                  <div className="query-builder__heading">
                    <div>
                      <h2>2. Filtros</h2>
                      <p>
                        Grupos, negações e relações podem ser combinados dentro dos limites do
                        catálogo.
                      </p>
                    </div>
                    <StatusBadge tone="neutral">AND / OR / NOT</StatusBadge>
                  </div>
                  <FilterGroupEditor
                    catalog={catalog.data}
                    depth={1}
                    entity={rootEntity}
                    group={filter}
                    root
                    onChange={(next) => {
                      setFilter(next);
                      changePlan();
                    }}
                  />
                </Stack>
              </Surface>

              <Surface className="query-builder" tone="raised">
                <Stack gap="4">
                  <div className="query-builder__heading">
                    <div>
                      <h2>3. Ordenação e limite</h2>
                      <p>
                        Sem ordenação explícita, o servidor aplica a ordem determinística do
                        catálogo.
                      </p>
                    </div>
                  </div>
                  <SortEditor
                    fields={sortableFields}
                    maximum={catalog.data.limits.maximum_sort_fields}
                    value={sort}
                    onChange={(next) => {
                      setSort(next);
                      changePlan();
                    }}
                  />
                  <label className="query-control query-control--small">
                    Máximo de linhas
                    <input
                      aria-label="Máximo de linhas"
                      max={catalog.data.limits.maximum_rows}
                      min={1}
                      type="number"
                      value={maximumRows}
                      onChange={(event) => {
                        setMaximumRows(
                          Math.min(
                            catalog.data.limits.maximum_rows,
                            Math.max(1, Number(event.target.value) || 1),
                          ),
                        );
                        changePlan();
                      }}
                    />
                  </label>
                  <Inline align="center">
                    <Button
                      disabled={!plan || !encodedURLPlan || validation.isPending}
                      onClick={() => plan && validation.mutate(plan)}
                    >
                      {validation.isPending ? "Validando" : "Validar plano"}
                    </Button>
                    <Button
                      disabled={!plan || !encodedURLPlan || execution.isPending}
                      onClick={() => plan && execution.mutate(plan)}
                    >
                      {execution.isPending ? "Executando" : "Executar consulta"}
                    </Button>
                  </Inline>
                  {projections.length === 0 ? (
                    <Alert title="Selecione ao menos uma coluna" tone="warning" />
                  ) : null}
                  {validation.data ? <PlanEstimatePanel estimate={validation.data} /> : null}
                  {validation.isError ? (
                    <Alert title="O plano precisa ser revisado" tone="danger">
                      {queryErrorMessage(validation.error)}
                    </Alert>
                  ) : null}
                  {execution.isError ? (
                    <Alert title="Não foi possível executar a consulta" tone="danger">
                      {queryErrorMessage(execution.error)}
                    </Alert>
                  ) : null}
                </Stack>
              </Surface>
            </>
          ) : null}

          {execution.data ? (
            <QueryResults
              page={result.data}
              pending={result.isPending || result.isFetching || waitingForResult}
              error={waitingForResult ? null : result.error}
              execution={execution.data}
              currentPage={resultPage}
              pageSize={resultLimit}
              onPage={setResultPage}
            />
          ) : null}
        </Stack>
      </Page.Content>
    </Page.Root>
  );
}

function FilterGroupEditor(props: {
  catalog: QueryCatalog;
  group: DraftGroup;
  entity: string;
  depth: number;
  root?: boolean;
  onChange: (value: DraftGroup) => void;
  onRemove?: () => void;
}) {
  const filterable = props.catalog.fields.filter(
    (field) => field.entity === props.entity && field.filterable,
  );
  const relations = props.catalog.relations.filter(
    (relation) => relation.from_entity === props.entity,
  );
  const canAdd =
    props.depth < props.catalog.limits.maximum_filter_depth &&
    props.group.children.length < props.catalog.limits.maximum_filter_nodes;
  const replaceChild = (id: string, value: DraftNode) =>
    props.onChange({
      ...props.group,
      children: props.group.children.map((child) => (child.id === id ? value : child)),
    });
  const removeChild = (id: string) =>
    props.onChange({
      ...props.group,
      children: props.group.children.filter((child) => child.id !== id),
    });
  const append = (node: DraftNode) =>
    props.onChange({ ...props.group, children: [...props.group.children, node] });

  return (
    <div className={`query-filter-group${props.root ? " query-filter-group--root" : ""}`}>
      <div className="query-filter-group__header">
        <label>
          Combinar com
          <select
            aria-label={props.root ? "Combinação principal" : "Combinação do grupo"}
            value={props.group.conjunction}
            onChange={(event) =>
              props.onChange({ ...props.group, conjunction: event.target.value as "AND" | "OR" })
            }
          >
            <option value="AND">Todas (AND)</option>
            <option value="OR">Qualquer (OR)</option>
          </select>
        </label>
        {!props.root && props.onRemove ? (
          <Button onClick={props.onRemove}>Remover grupo</Button>
        ) : null}
      </div>
      {props.group.children.length === 0 ? (
        <p className="query-filter-empty">
          Sem filtros — todas as linhas autorizadas podem participar.
        </p>
      ) : null}
      <div className="query-filter-group__children">
        {props.group.children.map((child) => (
          <FilterNodeEditor
            catalog={props.catalog}
            depth={props.depth + 1}
            entity={props.entity}
            key={child.id}
            node={child}
            onChange={(next) => replaceChild(child.id, next)}
            onRemove={() => removeChild(child.id)}
          />
        ))}
      </div>
      <Inline align="center">
        <Button
          disabled={!canAdd || filterable.length === 0}
          onClick={() => append(newPredicate(filterable))}
        >
          Adicionar condição
        </Button>
        <Button disabled={!canAdd} onClick={() => append(emptyGroup())}>
          Adicionar grupo
        </Button>
        <Button
          disabled={!canAdd || relations.length === 0}
          onClick={() => append(newRelation(relations))}
        >
          Adicionar relação
        </Button>
        <Button
          disabled={!canAdd}
          onClick={() => append({ id: draftID(), kind: "not", child: emptyGroup() })}
        >
          Adicionar negação
        </Button>
      </Inline>
    </div>
  );
}

function FilterNodeEditor(props: {
  catalog: QueryCatalog;
  node: DraftNode;
  entity: string;
  depth: number;
  onChange: (value: DraftNode) => void;
  onRemove: () => void;
}) {
  if (props.node.kind === "group") {
    return (
      <FilterGroupEditor
        catalog={props.catalog}
        depth={props.depth}
        entity={props.entity}
        group={props.node}
        onChange={props.onChange}
        onRemove={props.onRemove}
      />
    );
  }
  if (props.node.kind === "not") {
    const node = props.node;
    return (
      <div className="query-filter-special">
        <div className="query-filter-special__label">
          <StatusBadge tone="warning">NOT</StatusBadge>
          <Button onClick={props.onRemove}>Remover negação</Button>
        </div>
        <FilterGroupEditor
          catalog={props.catalog}
          depth={props.depth}
          entity={props.entity}
          group={node.child}
          onChange={(child) => props.onChange({ ...node, child })}
        />
      </div>
    );
  }
  if (props.node.kind === "relation") {
    const node = props.node;
    const relations = props.catalog.relations.filter(
      (relation) => relation.from_entity === props.entity,
    );
    const selected = relations.find((relation) => relation.key === node.relation);
    return (
      <div className="query-filter-special">
        <div className="query-filter-special__label">
          <label>
            Relação
            <select
              aria-label="Relação do filtro"
              value={node.relation}
              onChange={(event) =>
                props.onChange({ ...node, relation: event.target.value, child: emptyGroup() })
              }
            >
              {relations.map((relation) => (
                <option key={relation.key} value={relation.key}>
                  {relation.label} · {relation.cardinality === "ONE" ? "um" : "muitos"}
                </option>
              ))}
            </select>
          </label>
          <Button onClick={props.onRemove}>Remover relação</Button>
        </div>
        {selected ? (
          <FilterGroupEditor
            catalog={props.catalog}
            depth={props.depth}
            entity={selected.to_entity}
            group={node.child}
            onChange={(child) => props.onChange({ ...node, child })}
          />
        ) : null}
      </div>
    );
  }
  return <PredicateEditor {...props} node={props.node} />;
}

function PredicateEditor(props: {
  catalog: QueryCatalog;
  node: DraftPredicate;
  entity: string;
  onChange: (value: DraftNode) => void;
  onRemove: () => void;
}) {
  const fields = props.catalog.fields.filter(
    (field) => field.entity === props.entity && field.filterable,
  );
  const field = fields.find((value) => value.key === props.node.field) ?? fields[0];
  const operator = field?.operators.includes(props.node.operator)
    ? props.node.operator
    : field?.operators[0];
  const definition = props.catalog.operators.find((value) => value.key === operator);
  const setField = (key: string) => {
    const next = fields.find((value) => value.key === key)!;
    props.onChange({
      ...props.node,
      field: key,
      operator: next.operators[0]!,
      values: initialValues(next.operators[0]!, props.catalog),
    });
  };
  const setOperator = (value: QueryOperator) =>
    props.onChange({ ...props.node, operator: value, values: initialValues(value, props.catalog) });
  if (!field || !operator || !definition) return null;
  return (
    <div className="query-predicate">
      <label>
        Campo
        <select
          aria-label="Campo da condição"
          value={field.key}
          onChange={(event) => setField(event.target.value)}
        >
          {fields.map((value) => (
            <option key={value.key} value={value.key}>
              {value.label} · {valueKindLabel(value.kind)}
            </option>
          ))}
        </select>
      </label>
      <label>
        Operador
        <select
          aria-label={`Operador de ${field.label}`}
          value={operator}
          onChange={(event) => setOperator(event.target.value as QueryOperator)}
        >
          {field.operators.map((value) => (
            <option key={value} value={value}>
              {props.catalog.operators.find((item) => item.key === value)?.label ?? value}
            </option>
          ))}
        </select>
      </label>
      <PredicateValues
        definition={definition}
        field={field}
        values={props.node.values}
        onChange={(values) => props.onChange({ ...props.node, operator, values })}
      />
      <Button onClick={props.onRemove}>Remover condição</Button>
    </div>
  );
}

function PredicateValues(props: {
  definition: QueryCatalog["operators"][number];
  field: QueryField;
  values: string[];
  onChange: (values: string[]) => void;
}) {
  if (props.definition.maximum_values === 0)
    return <span className="query-predicate__no-value">Sem valor</span>;
  if (props.field.options && props.field.options.length > 0) {
    const multiple = props.definition.maximum_values > 1;
    return (
      <label>
        Valor
        <select
          aria-label={`Valor de ${props.field.label}`}
          multiple={multiple}
          size={multiple ? Math.min(5, props.field.options.length) : undefined}
          value={multiple ? props.values : (props.values[0] ?? "")}
          onChange={(event) =>
            props.onChange(
              multiple
                ? [...event.currentTarget.selectedOptions].map((option) => option.value)
                : [event.currentTarget.value],
            )
          }
        >
          {props.field.options.map((option) => (
            <option key={option.key} value={option.key}>
              {option.label}
            </option>
          ))}
        </select>
      </label>
    );
  }
  if (props.definition.maximum_values > 2) {
    return (
      <label>
        Valores separados por vírgula
        <textarea
          aria-label={`Valores de ${props.field.label}`}
          rows={2}
          value={props.values.join(", ")}
          onChange={(event) =>
            props.onChange(
              event.target.value
                .split(",")
                .map((value) => value.trim())
                .filter(Boolean),
            )
          }
        />
      </label>
    );
  }
  return (
    <div className="query-predicate__values">
      {Array.from({ length: props.definition.maximum_values }, (_, index) => (
        <label key={index}>
          {props.definition.maximum_values === 2 ? (index === 0 ? "De" : "Até") : "Valor"}
          {props.field.kind === "boolean" ? (
            <select
              aria-label={`${props.field.label} valor ${index + 1}`}
              value={props.values[index] ?? "true"}
              onChange={(event) =>
                replaceValue(props.values, index, event.target.value, props.onChange)
              }
            >
              <option value="true">Sim</option>
              <option value="false">Não</option>
            </select>
          ) : (
            <input
              aria-label={`${props.field.label} valor ${index + 1}`}
              step={
                props.field.kind === "integer"
                  ? 1
                  : props.field.kind === "decimal"
                    ? "any"
                    : undefined
              }
              type={inputType(props.field)}
              value={
                props.field.kind === "timestamp"
                  ? timestampInputValue(props.values[index])
                  : (props.values[index] ?? "")
              }
              onChange={(event) =>
                replaceValue(
                  props.values,
                  index,
                  props.field.kind === "timestamp"
                    ? timestampPlanValue(event.target.value)
                    : event.target.value,
                  props.onChange,
                )
              }
            />
          )}
        </label>
      ))}
    </div>
  );
}

function SortEditor(props: {
  fields: QueryField[];
  maximum: number;
  value: QuerySort[];
  onChange: (value: QuerySort[]) => void;
}) {
  const available = props.fields.filter(
    (field) => !props.value.some((sort) => sort.field === field.key),
  );
  return (
    <div className="query-sort">
      {props.value.length === 0 ? <p>Ordem padrão do catálogo.</p> : null}
      {props.value.map((sort, index) => (
        <div className="query-sort__row" key={sort.field}>
          <label>
            Campo {index + 1}
            <select
              aria-label={`Campo de ordenação ${index + 1}`}
              value={sort.field}
              onChange={(event) =>
                props.onChange(
                  props.value.map((item, position) =>
                    position === index ? { ...item, field: event.target.value } : item,
                  ),
                )
              }
            >
              {props.fields.map((field) => (
                <option
                  disabled={props.value.some(
                    (item, position) => position !== index && item.field === field.key,
                  )}
                  key={field.key}
                  value={field.key}
                >
                  {field.label}
                </option>
              ))}
            </select>
          </label>
          <label>
            Direção
            <select
              aria-label={`Direção da ordenação ${index + 1}`}
              value={sort.direction}
              onChange={(event) =>
                props.onChange(
                  props.value.map((item, position) =>
                    position === index
                      ? { ...item, direction: event.target.value as "asc" | "desc" }
                      : item,
                  ),
                )
              }
            >
              <option value="asc">Crescente</option>
              <option value="desc">Decrescente</option>
            </select>
          </label>
          <Button
            onClick={() => props.onChange(props.value.filter((_, position) => position !== index))}
          >
            Remover
          </Button>
        </div>
      ))}
      <Button
        disabled={props.value.length >= props.maximum || available.length === 0}
        onClick={() =>
          props.onChange([...props.value, { field: available[0]!.key, direction: "asc" }])
        }
      >
        Adicionar ordenação
      </Button>
    </div>
  );
}

function PlanEstimatePanel({ estimate }: { estimate: PlanEstimate }) {
  return (
    <Alert title="Plano válido" tone="success">
      Custo estimado: {estimate.cost}. {estimate.columns.length} coluna(s). Impressão digital{" "}
      {estimate.fingerprint.slice(0, 12)}.
    </Alert>
  );
}

function QueryResults(props: {
  execution: {
    id: string;
    state: string;
    row_count: number;
    expires_at: string;
    root_entity: string;
  };
  page: QueryResultPage | undefined;
  pending: boolean;
  error: Error | null;
  currentPage: number;
  pageSize: number;
  onPage: (page: number) => void;
}) {
  const currentExecution = props.page?.execution ?? props.execution;
  const totalPages = Math.max(1, Math.ceil(currentExecution.row_count / props.pageSize));
  return (
    <Surface className="query-results" tone="raised">
      <Stack gap="4">
        <div className="query-builder__heading">
          <div>
            <h2>Resultado tipado</h2>
            <p>
              {currentExecution.row_count} linha(s). Expira em{" "}
              {formatTimestamp(currentExecution.expires_at)}.
            </p>
          </div>
          <StatusBadge tone={currentExecution.state === "COMPLETED" ? "success" : "warning"}>
            {executionStateLabel(currentExecution.state)}
          </StatusBadge>
        </div>
        {props.pending ? <Alert title="Carregando resultado" tone="info" /> : null}
        {props.error ? (
          <Alert title="Não foi possível carregar o resultado" tone="danger">
            {queryErrorMessage(props.error)}
          </Alert>
        ) : null}
        {props.page ? <QueryResultGrid page={props.page} /> : null}
        {totalPages > 1 ? (
          <DataGridPagination
            label="resultados"
            onPage={props.onPage}
            page={props.currentPage}
            total={currentExecution.row_count}
            totalPages={totalPages}
          />
        ) : null}
      </Stack>
    </Surface>
  );
}

const queryResultColumnHelper = createColumnHelper<QueryResultRow>();

function QueryResultGrid({ page }: { page: QueryResultPage }) {
  const columns = useMemo<ColumnDef<QueryResultRow, unknown>[]>(
    () => [
      queryResultColumnHelper.display({
        id: "entity",
        header: "Registro",
        cell: ({ row }) => (
          <QueryEntityReference rootEntity={page.execution.root_entity} row={row.original} />
        ),
      }),
      ...page.columns.map((column) =>
        queryResultColumnHelper.display({
          id: column.field_key,
          header: column.label,
          cell: ({ row }) =>
            formatResultCell(
              row.original.cells.find((cell) => cell.column_position === column.position),
            ),
        }),
      ),
    ],
    [page.columns, page.execution.root_entity],
  );
  return (
    <DataGrid
      caption="Resultado da consulta"
      className="query-results__grid"
      columns={columns}
      data={page.rows}
      emptyLabel="Nenhuma linha corresponde ao plano."
      getRowId={(row) => `${row.entity_kind}:${row.entity_id}`}
      loading={false}
      loadingLabel="Carregando resultado"
      renderCard={(row) => (
        <article aria-label={`Resultado ${row.entity_label}`} className="query-result-card">
          <h3>
            <QueryEntityReference rootEntity={page.execution.root_entity} row={row} />
          </h3>
          <dl>
            {page.columns.map((column) => (
              <div key={column.field_key}>
                <dt>{column.label}</dt>
                <dd>
                  {formatResultCell(
                    row.cells.find((cell) => cell.column_position === column.position),
                  )}
                </dd>
              </div>
            ))}
          </dl>
        </article>
      )}
    />
  );
}

function QueryEntityReference({ rootEntity, row }: { rootEntity: string; row: QueryResultRow }) {
  return rootEntity === "profiles" ? (
    <Link search={normalizeProfileSearch({ selected: row.entity_id, mode: "view" })} to="/profiles">
      {row.entity_label}
    </Link>
  ) : (
    row.entity_label
  );
}

function newPredicate(fields: QueryField[]): DraftPredicate {
  const field = fields[0]!;
  const operator = field.operators[0]!;
  return {
    id: draftID(),
    kind: "predicate",
    field: field.key,
    operator,
    values: initialValues(operator),
  };
}

function newRelation(relations: QueryRelation[]): DraftRelation {
  return { id: draftID(), kind: "relation", relation: relations[0]!.key, child: emptyGroup() };
}

function initialValues(operator: QueryOperator, catalog?: QueryCatalog): string[] {
  const maximum = catalog?.operators.find((value) => value.key === operator)?.maximum_values;
  if (maximum === 0 || operator === "is_null" || operator === "not_null") return [];
  if (maximum === 2 || operator === "between") return ["", ""];
  return [""];
}

function draftFilterToPlan(
  node: DraftNode,
  catalog: QueryCatalog,
  entity: string,
): QueryFilterNode {
  switch (node.kind) {
    case "predicate":
      return { kind: "predicate", field: node.field, operator: node.operator, values: node.values };
    case "group":
      return {
        kind: "group",
        conjunction: node.conjunction,
        children: node.children.map((child) => draftFilterToPlan(child, catalog, entity)),
      };
    case "not":
      return { kind: "not", children: [draftFilterToPlan(node.child, catalog, entity)] };
    case "relation": {
      const relation = catalog.relations.find((value) => value.key === node.relation);
      return {
        kind: "relation",
        relation: node.relation,
        children: [draftFilterToPlan(node.child, catalog, relation?.to_entity ?? entity)],
      };
    }
  }
}

function planFilterToDraftGroup(node: QueryFilterNode): DraftGroup {
  const converted = planFilterToDraft(node);
  return converted.kind === "group"
    ? converted
    : { id: draftID(), kind: "group", conjunction: "AND", children: [converted] };
}

function planFilterToDraft(node: QueryFilterNode): DraftNode {
  switch (node.kind) {
    case "predicate":
      return {
        id: draftID(),
        kind: "predicate",
        field: node.field ?? "",
        operator: node.operator ?? "eq",
        values: node.values ?? [],
      };
    case "group":
      return {
        id: draftID(),
        kind: "group",
        conjunction: node.conjunction ?? "AND",
        children: (node.children ?? []).map(planFilterToDraft),
      };
    case "not":
      return {
        id: draftID(),
        kind: "not",
        child: planFilterToDraftGroup(node.children?.[0] ?? emptyPlanGroup()),
      };
    case "relation":
      return {
        id: draftID(),
        kind: "relation",
        relation: node.relation ?? "",
        child: planFilterToDraftGroup(node.children?.[0] ?? emptyPlanGroup()),
      };
  }
}

function emptyPlanGroup(): QueryFilterNode {
  return { kind: "group", conjunction: "AND", children: [] };
}

function replaceValue(
  values: string[],
  index: number,
  value: string,
  onChange: (values: string[]) => void,
) {
  const next = [...values];
  next[index] = value;
  onChange(next);
}

function inputType(field: QueryField): "date" | "datetime-local" | "month" | "number" | "text" {
  if (field.kind === "civil_date") return "date";
  if (field.kind === "civil_month") return "month";
  if (field.kind === "timestamp") return "datetime-local";
  if (field.kind === "integer" || field.kind === "decimal") return "number";
  return "text";
}

function timestampInputValue(value: string | undefined): string {
  if (!value) return "";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.valueOf())) return "";
  const offset = parsed.getTimezoneOffset() * 60_000;
  return new Date(parsed.valueOf() - offset).toISOString().slice(0, 16);
}

function timestampPlanValue(value: string): string {
  if (!value) return "";
  const parsed = new Date(value);
  return Number.isNaN(parsed.valueOf()) ? "" : parsed.toISOString();
}

function newQueryIdempotencyKey(): string {
  const random = globalThis.crypto?.randomUUID?.().replaceAll("-", "");
  return `query-${random ?? `${Date.now()}-${draftID()}`}`;
}

function valueKindLabel(kind: QueryField["kind"]): string {
  return (
    {
      text: "texto",
      long_text: "texto longo",
      identifier: "identificador",
      integer: "inteiro",
      decimal: "decimal",
      boolean: "sim/não",
      civil_date: "data",
      civil_month: "mês",
      timestamp: "data e hora",
      enum: "opção",
    } as const
  )[kind];
}

function formatResultCell(cell: QueryResultCell | undefined): string {
  if (!cell) return "—";
  if (cell.is_null) return "—";
  if (cell.text_value !== undefined) return cell.text_value;
  if (cell.integer_value !== undefined)
    return new Intl.NumberFormat("pt-BR").format(cell.integer_value);
  if (cell.decimal_value !== undefined) return cell.decimal_value.replace(".", ",");
  if (cell.boolean_value !== undefined) return cell.boolean_value ? "Sim" : "Não";
  if (cell.civil_date_value !== undefined) return cell.civil_date_value;
  if (cell.timestamp_value !== undefined) return formatTimestamp(cell.timestamp_value);
  return "—";
}

function hasInvalidQueryURLPlan(): boolean {
  const encoded = new URLSearchParams(window.location.search).get("plan");
  return encoded !== null && decodeQueryURLPlan(encoded) === undefined;
}

function formatTimestamp(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.valueOf())
    ? value
    : new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(date);
}

function executionStateLabel(value: string): string {
  return (
    { RUNNING: "Executando", COMPLETED: "Concluída", FAILED: "Falhou", CANCELLED: "Cancelada" }[
      value
    ] ?? value
  );
}

function queryErrorMessage(error: unknown): string {
  if (!(error instanceof APIRequestError)) return "Tente novamente em instantes.";
  if (error.fieldErrors.length > 0)
    return error.fieldErrors.map((field) => field.message).join(" · ");
  switch (error.code) {
    case "query_catalog_stale":
      return "O catálogo mudou. Recarregue a página e revise o plano.";
    case "query_too_costly":
      return "Reduza filtros, relações, colunas ou o máximo de linhas.";
    case "query_expired":
      return "O resultado expirou. Execute o plano novamente.";
    case "query_timeout":
      return "A consulta excedeu o tempo seguro. Reduza o plano e tente novamente.";
    case "query_cancelled":
      return "A consulta foi cancelada antes de produzir um resultado.";
    case "rate_limited":
      return "O limite temporário de consultas foi atingido.";
    default:
      return error.message;
  }
}
