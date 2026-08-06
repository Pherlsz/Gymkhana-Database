import { Alert, Button, Card, Flex, Layout, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useApplicationSession } from "./App";
import { APIRequestError } from "./lib/api/client";
import {
  beginGoogleFormsOAuth,
  cancelGoogleFormsSync,
  createGoogleFormsSource,
  disconnectGoogleForms,
  getGoogleFormsStatus,
  listGoogleFormsSources,
  listGoogleFormsSyncs,
  refreshGoogleFormsSource,
  requestGoogleFormsSync,
  saveGoogleFormsMapping,
  updateGoogleFormsSource,
  type GoogleFormsMapping,
  type GoogleFormsModule,
  type GoogleFormsSource,
  type GoogleFormsSync,
} from "./lib/api/googleForms";
import { getOperationsCatalog } from "./lib/api/operations";

export function GoogleFormsPage() {
  const session = useApplicationSession();
  const queryClient = useQueryClient();
  const initialSearch = new URLSearchParams(window.location.search);
  const [tab, setTab] = useState<"sources" | "history">(
    initialSearch.get("tab") === "history" ? "history" : "sources",
  );
  const [selectedSourceID, setSelectedSourceID] = useState(initialSearch.get("source") ?? "");
  const canManage = session.user.role === "ADMIN" || session.user.role === "SUPERADMIN";
  const status = useQuery({
    queryKey: ["google-forms-status"],
    queryFn: ({ signal }) => getGoogleFormsStatus(signal),
    enabled: canManage,
  });
  const sources = useQuery({
    queryKey: ["google-forms-sources"],
    queryFn: ({ signal }) => listGoogleFormsSources(signal),
    enabled: canManage && status.data?.connected === true,
  });
  const syncs = useQuery({
    queryKey: ["google-forms-syncs"],
    queryFn: ({ signal }) => listGoogleFormsSyncs(signal),
    enabled: canManage && status.data?.connected === true,
    refetchInterval: (query) =>
      query.state.data?.runs.some((value) => value.state === "QUEUED" || value.state === "RUNNING")
        ? 2_000
        : false,
  });
  const catalog = useQuery({
    queryKey: ["operations-catalog"],
    queryFn: ({ signal }) => getOperationsCatalog(signal),
    enabled: canManage && status.data?.connected === true,
    staleTime: 60_000,
  });
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["google-forms-status"] }),
      queryClient.invalidateQueries({ queryKey: ["google-forms-sources"] }),
      queryClient.invalidateQueries({ queryKey: ["google-forms-syncs"] }),
      queryClient.invalidateQueries({ queryKey: ["operation-imports"] }),
    ]);
  };
  const oauthResult = initialSearch.get("google_forms");
  const selectedSource = sources.data?.sources.find((value) => value.id === selectedSourceID);
  const visibleSources = selectedSourceID
    ? selectedSource
      ? [selectedSource]
      : []
    : (sources.data?.sources ?? []);
  const navigate = (nextTab: "sources" | "history", nextSource = selectedSourceID) => {
    setTab(nextTab);
    setSelectedSourceID(nextSource);
    replaceGoogleFormsSearch(nextTab, nextSource);
  };

  if (!canManage) {
    return (
      <Layout style={{ maxWidth: "lg", margin: "0 auto" }}>
        <header className="page-header">
          <div className="page-eyebrow">M9 · Integrações</div>
          <Typography.Title level={1} className="page-title">Google Forms</Typography.Title>
        </header>
        <div className="page-content">
          <Alert message="Acesso administrativo necessário" type="warning" description="Somente administradores podem conectar contas e configurar fontes do Google Forms." />
        </div>
      </Layout>
    );
  }

  return (
    <Layout style={{ maxWidth: "lg", margin: "0 auto" }}>
      <header className="page-header">
        <div className="page-eyebrow">M9 · Integrações</div>
        <Typography.Title level={1} className="page-title">Google Forms</Typography.Title>
        <Typography.Paragraph className="page-description">
          Conecte somente leitura, mapeie perguntas para campos lógicos e revise cada lote no fluxo
          de Operações antes de gravá-lo.
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex vertical gap="1.5rem">
          {oauthResult === "connected" ? (
            <Alert message="Google Forms conectado" type="success" description="A autorização foi armazenada de forma criptografada." />
          ) : oauthResult === "denied" ? (
            <Alert message="Autorização não concedida" type="warning" description="Nenhuma conexão foi criada. Você pode tentar novamente quando quiser." />
          ) : null}
          {status.isError ? (
            <Alert message="Não foi possível consultar a integração" type="error" description={<>{googleFormsError(status.error)}
            </>} />
          ) : null}
          {status.data && !status.data.enabled ? (
            <Alert message="Integração desativada" type="info" description="As credenciais e a chave de criptografia ainda não foram habilitadas neste ambiente." />
          ) : null}
          {status.data?.enabled && !status.data.connected ? (
            <ConnectionSetup onConnected={() => void refresh()} />
          ) : null}
          {status.data?.connected && status.data.connection ? (
            <ConnectionSummary
              connection={status.data.connection}
              onDisconnected={() => void refresh()}
            />
          ) : null}
          {status.data?.connected ? (
            <Flex aria-label="Seções do Google Forms" role="tablist">
              <Button
                aria-selected={tab === "sources"}
                onClick={() => navigate("sources")}
                role="tab"
              >
                Fontes
              </Button>
              <Button
                aria-selected={tab === "history"}
                onClick={() => navigate("history")}
                role="tab"
              >
                Histórico
              </Button>
            </Flex>
          ) : null}
          {status.data?.connected && tab === "sources" && catalog.data ? (
            <SourceCreator
              modules={catalog.data.modules.filter((value) => value.can_import)}
              onCreated={() => void refresh()}
            />
          ) : null}
          {status.data?.connected && tab === "sources" ? (
            <section className="page-section"
            >
      <Typography.Title level={2}>Fontes configuradas</Typography.Title>
      <Typography.Paragraph>Cada fonte pertence ao administrador conectado; alterações de tipo ou obrigatoriedade pausam a sincronização.</Typography.Paragraph>
              {sources.isError ? (
                <Alert message="Não foi possível carregar as fontes" type="error" description={<>{googleFormsError(sources.error)}
                </>} />
              ) : null}
              {(sources.data?.sources.length ?? 0) > 1 ? (
                <label>
                  Fonte selecionada
                  <select
                    aria-label="Fonte selecionada"
                    value={selectedSourceID}
                    onChange={(event) => navigate("sources", event.target.value)}
                  >
                    <option value="">Todas as fontes</option>
                    {sources.data?.sources.map((source) => (
                      <option key={source.id} value={source.id}>
                        {source.title}
                      </option>
                    ))}
                  </select>
                </label>
              ) : null}
              {selectedSourceID && sources.data && !selectedSource ? (
                <Alert message="Fonte não encontrada" type="warning" description="A fonte informada na URL não pertence a esta conexão ou não está mais disponível." />
              ) : null}
              <Flex vertical gap="1rem">
                {visibleSources.map((source) => (
                  <SourceCard
                    catalog={catalog.data?.modules.find((value) => value.id === source.module)}
                    key={source.id}
                    onUpdated={() => void refresh()}
                    source={source}
                  />
                ))}
                {!sources.isLoading && sources.data?.sources.length === 0 ? (
                  <Alert message="Nenhuma fonte configurada" type="info" description="Informe um formulário para carregar o esquema de perguntas." />
                ) : null}
              </Flex>
            </section>
          ) : null}
          {status.data?.connected && tab === "history" ? (
            <SyncHistory
              error={syncs.error}
              loading={syncs.isLoading}
              onUpdated={() => void refresh()}
              sources={sources.data?.sources ?? []}
              values={syncs.data?.runs ?? []}
            />
          ) : null}
        </Flex>
      </div>
    </Layout>
  );
}

