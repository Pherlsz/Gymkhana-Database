import { Alert, Button, Inline, Page, Stack, StatusBadge, Surface } from "@pherlsz/gymkhana-ui";
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
  const oauthResult = new URLSearchParams(window.location.search).get("google_forms");

  if (!canManage) {
    return (
      <Page.Root maxWidth="lg">
        <Page.Header>
          <Page.Eyebrow>M9 · Integrações</Page.Eyebrow>
          <Page.Title>Google Forms</Page.Title>
        </Page.Header>
        <Page.Content>
          <Alert title="Acesso administrativo necessário" tone="warning">
            Somente administradores podem conectar contas e configurar fontes do Google Forms.
          </Alert>
        </Page.Content>
      </Page.Root>
    );
  }

  return (
    <Page.Root maxWidth="lg">
      <Page.Header>
        <Page.Eyebrow>M9 · Integrações</Page.Eyebrow>
        <Page.Title>Google Forms</Page.Title>
        <Page.Description>
          Conecte somente leitura, mapeie perguntas para campos lógicos e revise cada lote no fluxo
          de Operações antes de gravá-lo.
        </Page.Description>
      </Page.Header>
      <Page.Content>
        <Stack gap="6">
          {oauthResult === "connected" ? (
            <Alert title="Google Forms conectado" tone="success">
              A autorização foi armazenada de forma criptografada.
            </Alert>
          ) : oauthResult === "denied" ? (
            <Alert title="Autorização não concedida" tone="warning">
              Nenhuma conexão foi criada. Você pode tentar novamente quando quiser.
            </Alert>
          ) : null}
          {status.isError ? (
            <Alert title="Não foi possível consultar a integração" tone="danger">
              {googleFormsError(status.error)}
            </Alert>
          ) : null}
          {status.data && !status.data.enabled ? (
            <Alert title="Integração desativada" tone="info">
              As credenciais e a chave de criptografia ainda não foram habilitadas neste ambiente.
            </Alert>
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
          {status.data?.connected && catalog.data ? (
            <SourceCreator
              modules={catalog.data.modules.filter((value) => value.can_import)}
              onCreated={() => void refresh()}
            />
          ) : null}
          {status.data?.connected ? (
            <Page.Section
              description="Cada fonte pertence ao administrador conectado; alterações de tipo ou obrigatoriedade pausam a sincronização."
              title="Fontes configuradas"
            >
              {sources.isError ? (
                <Alert title="Não foi possível carregar as fontes" tone="danger">
                  {googleFormsError(sources.error)}
                </Alert>
              ) : null}
              <Stack gap="4">
                {(sources.data?.sources ?? []).map((source) => (
                  <SourceCard
                    catalog={catalog.data?.modules.find((value) => value.id === source.module)}
                    key={source.id}
                    onUpdated={() => void refresh()}
                    source={source}
                  />
                ))}
                {!sources.isLoading && sources.data?.sources.length === 0 ? (
                  <Alert title="Nenhuma fonte configurada">
                    Informe um formulário para carregar o esquema de perguntas.
                  </Alert>
                ) : null}
              </Stack>
            </Page.Section>
          ) : null}
          {status.data?.connected ? (
            <SyncHistory
              error={syncs.error}
              loading={syncs.isLoading}
              onUpdated={() => void refresh()}
              sources={sources.data?.sources ?? []}
              values={syncs.data?.runs ?? []}
            />
          ) : null}
        </Stack>
      </Page.Content>
    </Page.Root>
  );
}

function ConnectionSetup({ onConnected }: { onConnected: () => void }) {
  const mutation = useMutation({
    mutationFn: beginGoogleFormsOAuth,
    onSuccess: onConnected,
  });
  return (
    <Surface className="google-forms-panel" tone="raised">
      <Stack gap="4">
        <div>
          <h2>Conectar conta Google</h2>
          <p className="google-forms-muted">
            Serão solicitados apenas os escopos de leitura do formulário e das respostas. O acesso
            pode ser revogado a qualquer momento.
          </p>
        </div>
        {mutation.isError ? (
          <Alert title="Não foi possível iniciar a autorização" tone="danger">
            {googleFormsError(mutation.error)}
          </Alert>
        ) : null}
        <Inline>
          <Button disabled={mutation.isPending} onClick={() => mutation.mutate()}>
            {mutation.isPending ? "Preparando…" : "Conectar com Google"}
          </Button>
        </Inline>
      </Stack>
    </Surface>
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
    <Surface className="google-forms-panel" tone="raised">
      <Inline align="center" className="google-forms-heading">
        <div>
          <h2>Conexão Google</h2>
          <p className="google-forms-muted">
            Atualizada em {formatDate(connection.updated_at)} · somente leitura
          </p>
        </div>
        <Inline>
          <FormsStatus value={connection.state} />
          <Button
            disabled={mutation.isPending}
            onClick={() => {
              if (window.confirm("Revogar a conexão e pausar todas as fontes?")) mutation.mutate();
            }}
          >
            {mutation.isPending ? "Revogando…" : "Desconectar"}
          </Button>
        </Inline>
      </Inline>
      {mutation.isError ? (
        <Alert title="Não foi possível revogar a conexão" tone="danger">
          {googleFormsError(mutation.error)}
        </Alert>
      ) : null}
    </Surface>
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
    <Surface className="google-forms-panel" tone="raised">
      <form
        onSubmit={(event: FormEvent) => {
          event.preventDefault();
          mutation.mutate();
        }}
      >
        <Stack gap="4">
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
            <Alert title="Não foi possível adicionar o formulário" tone="danger">
              {googleFormsError(mutation.error)}
            </Alert>
          ) : null}
          <Inline>
            <Button disabled={!reference.trim() || mutation.isPending} type="submit">
              {mutation.isPending ? "Carregando esquema…" : "Adicionar formulário"}
            </Button>
          </Inline>
        </Stack>
      </form>
    </Surface>
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
    <Surface className="google-forms-source" tone="raised">
      <Stack gap="4">
        <Inline align="center" className="google-forms-heading">
          <div>
            <h3>{source.title}</h3>
            <p className="google-forms-muted">
              {catalog?.label ?? source.module} · revisão {source.schema_revision}
            </p>
          </div>
          <FormsStatus value={source.state} />
        </Inline>
        {source.state === "SCHEMA_DRIFT" ? (
          <Alert title="O esquema do formulário mudou" tone="warning">
            Revise as perguntas e salve novamente o mapeamento antes de reativar.
          </Alert>
        ) : null}
        {source.state === "NEEDS_REAUTH" ? (
          <Alert title="A conexão precisa ser refeita" tone="warning">
            Reconecte a conta Google e reative esta fonte.
          </Alert>
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
          <Alert title="Não foi possível atualizar a fonte" tone="danger">
            {googleFormsError(mutationError)}
          </Alert>
        ) : null}
        <Inline>
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
        </Inline>
        <p className="google-forms-muted">
          Última sincronização:{" "}
          {source.last_synced_at ? formatDate(source.last_synced_at) : "nunca"}
          {source.pagination_pending ? " · há mais páginas agendadas" : ""}
        </p>
      </Stack>
    </Surface>
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
    <Page.Section
      description="A sincronização apenas prepara uma importação; a execução final continua no módulo Operações."
      title="Histórico de sincronizações"
    >
      {error ? (
        <Alert title="Não foi possível carregar o histórico" tone="danger">
          {googleFormsError(error)}
        </Alert>
      ) : null}
      {loading ? <Alert title="Carregando sincronizações">Aguarde…</Alert> : null}
      <Stack gap="3">
        {values.map((value) => (
          <SyncRow
            key={value.id}
            name={names.get(value.source_id) ?? "Formulário"}
            onUpdated={onUpdated}
            value={value}
          />
        ))}
        {!loading && values.length === 0 ? (
          <Alert title="Nenhuma sincronização solicitada">Ative uma fonte para começar.</Alert>
        ) : null}
      </Stack>
    </Page.Section>
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
    <Surface className="google-forms-sync" tone="raised">
      <Inline align="center" className="google-forms-heading">
        <div>
          <strong>{name}</strong>
          <p className="google-forms-muted">
            {formatDate(value.created_at)} · {value.staged_count} novas · {value.duplicate_count} já
            recebidas
          </p>
        </div>
        <Inline>
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
        </Inline>
      </Inline>
      {value.error_code ? (
        <span className="google-forms-muted">Código: {value.error_code}</span>
      ) : null}
    </Surface>
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
  return <StatusBadge tone={tone}>{stateLabel(value)}</StatusBadge>;
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
