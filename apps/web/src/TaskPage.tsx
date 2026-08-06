import { Alert, Button, Card, Flex, Layout, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { taskRoute } from "./App";
import { APIRequestError } from "./lib/api/client";
import {
  cancelTaskJob,
  createTaskDraft,
  executeAdvancedQueryPlan,
  getAdvancedQueryCatalog,
  getAdvancedQueryResult,
  getTaskCapability,
  getTaskDraft,
  getTaskJob,
  getTaskResults,
  interpretTask,
  reviewTaskDraft,
  startTaskJob,
  subscribeTaskEvents,
  validateAdvancedQueryPlan,
  type AdvancedPlanEstimate,
  type AdvancedQueryExecution,
  type AdvancedQueryPlan,
  type TaskDraft,
  type TaskJob,
  type TaskResultPage,
  type TaskSpec,
} from "./lib/api/tasks";

type TaskMode = "visual" | "json" | "task";
export type TaskSearch = {
  mode: TaskMode;
  draft?: string;
  job?: string;
  execution?: string;
};

export function normalizeTaskSearch(input: Record<string, unknown>): TaskSearch {
  const mode = input.mode === "json" || input.mode === "task" ? input.mode : "visual";
  return {
    mode,
    ...(typeof input.draft === "string" && input.draft ? { draft: input.draft } : {}),
    ...(typeof input.job === "string" && input.job ? { job: input.job } : {}),
    ...(typeof input.execution === "string" && input.execution
      ? { execution: input.execution }
      : {}),
  };
}

export function TaskPage() {
  const search = taskRoute.useSearch();
  const navigate = taskRoute.useNavigate();
  const queryClient = useQueryClient();
  const capability = useQuery({
    queryKey: ["task-capability"],
    queryFn: ({ signal }) => getTaskCapability(signal),
  });
  const catalog = useQuery({
    queryKey: ["advanced-query-catalog"],
    queryFn: ({ signal }) => getAdvancedQueryCatalog(signal),
    staleTime: 30_000,
  });
  const [planText, setPlanText] = useState("");
  const [specText, setSpecText] = useState("");
  const [taskText, setTaskText] = useState("");
  const [queryEstimate, setQueryEstimate] = useState<AdvancedPlanEstimate>();
  const [queryExecution, setQueryExecution] = useState<AdvancedQueryExecution>();
  const [streamMessage, setStreamMessage] = useState("");
  const [streamError, setStreamError] = useState(false);

  useEffect(() => {
    if (!catalog.data || planText) return;
    const root = catalog.data.entities[0]?.key ?? "";
    const projection = catalog.data.fields.find(
      (field) => field.entity === root && field.projectable,
    )?.key;
    setPlanText(
      JSON.stringify(
        {
          version: "v2",
          catalog_version: catalog.data.version,
          root_entity: root,
          projections: projection ? [projection] : [],
          maximum_rows: 100,
        } satisfies AdvancedQueryPlan,
        null,
        2,
      ),
    );
    setSpecText(
      JSON.stringify(
        {
          version: "v1",
          catalog_version: catalog.data.version,
          state: "PROPOSED",
          roles: root
            ? [{ key: "candidate", entity: root, minimum_count: 1, maximum_count: 1 }]
            : [],
          requirements: [],
          constraints: [],
          goal: { minimum_solutions: 1, maximum_solutions: 10 },
          ambiguities: [],
        } satisfies TaskSpec,
        null,
        2,
      ),
    );
  }, [catalog.data, planText]);

  const draft = useQuery({
    queryKey: ["task-draft", search.draft],
    queryFn: ({ signal }) => getTaskDraft(search.draft ?? "", signal),
    enabled: Boolean(search.draft),
  });
  useEffect(() => {
    if (draft.data) setSpecText(JSON.stringify(draft.data.spec, null, 2));
  }, [draft.data]);

  const job = useQuery({
    queryKey: ["task-job", search.job],
    queryFn: ({ signal }) => getTaskJob(search.job ?? "", signal),
    enabled: Boolean(search.job),
    refetchInterval: (query) => (isTerminalJob(query.state.data?.state) ? false : 5_000),
  });
  const results = useQuery({
    queryKey: ["task-results", search.job],
    queryFn: ({ signal }) => getTaskResults(search.job ?? "", 100, 0, signal),
    enabled: Boolean(search.job && isResultJob(job.data?.state)),
  });
  const queryResult = useQuery({
    queryKey: ["advanced-query-result", search.execution],
    queryFn: ({ signal }) => getAdvancedQueryResult(search.execution ?? "", 100, 0, signal),
    enabled: Boolean(search.execution),
  });

  useEffect(() => {
    if (!search.job || isTerminalJob(job.data?.state)) return;
    setStreamError(false);
    return subscribeTaskEvents(
      search.job,
      (event) => {
        setStreamMessage(taskEventLabel(event.kind, event.progress_current, event.progress_total));
        void queryClient.invalidateQueries({ queryKey: ["task-job", search.job] });
        if (event.kind === "JOB_COMPLETED" || event.kind === "JOB_INCOMPLETE") {
          void queryClient.invalidateQueries({ queryKey: ["task-results", search.job] });
        }
      },
      () => setStreamError(true),
    );
  }, [job.data?.state, queryClient, search.job]);

  const validateQuery = useMutation({
    mutationFn: () => validateAdvancedQueryPlan(parseJSON<AdvancedQueryPlan>(planText)),
    onSuccess: setQueryEstimate,
  });
  const executeQuery = useMutation({
    mutationFn: () =>
      executeAdvancedQueryPlan(
        parseJSON<AdvancedQueryPlan>(planText),
        `web-v2-${crypto.randomUUID()}`,
      ),
    onSuccess: (execution) => {
      setQueryExecution(execution);
      void navigate({ search: (previous) => ({ ...previous, execution: execution.id }) });
    },
  });
  const interpret = useMutation({
    mutationFn: () => interpretTask(taskText),
    onSuccess: (proposal) => {
      setSpecText(JSON.stringify(proposal.spec, null, 2));
      void navigate({ search: (previous) => ({ ...previous, mode: "task" }) });
    },
  });
  const createDraft = useMutation({
    mutationFn: () => createTaskDraft(parseJSON<TaskSpec>(specText)),
    onSuccess: (created) => {
      setSpecText(JSON.stringify(created.spec, null, 2));
      void navigate({
        search: (previous) => {
          const next = { ...previous, draft: created.id };
          delete next.job;
          return next;
        },
      });
    },
  });
  const reviewDraft = useMutation({
    mutationFn: () => {
      if (!draft.data) throw new Error("Crie ou abra um rascunho antes da revisão.");
      const spec = parseJSON<TaskSpec>(specText);
      return reviewTaskDraft(draft.data.id, { ...spec, state: "REVIEWED" }, draft.data.version);
    },
    onSuccess: (reviewed) => {
      setSpecText(JSON.stringify(reviewed.spec, null, 2));
      void queryClient.setQueryData(["task-draft", reviewed.id], reviewed);
    },
  });
  const startJob = useMutation({
    mutationFn: (retryOf?: string) => {
      if (!draft.data) throw new Error("Revise um rascunho antes de executar.");
      return startTaskJob(draft.data.id, `web-task-${crypto.randomUUID()}`, retryOf);
    },
    onSuccess: (started) => {
      setStreamMessage("Job aceito");
      void navigate({ search: (previous) => ({ ...previous, job: started.id }) });
    },
  });
  const cancelJobMutation = useMutation({
    mutationFn: () => cancelTaskJob(search.job ?? ""),
    onSuccess: (cancelled) => queryClient.setQueryData(["task-job", cancelled.id], cancelled),
  });

  const parsedPlan = useMemo(() => safeParsePlan(planText), [planText]);
  const entityFields = useMemo(() => {
    if (!catalog.data || !parsedPlan) return [];
    return catalog.data.fields.filter((field) => field.entity === parsedPlan.root_entity);
  }, [catalog.data, parsedPlan]);

  return (
    <Layout style={{ maxWidth: "lg", margin: "0 auto" }}>
      <header className="page-header">
        <div className="page-eyebrow">M14 · Consultas avançadas</div>
        <Typography.Title level={1} className="page-title">
          Consultas avançadas e tarefas
        </Typography.Title>
        <Typography.Paragraph className="page-description">
          Monte consultas tipadas ou revise uma tarefa completa antes de qualquer execução somente
          leitura.
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex vertical gap="1.25rem">
          <Flex gap="0.5rem" wrap>
            {(["visual", "json", "task"] as const).map((mode) => (
              <Button
                key={mode}
                type={search.mode === mode ? "primary" : "default"}
                onClick={() => void navigate({ search: (previous) => ({ ...previous, mode }) })}
              >
                {mode === "visual"
                  ? "Visual"
                  : mode === "json"
                    ? "QueryPlan JSON"
                    : "Tarefa completa"}
              </Button>
            ))}
          </Flex>

          {capability.isError ? <RequestError error={capability.error} /> : null}
          {catalog.isError ? <RequestError error={catalog.error} /> : null}
          {capability.data && !capability.data.semantic_interpretation ? (
            <Alert
              type="info"
              title="Interpretação semântica desativada"
              description="A construção tipada continua disponível. Nenhum provedor externo recebe o texto da
              tarefa."
            />
          ) : null}

          {search.mode === "visual" ? (
            <VisualQueryBuilder
              catalog={catalog.data}
              plan={parsedPlan}
              fields={entityFields}
              onChange={(plan) => setPlanText(JSON.stringify(plan, null, 2))}
            />
          ) : null}

          {search.mode === "json" ? (
            <Card className="task-workspace__panel" style={{ padding: "1rem" }}>
              <label className="task-workspace__field">
                QueryPlan v2
                <textarea
                  aria-label="QueryPlan v2"
                  rows={22}
                  value={planText}
                  onChange={(event) => setPlanText(event.target.value)}
                  spellCheck={false}
                />
              </label>
            </Card>
          ) : null}

          {search.mode !== "task" ? (
            <Card className="task-workspace__panel" style={{ padding: "1rem" }}>
              <Flex gap="0.75rem" wrap>
                <Button onClick={() => validateQuery.mutate()} disabled={validateQuery.isPending}>
                  Validar plano
                </Button>
                <Button
                  type="primary"
                  onClick={() => executeQuery.mutate()}
                  disabled={executeQuery.isPending}
                >
                  Executar consulta
                </Button>
              </Flex>
              <MutationError mutation={validateQuery} />
              <MutationError mutation={executeQuery} />
              {queryEstimate ? (
                <p role="status">
                  Plano válido · custo {queryEstimate.cost ?? 0} ·{" "}
                  {queryEstimate.columns?.length ?? 0} colunas
                </p>
              ) : null}
              {queryExecution ? (
                <p>
                  Execução {queryExecution.state}: {queryExecution.id}
                </p>
              ) : null}
            </Card>
          ) : null}

          {queryResult.data ? <AdvancedQueryResultView result={queryResult.data} /> : null}
          {queryResult.isError ? <RequestError error={queryResult.error} /> : null}

          {search.mode === "task" ? (
            <>
              <Card className="task-workspace__panel" style={{ padding: "1rem" }}>
                <Flex vertical gap="0.75rem">
                  <label className="task-workspace__field">
                    Texto original da tarefa
                    <textarea
                      rows={8}
                      value={taskText}
                      onChange={(event) => setTaskText(event.target.value)}
                      placeholder="Cole a tarefa completa aqui. A interpretação nunca executa automaticamente."
                    />
                  </label>
                  <Button
                    onClick={() => interpret.mutate()}
                    disabled={
                      !capability.data?.semantic_interpretation ||
                      interpret.isPending ||
                      !taskText.trim()
                    }
                  >
                    Propor especificação
                  </Button>
                  <MutationError mutation={interpret} />
                </Flex>
              </Card>

              <Card className="task-workspace__panel" style={{ padding: "1rem" }}>
                <Flex vertical gap="0.75rem">
                  <label className="task-workspace__field">
                    TaskSpec revisável
                    <textarea
                      aria-label="TaskSpec revisável"
                      rows={28}
                      value={specText}
                      onChange={(event) => setSpecText(event.target.value)}
                      spellCheck={false}
                    />
                  </label>
                  <Flex gap="0.75rem" wrap>
                    <Button onClick={() => createDraft.mutate()} disabled={createDraft.isPending}>
                      Salvar rascunho
                    </Button>
                    <Button
                      onClick={() => reviewDraft.mutate()}
                      disabled={!draft.data || reviewDraft.isPending}
                    >
                      Confirmar revisão
                    </Button>
                    <Button
                      type="primary"
                      onClick={() => startJob.mutate(undefined)}
                      disabled={draft.data?.state !== "REVIEWED" || startJob.isPending}
                    >
                      Executar tarefa
                    </Button>
                  </Flex>
                  <MutationError mutation={createDraft} />
                  <MutationError mutation={reviewDraft} />
                  <MutationError mutation={startJob} />
                  {draft.data ? <DraftSummary draft={draft.data} /> : null}
                </Flex>
              </Card>
            </>
          ) : null}

          {job.data ? (
            <JobPanel
              job={job.data}
              streamMessage={streamMessage}
              streamError={streamError}
              onCancel={() => cancelJobMutation.mutate()}
              onRetry={() => startJob.mutate(job.data.id)}
              cancelling={cancelJobMutation.isPending}
              retrying={startJob.isPending}
            />
          ) : null}
          {job.isError ? <RequestError error={job.error} /> : null}
          {results.data ? <TaskResultsView page={results.data} /> : null}
          {results.isError ? <RequestError error={results.error} /> : null}
        </Flex>
      </div>
    </Layout>
  );
}

function VisualQueryBuilder({
  catalog,
  plan,
  fields,
  onChange,
}: {
  catalog: Awaited<ReturnType<typeof getAdvancedQueryCatalog>> | undefined;
  plan: AdvancedQueryPlan | undefined;
  fields: Array<{ key: string; label: string; projectable: boolean; sortable: boolean }>;
  onChange: (plan: AdvancedQueryPlan) => void;
}) {
  if (!catalog || !plan)
    return (
      <Card className="task-workspace__panel" style={{ padding: "1rem" }}>
        Carregando catálogo…
      </Card>
    );
  const projections = plan.projections ?? [];
  const groupBy = plan.group_by ?? [];
  const aggregates = plan.aggregates ?? [];
  return (
    <Card className="task-workspace__panel" style={{ padding: "1rem" }}>
      <Flex vertical gap="1rem">
        <div className="task-workspace__grid">
          <label className="task-workspace__field">
            Entidade principal
            <select
              value={plan.root_entity ?? ""}
              onChange={(event) =>
                onChange({
                  ...plan,
                  root_entity: event.target.value,
                  projections: [],
                  group_by: [],
                  aggregates: [],
                })
              }
            >
              {catalog.entities.map((entity) => (
                <option key={entity.key} value={entity.key}>
                  {entity.label}
                </option>
              ))}
            </select>
          </label>
          <label className="task-workspace__field">
            Máximo de linhas
            <input
              type="number"
              min={1}
              max={500}
              value={plan.maximum_rows}
              onChange={(event) => onChange({ ...plan, maximum_rows: Number(event.target.value) })}
            />
          </label>
        </div>
        <fieldset className="task-workspace__fieldset">
          <legend>Colunas</legend>
          <div className="task-workspace__checks">
            {fields
              .filter((field) => field.projectable)
              .map((field) => (
                <label key={field.key}>
                  <input
                    type="checkbox"
                    checked={projections.includes(field.key)}
                    onChange={() =>
                      onChange({ ...plan, projections: toggle(projections, field.key) })
                    }
                  />
                  {field.label}
                </label>
              ))}
          </div>
        </fieldset>
        <fieldset className="task-workspace__fieldset">
          <legend>Agrupar por</legend>
          <div className="task-workspace__checks">
            {fields
              .filter((field) => field.sortable)
              .map((field) => (
                <label key={field.key}>
                  <input
                    type="checkbox"
                    checked={groupBy.includes(field.key)}
                    onChange={() =>
                      onChange({ ...plan, group_by: toggle(groupBy, field.key), projections: [] })
                    }
                  />
                  {field.label}
                </label>
              ))}
          </div>
        </fieldset>
        <Flex gap="0.75rem" wrap>
          <Button
            onClick={() =>
              onChange({
                ...plan,
                projections: [],
                aggregates: [
                  ...aggregates,
                  { key: `count_${aggregates.length + 1}`, function: "count" },
                ],
              })
            }
          >
            Adicionar contagem
          </Button>
          <Button
            onClick={() => {
              const field = fields.find((candidate) => candidate.projectable);
              if (!field) return;
              onChange({
                ...plan,
                projections: [],
                patterns: [
                  ...(plan.patterns ?? []),
                  { field: field.key, grammar: "literal_sequence", pattern: "", case_fold: true },
                ],
              });
            }}
          >
            Adicionar padrão
          </Button>
        </Flex>
        {(plan.patterns ?? []).map((pattern, index) => (
          <div className="task-workspace__grid" key={`${pattern.field}-${index}`}>
            <label className="task-workspace__field">
              Campo do padrão
              <select
                value={pattern.field}
                onChange={(event) =>
                  onChange({
                    ...plan,
                    patterns: replaceAt(plan.patterns ?? [], index, {
                      ...pattern,
                      field: event.target.value,
                    }),
                  })
                }
              >
                {fields.map((field) => (
                  <option key={field.key} value={field.key}>
                    {field.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="task-workspace__field">
              Gramática
              <select
                value={pattern.grammar}
                onChange={(event) =>
                  onChange({
                    ...plan,
                    patterns: replaceAt(plan.patterns ?? [], index, {
                      ...pattern,
                      grammar: event.target.value as typeof pattern.grammar,
                    }),
                  })
                }
              >
                <option value="literal_sequence">Sequência literal</option>
                <option value="character_class">Caracteres permitidos</option>
                <option value="binary_digits">Dígitos binários</option>
                <option value="digits">Dígitos</option>
                <option value="letters">Letras</option>
                <option value="alphanumeric">Alfanumérico</option>
              </select>
            </label>
            <label className="task-workspace__field">
              Padrão
              <input
                value={pattern.pattern}
                onChange={(event) =>
                  onChange({
                    ...plan,
                    patterns: replaceAt(plan.patterns ?? [], index, {
                      ...pattern,
                      pattern: event.target.value,
                    }),
                  })
                }
              />
            </label>
          </div>
        ))}
      </Flex>
    </Card>
  );
}

function AdvancedQueryResultView({
  result,
}: {
  result: Awaited<ReturnType<typeof getAdvancedQueryResult>>;
}) {
  return (
    <Card className="task-workspace__panel task-workspace__table-wrap" style={{ padding: "1rem" }}>
      <h2>Resultado da consulta</h2>
      <table className="task-workspace__table">
        <thead>
          <tr>
            {result.columns.map((column) => (
              <th key={column.position}>{column.label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {result.rows.map((row) => (
            <tr key={`${row.entity_kind}-${row.entity_id}`}>
              {(row.cells ?? []).map((cell) => (
                <td key={cell.column_position}>{renderCell(cell)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  );
}

function DraftSummary({ draft }: { draft: TaskDraft }) {
  return (
    <p role="status">
      Rascunho {draft.state} · versão {draft.version} · {draft.id}
    </p>
  );
}

function JobPanel({
  job,
  streamMessage,
  streamError,
  onCancel,
  onRetry,
  cancelling,
  retrying,
}: {
  job: TaskJob;
  streamMessage: string;
  streamError: boolean;
  onCancel: () => void;
  onRetry: () => void;
  cancelling: boolean;
  retrying: boolean;
}) {
  return (
    <Card className="task-workspace__panel" style={{ padding: "1rem" }}>
      <Flex vertical gap="0.75rem">
        <Flex gap="0.75rem" align="center" wrap>
          <h2>Job</h2>
          <Tag color={jobTone(job.state)}>{job.state}</Tag>
        </Flex>
        <p>
          {streamMessage ||
            `${job.progress_current}/${job.progress_total} etapas · ${job.candidate_count} candidatos`}
        </p>
        {streamError && !isTerminalJob(job.state) ? (
          <Alert
            type="warning"
            title="Reconectando eventos"
            description="O status também será conferido por consulta periódica."
          />
        ) : null}
        <Flex gap="0.75rem" wrap>
          {!isTerminalJob(job.state) ? (
            <Button onClick={onCancel} disabled={cancelling}>
              Cancelar
            </Button>
          ) : null}
          {job.state === "FAILED" || job.state === "CANCELLED" || job.state === "INCOMPLETE" ? (
            <Button onClick={onRetry} disabled={retrying}>
              Repetir explicitamente
            </Button>
          ) : null}
        </Flex>
      </Flex>
    </Card>
  );
}

function TaskResultsView({ page }: { page: TaskResultPage }) {
  return (
    <Card className="task-workspace__panel" style={{ padding: "1rem" }}>
      <h2>Composições encontradas ({page.total})</h2>
      <Flex vertical gap="0.75rem">
        {page.compositions.map((composition) => (
          <details key={composition.position} className="task-workspace__result">
            <summary>Composição {composition.position + 1}</summary>
            {Object.entries(composition.selected).map(([role, candidates]) => (
              <div key={role}>
                <strong>{role}</strong>
                <ul>
                  {candidates.map((candidate) => (
                    <li key={`${candidate.entity}-${candidate.id}`}>
                      {candidate.label}{" "}
                      <small>
                        ({candidate.entity} · {candidate.id})
                      </small>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
            <details>
              <summary>Evidências ({composition.evidence.length})</summary>
              <ul>
                {composition.evidence.map((evidence, index) => (
                  <li key={`${evidence.requirement ?? evidence.constraint}-${index}`}>
                    {evidence.satisfied ? "✓" : "✕"} {evidence.requirement ?? evidence.constraint}:{" "}
                    {evidence.source_entity}/{evidence.source_id}
                  </li>
                ))}
              </ul>
            </details>
          </details>
        ))}
      </Flex>
    </Card>
  );
}

function RequestError({ error }: { error: unknown }) {
  const message =
    error instanceof APIRequestError ? error.message : "Não foi possível carregar os dados.";
  return <Alert type="error" title="Falha na solicitação" description={<>{message}</>} />;
}

function MutationError({ mutation }: { mutation: { isError: boolean; error: unknown } }) {
  return mutation.isError ? <RequestError error={mutation.error} /> : null;
}

function parseJSON<T>(value: string): T {
  return JSON.parse(value) as T;
}
function safeParsePlan(value: string): AdvancedQueryPlan | undefined {
  try {
    return parseJSON<AdvancedQueryPlan>(value);
  } catch {
    return undefined;
  }
}
function toggle(values: string[], value: string): string[] {
  return values.includes(value) ? values.filter((item) => item !== value) : [...values, value];
}
function replaceAt<T>(values: T[], index: number, value: T): T[] {
  return values.map((item, itemIndex) => (itemIndex === index ? value : item));
}
function isTerminalJob(state: TaskJob["state"] | undefined): boolean {
  return (
    state === "COMPLETED" || state === "INCOMPLETE" || state === "FAILED" || state === "CANCELLED"
  );
}
function isResultJob(state: TaskJob["state"] | undefined): boolean {
  return state === "COMPLETED" || state === "INCOMPLETE";
}
function taskEventLabel(kind: string, current?: number, total?: number): string {
  return `${kind.replaceAll("_", " ")}${current !== undefined && total !== undefined ? ` · ${current}/${total}` : ""}`;
}
function jobTone(state: TaskJob["state"]): "neutral" | "info" | "success" | "warning" | "danger" {
  if (state === "COMPLETED") return "success";
  if (state === "INCOMPLETE") return "warning";
  if (state === "FAILED" || state === "CANCELLED") return "danger";
  if (state === "RUNNING") return "info";
  return "neutral";
}
function renderCell(cell: Record<string, unknown>): string {
  for (const key of [
    "text_value",
    "integer_value",
    "decimal_value",
    "boolean_value",
    "civil_date_value",
    "timestamp_value",
  ]) {
    const value = cell[key];
    if (value !== undefined && value !== null) return String(value);
  }
  return "—";
}