function ConnectionSetup({ onConnected }: { onConnected: () => void }) {
  const mutation = useMutation({
    mutationFn: () => beginGoogleFormsOAuth(googleFormsReturnPath()),
    onSuccess: onConnected,
  });
  return (
    <Card className="google-forms-panel" style={{ padding: "1rem" }}>
      <Flex vertical gap="1rem">
        <div>
          <h2>Conectar conta Google</h2>
          <p className="google-forms-muted">
            Serão solicitados apenas os escopos de leitura do formulário e das respostas. O acesso
            pode ser revogado a qualquer momento.
          </p>
        </div>
        {mutation.isError ? (
          <Alert message="Não foi possível iniciar a autorização" type="error" description={<>{googleFormsError(mutation.error)}
          </>} />
        ) : null}
        <Flex>
          <Button disabled={mutation.isPending} onClick={() => mutation.mutate()}>
            {mutation.isPending ? "Preparando…" : "Conectar com Google"}
          </Button>
        </Flex>
      </Flex>
    </Card>
  );
}

function ConnectionSummary({
  connection,
  onDisconnected,
}: {
  connection: NonNullable<Awaited<ReturnType<typeof getGoogleFormsStatus>>["connection"]>;
  onDisconnected: () => void;
}) {
  const mutation = useMutation({
    mutationFn: () => disconnectGoogleForms(connection.version),
    onSuccess: onDisconnected,
  });
  return (
    <Card className="google-forms-panel" style={{ padding: "1rem" }}>
      <Flex align="center" className="google-forms-heading">
        <div>
          <h2>Conexão Google</h2>
          <p className="google-forms-muted">
            Atualizada em {formatDate(connection.updated_at)} · somente leitura
          </p>
        </div>
        <Flex>
          <FormsStatus value={connection.state} />
          <Button
            disabled={mutation.isPending}
            onClick={() => {
              if (window.confirm("Revogar a conexão e pausar todas as fontes?")) mutation.mutate();
            }}
          >
            {mutation.isPending ? "Revogando…" : "Desconectar"}
          </Button>
        </Flex>
      </Flex>
      {mutation.isError ? (
        <Alert message="Não foi possível revogar a conexão" type="error" description={<>{googleFormsError(mutation.error)}
        </>} />
      ) : null}
    </Card>
  );
}

