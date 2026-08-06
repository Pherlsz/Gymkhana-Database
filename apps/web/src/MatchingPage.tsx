import { Alert, Button, Card, Flex, Layout, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";
import { matchingRoute, useApplicationSession } from "./App";
import { normalizeProfileSearch } from "./ProfilesPage";
import { APIRequestError } from "./lib/api/client";
import {
  cancelMatchingAnalysis,
  dismissMatchingCase,
  getMatchingAnalysis,
  getMatchingCase,
  getMatchingCatalog,
  listMatchingCases,
  mergeMatchingProfiles,
  previewMatchingMerge,
  startMatchingAnalysis,
  type MatchingAnalysis,
  type MatchingCase,
  type MatchingCaseSort,
  type MatchingCaseState,
  type MatchingFieldSource,
  type MatchingMergePreview,
  type MatchingMergeResult,
  type MatchingPreviewRequest,
  type MatchingProfile,
  type MatchingScoreBand,
  type MatchingSortOrder,
} from "./lib/api/matching";

const identifierPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const matchingStates = ["PENDING", "NOT_DUPLICATE", "MERGED", "STALE"] as const;
const scoreBands = ["LOW", "MEDIUM", "HIGH"] as const;
const pageSize = 25;

export type MatchingSearchState = {
  matching_state: MatchingCaseState;
  matching_band: "" | MatchingScoreBand;
  matching_page: number;
  matching_sort: MatchingCaseSort;
  matching_order: MatchingSortOrder;
  matching_case?: string | undefined;
};

export function normalizeMatchingSearch(search: Record<string, unknown>): MatchingSearchState {
  const rawState = typeof search.matching_state === "string" ? search.matching_state : "";
  const rawBand = typeof search.matching_band === "string" ? search.matching_band : "";
  const rawPage = Number(search.matching_page);
  const rawSort = typeof search.matching_sort === "string" ? search.matching_sort : "";
  const rawOrder = typeof search.matching_order === "string" ? search.matching_order : "";
  const rawCase = typeof search.matching_case === "string" ? search.matching_case.trim() : "";
  return {
    matching_state: matchingStates.includes(rawState as MatchingCaseState)
      ? (rawState as MatchingCaseState)
      : "PENDING",
    matching_band: scoreBands.includes(rawBand as MatchingScoreBand)
      ? (rawBand as MatchingScoreBand)
      : "",
    matching_page: Number.isInteger(rawPage) && rawPage >= 1 && rawPage <= 400 ? rawPage : 1,
    matching_sort: ["score", "updated_at", "created_at"].includes(rawSort)
      ? (rawSort as MatchingCaseSort)
      : "score",
    matching_order: rawOrder === "asc" ? "asc" : "desc",
    ...(identifierPattern.test(rawCase) ? { matching_case: rawCase } : {}),
  };
}

export function MatchingPage() {
  const session = useApplicationSession();
  const queryClient = useQueryClient();
  const search = matchingRoute.useSearch();
  const navigate = matchingRoute.useNavigate();
  const [analysisID, setAnalysisID] = useState("");
  const [lastMerge, setLastMerge] = useState<MatchingMergeResult | null>(null);
  const completedAnalysis = useRef("");
  const analysisIdempotencyKey = useRef(newMatchingIdempotencyKey("analysis"));
  const canMerge = session.user.role === "ADMIN" || session.user.role === "SUPERADMIN";
  const setSearch = (patch: Partial<MatchingSearchState>) => {
    void navigate({
      replace: true,
      search: (current) => normalizeMatchingSearch({ ...current, ...patch }),
    });
  };

  const catalog = useQuery({
    queryKey: ["matching-catalog", session.user.role],
    queryFn: ({ signal }) => getMatchingCatalog(signal),
    staleTime: 60_000,
  });
  const cases = useQuery({
    queryKey: [
      "matching-cases",
      search.matching_state,
      search.matching_band,
      search.matching_sort,
      search.matching_order,
      search.matching_page,
    ],
    queryFn: ({ signal }) =>
      listMatchingCases(
        {
          states: [search.matching_state],
          bands: search.matching_band ? [search.matching_band] : [],
          sort: search.matching_sort,
          order: search.matching_order,
          limit: pageSize,
          offset: (search.matching_page - 1) * pageSize,
        },
        signal,
      ),
  });
  const selected = useQuery({
    queryKey: ["matching-case", search.matching_case],
    queryFn: ({ signal }) => getMatchingCase(search.matching_case!, signal),
    enabled: Boolean(search.matching_case),
    retry: (attempt, error) =>
      !(error instanceof APIRequestError && [400, 403, 404].includes(error.status)) && attempt < 1,
  });
  const startAnalysis = useMutation({
    mutationFn: () => startMatchingAnalysis(analysisIdempotencyKey.current),
    onSuccess: (value) => {
      completedAnalysis.current = "";
      analysisIdempotencyKey.current = newMatchingIdempotencyKey("analysis");
      setAnalysisID(value.id);
    },
  });
  const analysis = useQuery({
    queryKey: ["matching-analysis", analysisID],
    queryFn: ({ signal }) => getMatchingAnalysis(analysisID, signal),
    enabled: Boolean(analysisID),
    ...(startAnalysis.data?.id === analysisID ? { placeholderData: startAnalysis.data } : {}),
    refetchInterval: (query) =>
      query.state.data && analysisActive(query.state.data.state) ? 1_500 : false,
  });
  const cancelAnalysis = useMutation({
    mutationFn: () => cancelMatchingAnalysis(analysisID),
    onSuccess: (value) => queryClient.setQueryData(["matching-analysis", analysisID], value),
  });

  useEffect(() => {
    const value = analysis.data;
    if (!value || analysisActive(value.state) || completedAnalysis.current === value.id) return;
    completedAnalysis.current = value.id;
    if (value.state === "COMPLETED") {
      void queryClient.invalidateQueries({ queryKey: ["matching-cases"] });
    }
  }, [analysis.data, queryClient]);

  const refreshCases = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["matching-cases"] }),
      queryClient.invalidateQueries({ queryKey: ["matching-case", search.matching_case] }),
    ]);
  };
  const closeCase = () => setSearch({ matching_case: undefined });
  const pageCount = Math.max(1, Math.ceil((cases.data?.total ?? 0) / pageSize));

  return (
    <Layout style={{ maxWidth: "lg", margin: "0 auto" }}>
      <header className="page-header">
        <div className="page-eyebrow">M11 · Matching de perfis</div>
        <Typography.Title level={1} className="page-title">Revisão de possíveis duplicidades</Typography.Title>
        <Typography.Paragraph className="page-description">
          Gere candidatos sob demanda, confira as evidências e registre uma decisão humana. Nenhum
          perfil é mesclado automaticamente.
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex vertical gap="1.5rem">
          {catalog.isError ? (
            <Alert message="Matching indisponível" type="error" description={<>{matchingError(catalog.error)}
            </>} />
          ) : null}
          {lastMerge ? (
            <Alert message="Perfis mesclados" type="success" description={<>A pessoa preservada foi atualizada para a versão {lastMerge.survivor_version}; todas
              as dependências indicadas na prévia foram movidas atomicamente.
            </>} />
          ) : null}
          <AnalysisPanel
            analysis={analysis.data ?? startAnalysis.data}
            cancelError={cancelAnalysis.error}
            cancelling={cancelAnalysis.isPending}
            error={startAnalysis.error ?? analysis.error}
            limits={catalog.data}
            onCancel={() => cancelAnalysis.mutate()}
            onStart={() => {
              setLastMerge(null);
              startAnalysis.mutate();
            }}
            starting={startAnalysis.isPending}
          />
          <section className="page-section"
          >
      <Typography.Title level={2}>Fila de revisão</Typography.Title>
      <Typography.Paragraph>Casos persistem entre análises. Um descarte só é reaberto quando a versão de um dos perfis muda.</Typography.Paragraph>
            <Flex vertical gap="1rem">
              <Flex className="matching-filters">
                <label>
                  Decisão
                  <select
                    aria-label="Decisão do caso"
                    value={search.matching_state}
                    onChange={(event) =>
                      setSearch({
                        matching_state: event.target.value as MatchingCaseState,
                        matching_page: 1,
                        matching_case: undefined,
                      })
                    }
                  >
                    {matchingStates.map((state) => (
                      <option key={state} value={state}>
                        {caseStateLabel(state)}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Faixa de confiança
                  <select
                    aria-label="Faixa de confiança"
                    value={search.matching_band}
                    onChange={(event) =>
                      setSearch({
                        matching_band: event.target.value as "" | MatchingScoreBand,
                        matching_page: 1,
                        matching_case: undefined,
                      })
                    }
                  >
                    <option value="">Todas</option>
                    {scoreBands.map((band) => (
                      <option key={band} value={band}>
                        {scoreBandLabel(band)}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Ordenar por
                  <select
                    aria-label="Ordenar casos por"
                    value={search.matching_sort}
                    onChange={(event) =>
                      setSearch({
                        matching_sort: event.target.value as MatchingCaseSort,
                        matching_page: 1,
                        matching_case: undefined,
                      })
                    }
                  >
                    <option value="score">Pontuação</option>
                    <option value="updated_at">Atualização</option>
                    <option value="created_at">Criação</option>
                  </select>
                </label>
                <label>
                  Direção
                  <select
                    aria-label="Direção da ordenação"
                    value={search.matching_order}
                    onChange={(event) =>
                      setSearch({
                        matching_order: event.target.value as MatchingSortOrder,
                        matching_page: 1,
                        matching_case: undefined,
                      })
                    }
                  >
                    <option value="desc">Decrescente</option>
                    <option value="asc">Crescente</option>
                  </select>
                </label>
                <Button disabled={cases.isFetching} onClick={() => void refreshCases()}>
                  {cases.isFetching ? "Atualizando…" : "Atualizar fila"}
                </Button>
              </Flex>
              {cases.isError ? (
                <Alert message="Não foi possível carregar os casos" type="error" description={<>{matchingError(cases.error)}
                </>} />
              ) : null}
              <div aria-label="Casos de possível duplicidade" className="matching-case-list">
                {cases.data?.cases.map((value) => (
                  <CaseCard
                    key={value.id}
                    onOpen={() => setSearch({ matching_case: value.id })}
                    selected={search.matching_case === value.id}
                    value={value}
                  />
                ))}
                {!cases.isLoading && cases.data?.cases.length === 0 ? (
                  <Alert message="Nenhum caso nesta seleção" type="info" description="Execute uma análise ou altere os filtros para consultar decisões anteriores." />
                ) : null}
                {cases.isLoading ? <span>Carregando casos…</span> : null}
              </div>
              <Flex align="center" className="matching-pagination">
                <Button
                  disabled={search.matching_page <= 1}
                  onClick={() =>
                    setSearch({
                      matching_page: search.matching_page - 1,
                      matching_case: undefined,
                    })
                  }
                >
                  Anterior
                </Button>
                <span>
                  Página {search.matching_page} de {pageCount} · {cases.data?.total ?? 0} casos
                </span>
                <Button
                  disabled={search.matching_page >= pageCount}
                  onClick={() =>
                    setSearch({
                      matching_page: search.matching_page + 1,
                      matching_case: undefined,
                    })
                  }
                >
                  Próxima
                </Button>
              </Flex>
            </Flex>
          </section>
          {search.matching_case ? (
            <CaseWorkspace
              canMerge={canMerge && catalog.data?.can_merge === true}
              error={selected.error}
              evidenceLabels={
                new Map(catalog.data?.evidence.map((value) => [value.kind, value.label]) ?? [])
              }
              loading={selected.isLoading}
              onClose={closeCase}
              onDismissed={async () => {
                closeCase();
                await refreshCases();
              }}
              onMerged={async (result) => {
                setLastMerge(result);
                closeCase();
                await Promise.all([
                  refreshCases(),
                  queryClient.invalidateQueries({ queryKey: ["profiles"] }),
                  queryClient.invalidateQueries({ queryKey: ["global-search"] }),
                  queryClient.invalidateQueries({ queryKey: ["query-result"] }),
                  queryClient.invalidateQueries({ queryKey: ["query-catalog"] }),
                ]);
              }}
              value={selected.data}
            />
          ) : null}
        </Flex>
      </div>
    </Layout>
  );
}

function AnalysisPanel({
  analysis,
  cancelError,
  cancelling,
  error,
  limits,
  onCancel,
  onStart,
  starting,
}: {
  analysis: MatchingAnalysis | undefined;
  cancelError: Error | null;
  cancelling: boolean;
  error: Error | null;
  limits:
    | {
        maximum_candidates: number;
        maximum_analyses_per_window: number;
        analysis_window_seconds: number;
      }
    | undefined;
  onCancel: () => void;
  onStart: () => void;
  starting: boolean;
}) {
  return (
    <Card className="matching-analysis" style={{ padding: "1rem" }}>
      <Flex vertical gap="1rem">
        <Flex align="center" className="matching-heading">
          <div>
            <h2>Análise sob demanda</h2>
            <p className="matching-muted">
              Compara somente perfis, com pontuação fixa e evidências explicáveis. Limite de até{" "}
              {limits?.maximum_candidates ?? 2_000} candidatos por execução.
            </p>
          </div>
          <Flex>
            <Button
              disabled={starting || (analysis ? analysisActive(analysis.state) : false)}
              onClick={onStart}
            >
              {starting ? "Solicitando…" : "Analisar perfis"}
            </Button>
            {analysis && analysisActive(analysis.state) ? (
              <Button disabled={cancelling} onClick={onCancel}>
                {cancelling ? "Cancelando…" : "Cancelar"}
              </Button>
            ) : null}
          </Flex>
        </Flex>
        {limits ? (
          <span className="matching-muted">
            {limits.maximum_analyses_per_window} análises por{" "}
            {formatDuration(limits.analysis_window_seconds)}.
          </span>
        ) : null}
        {error || cancelError ? (
          <Alert message="Não foi possível concluir a solicitação" type="error" description={<>{matchingError(error ?? cancelError)}
          </>} />
        ) : null}
        {analysis ? (
          <div className="matching-analysis-summary" aria-live="polite">
            <div>
              <span>Estado</span>
              <AnalysisStatus value={analysis.state} />
            </div>
            <div>
              <span>Perfis verificados</span>
              <strong>{analysis.profiles_scanned}</strong>
            </div>
            <div>
              <span>Candidatos</span>
              <strong>{analysis.candidate_count}</strong>
            </div>
            <div>
              <span>Casos atualizados</span>
              <strong>{analysis.refreshed_count}</strong>
            </div>
          </div>
        ) : null}
      </Flex>
    </Card>
  );
}

function CaseCard({
  value,
  selected,
  onOpen,
}: {
  value: MatchingCase;
  selected: boolean;
  onOpen: () => void;
}) {
  return (
    <article className={`matching-case-card${selected ? " matching-case-card--selected" : ""}`}>
      <Flex align="center" className="matching-heading">
        <div>
          <strong>
            {value.left?.full_name ?? "Perfil A"} × {value.right?.full_name ?? "Perfil B"}
          </strong>
          <p className="matching-muted">
            {value.evidence.length} evidências · atualizado em {formatDate(value.updated_at)}
          </p>
        </div>
        <Flex>
          <Tag color={scoreBandTone(value.score_band)}>
            {value.score}% · {scoreBandLabel(value.score_band)}
          </Tag>
          <Tag color={caseStateTone(value.state)}>{caseStateLabel(value.state)}</Tag>
          <Button onClick={onOpen}>Revisar</Button>
        </Flex>
      </Flex>
    </article>
  );
}

function CaseWorkspace({
  canMerge,
  error,
  evidenceLabels,
  loading,
  onClose,
  onDismissed,
  onMerged,
  value,
}: {
  canMerge: boolean;
  error: Error | null;
  evidenceLabels: Map<string, string>;
  loading: boolean;
  onClose: () => void;
  onDismissed: () => Promise<void>;
  onMerged: (result: MatchingMergeResult) => Promise<void>;
  value: MatchingCase | undefined;
}) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (value?.id) heading.current?.focus();
  }, [value?.id]);
  const dismiss = useMutation({
    mutationFn: () => dismissMatchingCase(value!.id, value!.version),
    onSuccess: onDismissed,
  });
  return (
    <section className="page-section"
    >
      <Typography.Title level={2}>Revisão do caso</Typography.Title>
      <Typography.Paragraph>Confira os valores originais. Evidência indica semelhança, não prova identidade.</Typography.Paragraph>
      <Card className="matching-workspace" style={{ padding: "1rem" }}>
        <Flex vertical gap="1.25rem">
          <Flex align="center" className="matching-heading">
            <div>
              <h2 ref={heading} tabIndex={-1}>
                Comparação detalhada
              </h2>
              {value ? (
                <p className="matching-muted">
                  Pontuação {value.score}% · versão do caso {value.version}
                </p>
              ) : null}
            </div>
            <Button onClick={onClose}>Fechar</Button>
          </Flex>
          {loading ? <span>Carregando comparação…</span> : null}
          {error ? (
            <Alert message="Caso indisponível" type="error" description={<>{matchingError(error)}
            </>} />
          ) : null}
          {dismiss.isError ? (
            <Alert message="Não foi possível registrar a decisão" type="error" description={<>{matchingError(dismiss.error)}
            </>} />
          ) : null}
          {value ? (
            <>
              {value.state !== "PENDING" ? (
                <Alert title={`Caso ${caseStateLabel(value.state).toLowerCase()}`} type="info" description="Este histórico está em modo de consulta e não aceita uma nova decisão nesta
                  versão." />
              ) : null}
              <EvidencePanel labels={evidenceLabels} value={value} />
              {value.left && value.right ? (
                <ProfileComparison left={value.left} right={value.right} />
              ) : (
                <Alert message="Perfis não disponíveis" type="warning" description="Um dos perfis mudou ou não está mais acessível. Execute uma nova análise." />
              )}
              {value.state === "PENDING" ? (
                <Flex>
                  <Button
                    disabled={dismiss.isPending}
                    onClick={() => {
                      if (
                        window.confirm("Confirmar que estes perfis não representam a mesma pessoa?")
                      ) {
                        dismiss.mutate();
                      }
                    }}
                  >
                    {dismiss.isPending ? "Registrando…" : "Não são duplicados"}
                  </Button>
                </Flex>
              ) : null}
              {value.state === "PENDING" && canMerge && value.left && value.right ? (
                <MergeWorkspace onMerged={onMerged} value={value} />
              ) : value.state === "PENDING" && !canMerge ? (
                <Alert message="Mesclagem requer administrador" type="info" description="Membros podem revisar e descartar casos. A exclusão do perfil absorvido e a
                  transferência das dependências exigem ADMIN ou SUPERADMIN." />
              ) : null}
            </>
          ) : null}
        </Flex>
      </Card>
    </section>
  );
}

function EvidencePanel({ labels, value }: { labels: Map<string, string>; value: MatchingCase }) {
  return (
    <div>
      <h3>Evidências da pontuação</h3>
      <div className="matching-evidence-list">
        {value.evidence.map((evidence) => (
          <div className="matching-evidence" key={evidence.kind}>
            <strong>{labels.get(evidence.kind) ?? evidence.kind}</strong>
            <span>Força {evidence.strength}%</span>
            <span>+{evidence.contribution} pontos</span>
          </div>
        ))}
      </div>
    </div>
  );
}

const comparisonFields: Array<{ key: keyof MatchingProfile; label: string }> = [
  { key: "full_name", label: "Nome completo" },
  { key: "social_name", label: "Nome social" },
  { key: "cpf", label: "CPF" },
  { key: "email", label: "E-mail" },
  { key: "mobile_phone", label: "Celular" },
  { key: "landline_phone", label: "Telefone" },
  { key: "address_street", label: "Logradouro" },
  { key: "address_number", label: "Número" },
  { key: "address_complement", label: "Complemento" },
  { key: "address_neighborhood", label: "Bairro" },
  { key: "address_city", label: "Cidade" },
  { key: "address_state", label: "UF" },
  { key: "address_postal_code", label: "CEP" },
  { key: "notes", label: "Observações" },
];

function ProfileComparison({ left, right }: { left: MatchingProfile; right: MatchingProfile }) {
  return (
    <div className="matching-comparison" role="table" aria-label="Comparação dos perfis">
      <div className="matching-comparison__header" role="row">
        <strong role="columnheader">Campo</strong>
        <div role="columnheader">
          <strong>{left.full_name}</strong>
          <ProfileLink id={left.id} />
        </div>
        <div role="columnheader">
          <strong>{right.full_name}</strong>
          <ProfileLink id={right.id} />
        </div>
      </div>
      {comparisonFields.map((field) => (
        <div className="matching-comparison__row" role="row" key={field.key}>
          <strong role="rowheader">{field.label}</strong>
          <span role="cell">{displayProfileValue(left[field.key])}</span>
          <span role="cell">{displayProfileValue(right[field.key])}</span>
        </div>
      ))}
    </div>
  );
}

function ProfileLink({ id }: { id: string }) {
  return (
    <Link
      className="matching-profile-link"
      search={normalizeProfileSearch({ selected: id, mode: "view" })}
      to="/profiles"
    >
      Abrir cadastro
    </Link>
  );
}

function MergeWorkspace({
  value,
  onMerged,
}: {
  value: MatchingCase;
  onMerged: (result: MatchingMergeResult) => Promise<void>;
}) {
  const [survivorID, setSurvivorID] = useState(value.left_profile_id);
  const [choices, setChoices] = useState<Record<string, MatchingFieldSource>>({});
  const [previewSignature, setPreviewSignature] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [idempotencyKey, setIdempotencyKey] = useState(() => newMatchingIdempotencyKey("merge"));
  const survivorIsLeft = survivorID === value.left_profile_id;
  const survivor = survivorIsLeft ? value.left! : value.right!;
  const source = survivorIsLeft ? value.right! : value.left!;
  const request = useMemo<MatchingPreviewRequest>(
    () => ({
      survivor_profile_id: survivor.id,
      source_profile_id: source.id,
      survivor_version: survivor.version,
      source_version: source.version,
      choices: Object.entries(choices)
        .toSorted(([left], [right]) => left.localeCompare(right))
        .map(([field_key, choiceSource]) => ({ field_key, source: choiceSource })),
    }),
    [choices, source.id, source.version, survivor.id, survivor.version],
  );
  const requestSignature = JSON.stringify(request);
  const preview = useMutation({
    mutationFn: (input: MatchingPreviewRequest) => previewMatchingMerge(value.id, input),
    onSuccess: (_, variables) => {
      setPreviewSignature(JSON.stringify(variables));
      setConfirmation("");
      setIdempotencyKey(newMatchingIdempotencyKey("merge"));
    },
  });
  const merge = useMutation({
    mutationFn: (current: MatchingMergePreview) =>
      mergeMatchingProfiles(value.id, {
        ...request,
        preview_fingerprint: current.preview_fingerprint,
        idempotency_key: idempotencyKey,
        confirmation,
      }),
    onSuccess: onMerged,
  });
  const previewFresh = preview.data !== undefined && previewSignature === requestSignature;
  const ready =
    previewFresh &&
    preview.data.unresolved_field_count === 0 &&
    preview.data.conflicts.length === 0;
  const chooseSurvivor = (id: string) => {
    setSurvivorID(id);
    setChoices({});
    preview.reset();
    merge.reset();
    setPreviewSignature("");
    setConfirmation("");
  };

  return (
    <Card className="matching-merge" style={{ padding: "1rem" }}>
      <Flex vertical gap="1rem">
        <div>
          <h3>Mesclagem administrativa</h3>
          <p className="matching-muted">
            Escolha o cadastro que continuará existindo. O outro será removido somente dentro da
            transação que também move todas as dependências.
          </p>
        </div>
        <fieldset className="matching-survivor">
          <legend>Perfil que será preservado</legend>
          {[value.left!, value.right!].map((profile) => (
            <label key={profile.id}>
              <input
                checked={survivorID === profile.id}
                name="matching-survivor"
                onChange={() => chooseSurvivor(profile.id)}
                type="radio"
              />
              {profile.full_name} · versão {profile.version}
            </label>
          ))}
        </fieldset>
        <Flex>
          <Button disabled={preview.isPending} onClick={() => preview.mutate(request)}>
            {preview.isPending
              ? "Calculando…"
              : preview.data
                ? "Atualizar prévia"
                : "Gerar prévia da mesclagem"}
          </Button>
        </Flex>
        {preview.isError ? (
          <Alert message="Não foi possível gerar a prévia" type="error" description={<>{matchingError(preview.error)}
          </>} />
        ) : null}
        {merge.isError ? (
          <Alert message="A mesclagem não foi aplicada" type="error" description={<>{matchingError(merge.error)}
          </>} />
        ) : null}
        {preview.data ? (
          <>
            {!previewFresh ? (
              <Alert message="Prévia desatualizada" type="warning" description="As escolhas mudaram. Atualize a prévia antes de confirmar." />
            ) : null}
            <MergeFields
              choices={choices}
              onChange={(field, fieldSource) =>
                setChoices((current) => ({ ...current, [field]: fieldSource }))
              }
              preview={preview.data}
            />
            <DependencyPreview preview={preview.data} />
            {preview.data.unresolved_field_count > 0 ? (
              <Alert message="Escolhas obrigatórias pendentes" type="warning" description={<>Escolha explicitamente a origem de {preview.data.unresolved_field_count} campo(s) e
                atualize a prévia.
              </>} />
            ) : null}
            {preview.data.conflicts.length > 0 ? (
              <Alert message="Dependências incompatíveis" type="error" description="A mesclagem está bloqueada. Corrija os registros conflitantes nos cadastros e gere
                uma nova prévia." />
            ) : null}
            {ready ? (
              <div className="matching-confirmation">
                <label>
                  Digite exatamente <strong>{preview.data.confirmation}</strong>
                  <input
                    autoComplete="off"
                    aria-label="Confirmação da mesclagem"
                    value={confirmation}
                    onChange={(event) => setConfirmation(event.target.value)}
                  />
                </label>
                <Button
                  disabled={merge.isPending || confirmation !== preview.data.confirmation}
                  onClick={() => merge.mutate(preview.data)}
                >
                  {merge.isPending ? "Mesclando…" : "Mesclar perfis definitivamente"}
                </Button>
              </div>
            ) : null}
          </>
        ) : null}
      </Flex>
    </Card>
  );
}

function MergeFields({
  choices,
  onChange,
  preview,
}: {
  choices: Record<string, MatchingFieldSource>;
  onChange: (field: string, source: MatchingFieldSource) => void;
  preview: MatchingMergePreview;
}) {
  return (
    <div>
      <h4>Precedência dos campos</h4>
      <div className="matching-field-choices">
        {preview.fields.map((field) => (
          <div
            className={
              field.choice_required ? "matching-field matching-field--required" : "matching-field"
            }
            key={field.key}
          >
            <div>
              <strong>{field.label}</strong>
              <span>{field.conflict ? "Valores diferentes" : "Sem conflito"}</span>
            </div>
            <span>{field.survivor_value || "—"}</span>
            <span>{field.source_value || "—"}</span>
            {field.choice_required ? (
              <label>
                Valor preservado
                <select
                  aria-label={`Valor preservado para ${field.label}`}
                  value={choices[field.key] ?? ""}
                  onChange={(event) =>
                    onChange(field.key, event.target.value as MatchingFieldSource)
                  }
                >
                  <option value="">Escolha…</option>
                  <option value="SURVIVOR">Do perfil preservado</option>
                  <option value="SOURCE">Do perfil absorvido</option>
                </select>
              </label>
            ) : (
              <span className="matching-muted">Mantido sem decisão</span>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

function DependencyPreview({ preview }: { preview: MatchingMergePreview }) {
  return (
    <div>
      <h4>Dependências que serão movidas</h4>
      <dl className="matching-dependencies">
        {preview.dependencies.map((dependency) => (
          <div key={dependency.kind}>
            <dt>{dependencyLabel(dependency.kind)}</dt>
            <dd>{dependency.count}</dd>
          </div>
        ))}
      </dl>
      {preview.conflicts.length > 0 ? (
        <ul className="matching-conflicts">
          {preview.conflicts.map((conflict) => (
            <li key={conflict.kind}>
              {conflictLabel(conflict.kind)}: {conflict.count}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function AnalysisStatus({ value }: { value: MatchingAnalysis["state"] }) {
  const tone =
    value === "COMPLETED"
      ? "success"
      : value === "FAILED"
        ? "danger"
        : value === "CANCELLED"
          ? "warning"
          : "info";
  return <Tag color={tone}>{analysisStateLabel(value)}</Tag>;
}

function analysisActive(value: MatchingAnalysis["state"]): boolean {
  return value === "QUEUED" || value === "RUNNING";
}

function analysisStateLabel(value: MatchingAnalysis["state"]): string {
  return {
    QUEUED: "Na fila",
    RUNNING: "Em execução",
    COMPLETED: "Concluída",
    FAILED: "Falhou",
    CANCELLED: "Cancelada",
  }[value];
}

function caseStateLabel(value: MatchingCaseState): string {
  return {
    PENDING: "Pendente",
    NOT_DUPLICATE: "Não duplicado",
    MERGED: "Mesclado",
    STALE: "Desatualizado",
  }[value];
}

function caseStateTone(value: MatchingCaseState): "neutral" | "success" | "warning" | "info" {
  return value === "MERGED"
    ? "success"
    : value === "PENDING"
      ? "warning"
      : value === "STALE"
        ? "info"
        : "neutral";
}

function scoreBandLabel(value: MatchingScoreBand): string {
  return value === "HIGH" ? "Alta" : value === "MEDIUM" ? "Média" : "Baixa";
}

function scoreBandTone(value: MatchingScoreBand): "neutral" | "success" | "warning" {
  return value === "HIGH" ? "success" : value === "MEDIUM" ? "warning" : "neutral";
}

function dependencyLabel(value: string): string {
  const labels: Record<string, string> = {
    DOCUMENT_OWNER: "Documentos próprios",
    DOCUMENT_HOLDER: "Documentos em uso",
    BILL_OWNER: "Contas próprias",
    BILL_HOLDER: "Contas em uso",
    CUSTOM_ENTITY_OWNER: "Registros personalizados",
    CUSTOM_PROFILE_VALUE: "Valores personalizados",
    ATTACHMENT_INTENT: "Uploads pendentes",
    ATTACHMENT: "Anexos",
  };
  return labels[value] ?? value;
}

function conflictLabel(value: string): string {
  return value === "DOCUMENT_UNIQUENESS"
    ? "Documentos com unicidade incompatível"
    : value === "CUSTOM_ENTITY_CARDINALITY"
      ? "Registros personalizados de cardinalidade única"
      : value;
}

function displayProfileValue(value: MatchingProfile[keyof MatchingProfile]): string {
  return typeof value === "string" && value.trim() ? value : "—";
}

function newMatchingIdempotencyKey(scope: "analysis" | "merge"): string {
  return `matching:${scope}:${crypto.randomUUID()}`;
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(
    new Date(value),
  );
}

function formatDuration(seconds: number): string {
  return seconds % 3_600 === 0
    ? `${seconds / 3_600} hora(s)`
    : `${Math.ceil(seconds / 60)} minuto(s)`;
}

function matchingError(error: unknown): string {
  if (error instanceof APIRequestError) {
    const request = error.requestId ? ` Referência: ${error.requestId}.` : "";
    if (error.code === "rate_limited")
      return `O limite de análises foi atingido. Aguarde antes de tentar novamente.${request}`;
    if (error.code === "matching_stale_preview")
      return `Os perfis ou dependências mudaram. Gere uma nova prévia.${request}`;
    if (error.code === "matching_dependency_conflict")
      return `Há dependências incompatíveis que bloqueiam a mesclagem.${request}`;
    return `${error.message}.${request}`;
  }
  return "Ocorreu um erro inesperado. Tente novamente.";
}