function SourceCreator({
  modules,
  onCreated,
}: {
  modules: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"];
  onCreated: () => void;
}) {
  const [reference, setReference] = useState("");
  const [module, setModule] = useState<GoogleFormsModule>(modules[0]?.id ?? "PROFILES");
  const mutation = useMutation({
    mutationFn: () => createGoogleFormsSource(reference, module),
    onSuccess: () => {
      setReference("");
      onCreated();
    },
  });
  return (
    <Card className="google-forms-panel" style={{ padding: "1rem" }}>
      <form
        onSubmit={(event: FormEvent) => {
          event.preventDefault();
          mutation.mutate();
        }}
      >
        <Flex vertical gap="1rem">
          <div>
            <h2>Adicionar formulário</h2>
            <p className="google-forms-muted">
              Cole o ID ou a URL de edição. O formulário precisa estar acessível pela conta
              conectada.
            </p>
          </div>
          <div className="google-forms-grid">
            <label>
              ID ou URL do formulário
              <input
                required
                value={reference}
                onChange={(event) => setReference(event.target.value)}
              />
            </label>
            <label>
              Módulo de destino
              <select
                value={module}
                onChange={(event) => setModule(event.target.value as GoogleFormsModule)}
              >
                {modules.map((value) => (
                  <option key={value.id} value={value.id}>
                    {value.label}
                  </option>
                ))}
              </select>
            </label>
          </div>
          {mutation.isError ? (
            <Alert message="Não foi possível adicionar o formulário" type="error" description={<>{googleFormsError(mutation.error)}
            </>} />
          ) : null}
          <Flex>
            <Button disabled={!reference.trim() || mutation.isPending} htmlType="submit">
              {mutation.isPending ? "Carregando esquema…" : "Adicionar formulário"}
            </Button>
          </Flex>
        </Flex>
      </form>
    </Card>
  );
}

function SourceCard({
  source,
  catalog,
  onUpdated,
}: {
  source: GoogleFormsSource;
  catalog: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"][number] | undefined;
  onUpdated: () => void;
}) {
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [syncMode, setSyncMode] = useState<"MANUAL" | "POLL">(source.sync_mode);
  const [interval, setIntervalValue] = useState(source.poll_interval_seconds);
  useEffect(() => {
    setMapping(
      Object.fromEntries(
        source.questions
          .filter((question) => question.target_field)
          .map((question) => [question.id, question.target_field!]),
      ),
    );
    setSyncMode(source.sync_mode);
    setIntervalValue(source.poll_interval_seconds);
  }, [source]);
  const fields = useMemo(
    () => catalog?.fields.filter((value) => value.importable) ?? [],
    [catalog],
  );
  const saveMapping = useMutation({
    mutationFn: () => {
      const values: GoogleFormsMapping = Object.entries(mapping)
        .filter(([, target]) => target)
        .map(([question_id, target_field]) => ({ question_id, target_field }));
      return saveGoogleFormsMapping(source, values);
    },
    onSuccess: onUpdated,
  });
  const updateState = useMutation({
    mutationFn: (enabled: boolean) =>
      updateGoogleFormsSource(source, {
        sync_mode: syncMode,
        poll_interval_seconds: interval,
        enabled,
      }),
    onSuccess: onUpdated,
  });
  const refresh = useMutation({
    mutationFn: () => refreshGoogleFormsSource(source),
    onSuccess: onUpdated,
  });
  const sync = useMutation({
    mutationFn: () => requestGoogleFormsSync(source),
    onSuccess: onUpdated,
  });
  const mutationError = saveMapping.error ?? updateState.error ?? refresh.error ?? sync.error;
  const busy =
    saveMapping.isPending || updateState.isPending || refresh.isPending || sync.isPending;
  return (
    <Card className="google-forms-source" style={{ padding: "1rem" }}>
      <Flex vertical gap="1rem">
        <Flex align="center" className="google-forms-heading">
          <div>
            <h3>{source.title}</h3>
            <p className="google-forms-muted">
              {catalog?.label ?? source.module} · revisão {source.schema_revision}
            </p>
          </div>
          <FormsStatus value={source.state} />
        </Flex>
        {source.state === "SCHEMA_DRIFT" ? (
          <Alert message="O esquema do formulário mudou" type="warning" description="Revise as perguntas e salve novamente o mapeamento antes de reativar." />
        ) : null}
        {source.state === "NEEDS_REAUTH" ? (
          <Alert message="A conexão precisa ser refeita" type="warning" description="Reconecte a conta Google e reative esta fonte." />
        ) : null}
        <div className="google-forms-mapping">
          {source.questions.map((question) => (
            <label key={question.id}>
              <span>
                {question.title}
                {question.required ? " *" : ""}
              </span>
              {question.supported ? (
                <select
                  aria-label={`Destino para ${question.title}`}
                  disabled={busy}
                  value={mapping[question.id] ?? ""}
                  onChange={(event) =>
                    setMapping((current) => ({ ...current, [question.id]: event.target.value }))
                  }
                >
                  <option value="">Não importar</option>
                  {fields.map((field) => (
                    <option
                      disabled={Object.entries(mapping).some(
                        ([questionID, target]) => questionID !== question.id && target === field.id,
                      )}
                      key={field.id}
                      value={field.id}
                    >
                      {field.label}
                      {field.required ? " (obrigatório)" : ""}
                    </option>
                  ))}
                </select>
              ) : (
                <span className="google-forms-unsupported">
                  Não suportada: {sourceErrorLabel(question.unsupported_code)}
                </span>
              )}
            </label>
          ))}
        </div>
        <div className="google-forms-grid">
          <label>
            Sincronização
            <select
              disabled={busy}
              value={syncMode}
              onChange={(event) => setSyncMode(event.target.value as "MANUAL" | "POLL")}
            >
              <option value="MANUAL">Manual</option>
              <option value="POLL">Automática</option>
            </select>
          </label>
          <label>
            Intervalo
            <select
              disabled={busy || syncMode !== "POLL"}
              value={interval}
              onChange={(event) => setIntervalValue(Number(event.target.value))}
            >
              <option value={300}>5 minutos</option>
              <option value={900}>15 minutos</option>
              <option value={3600}>1 hora</option>
              <option value={21600}>6 horas</option>
              <option value={86400}>24 horas</option>
            </select>
          </label>
        </div>
        {mutationError ? (
          <Alert message="Não foi possível atualizar a fonte" type="error" description={<>{googleFormsError(mutationError)}
          </>} />
        ) : null}
        <Flex>
          <Button disabled={busy} onClick={() => saveMapping.mutate()}>
            Salvar mapeamento
          </Button>
          <Button disabled={busy} onClick={() => refresh.mutate()}>
            Atualizar esquema
          </Button>
          {source.state === "ACTIVE" ? (
            <>
              <Button disabled={busy} onClick={() => sync.mutate()}>
                Sincronizar agora
              </Button>
              <Button disabled={busy} onClick={() => updateState.mutate(false)}>
                Pausar
              </Button>
            </>
          ) : (
            <Button
              disabled={busy || source.state === "NEEDS_REAUTH" || source.state === "SCHEMA_DRIFT"}
              onClick={() => updateState.mutate(true)}
            >
              Ativar
            </Button>
          )}
        </Flex>
        <p className="google-forms-muted">
          Última sincronização:{" "}
          {source.last_synced_at ? formatDate(source.last_synced_at) : "nunca"}
          {source.pagination_pending ? " · há mais páginas agendadas" : ""}
        </p>
      </Flex>
    </Card>
  );
}

function SyncHistory({
  values,
  sources,
  loading,
  error,
  onUpdated,
}: {
  values: GoogleFormsSync[];
  sources: GoogleFormsSource[];
  loading: boolean;
  error: Error | null;
  onUpdated: () => void;
}) {
  const names = new Map(sources.map((value) => [value.id, value.title]));
  return (
    <section className="page-section"
    >
      <Typography.Title level={2}>Histórico de sincronizações</Typography.Title>
      <Typography.Paragraph>A sincronização apenas prepara uma importação; a execução final continua no módulo Operações.</Typography.Paragraph>
      {error ? (
        <Alert message="Não foi possível carregar o histórico" type="error" description={<>{googleFormsError(error)}
        </>} />
      ) : null}
      {loading ? <Alert message="Carregando sincronizações" type="info" description="Aguarde…" /> : null}
      <Flex vertical gap="0.75rem">
        {values.map((value) => (
          <SyncRow
            key={value.id}
            name={names.get(value.source_id) ?? "Formulário"}
            onUpdated={onUpdated}
            value={value}
          />
        ))}
        {!loading && values.length === 0 ? (
          <Alert message="Nenhuma sincronização solicitada" type="info" description="Ative uma fonte para começar." />
        ) : null}
      </Flex>
    </section>
  );
}

function SyncRow({
  value,
  name,
  onUpdated,
}: {
  value: GoogleFormsSync;
  name: string;
  onUpdated: () => void;
}) {
  const mutation = useMutation({
    mutationFn: () => cancelGoogleFormsSync(value),
    onSuccess: onUpdated,
  });
  return (
    <Card className="google-forms-sync" style={{ padding: "1rem" }}>
      <Flex align="center" className="google-forms-heading">
        <div>
          <strong>{name}</strong>
          <p className="google-forms-muted">
            {formatDate(value.created_at)} · {value.staged_count} novas · {value.duplicate_count} já
            recebidas
          </p>
        </div>
        <Flex>
          <FormsStatus value={value.state} />
          {value.operation_import_id ? (
            <Link
              className="app-nav__link"
              search={{ selected: value.operation_import_id }}
              to="/operations"
            >
              Abrir importação
            </Link>
          ) : null}
          {value.state === "QUEUED" || value.state === "RUNNING" ? (
            <Button disabled={mutation.isPending} onClick={() => mutation.mutate()}>
              Cancelar
            </Button>
          ) : null}
        </Flex>
      </Flex>
      {value.error_code ? (
        <span className="google-forms-muted">Código: {value.error_code}</span>
      ) : null}
    </Card>
  );
}

function FormsStatus({ value }: { value: string }) {
  const tone =
    value === "ACTIVE" || value === "COMPLETED"
      ? "success"
      : value === "FAILED" || value === "ERROR" || value === "NEEDS_REAUTH"
        ? "danger"
        : value === "SCHEMA_DRIFT" || value === "PAUSED" || value === "CANCELLED"
          ? "warning"
          : "info";
  return <Tag color={tone}>{stateLabel(value)}</Tag>;
}

function stateLabel(value: string) {
  const labels: Record<string, string> = {
    ACTIVE: "Ativa",
    DRAFT: "Rascunho",
    PAUSED: "Pausada",
    SCHEMA_DRIFT: "Esquema alterado",
    NEEDS_REAUTH: "Reconectar",
    ERROR: "Erro",
    DISCONNECTED: "Desconectada",
    QUEUED: "Na fila",
    RUNNING: "Sincronizando",
    COMPLETED: "Concluída",
    FAILED: "Falhou",
    CANCELLED: "Cancelada",
  };
  return labels[value] ?? value;
}

function sourceErrorLabel(value?: string) {
  if (value === "file_upload") return "upload de arquivo";
  if (value === "multiple_answers") return "múltiplas respostas";
  if (value === "question_group") return "grade de perguntas";
  return "tipo ainda não compatível";
}

function googleFormsError(error: unknown) {
  if (error instanceof APIRequestError) {
    return error.requestId ? `${error.message} (requisição ${error.requestId})` : error.message;
  }
  return error instanceof Error ? error.message : "Tente novamente.";
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(
    new Date(value),
  );
}

function replaceGoogleFormsSearch(tab: "sources" | "history", sourceID: string) {
  const target = new URL(window.location.href);
  target.searchParams.delete("google_forms");
  target.searchParams.set("tab", tab);
  if (sourceID) target.searchParams.set("source", sourceID);
  else target.searchParams.delete("source");
  window.history.replaceState({}, "", `${target.pathname}${target.search}${target.hash}`);
}

function googleFormsReturnPath() {
  const current = new URL(window.location.href);
  const parameters = new URLSearchParams();
  const tab = current.searchParams.get("tab");
  const source = current.searchParams.get("source");
  if (tab === "sources" || tab === "history") parameters.set("tab", tab);
  if (source && /^[0-9a-f-]{36}$/i.test(source)) parameters.set("source", source);
  const query = parameters.toString();
  return query ? `/google-forms?${query}` : "/google-forms";
}
