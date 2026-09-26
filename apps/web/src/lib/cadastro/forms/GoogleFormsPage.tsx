import { Button, Card, Flex, Tag, Typography } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Link2Off } from "lucide-react";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { InlineStatus } from "../../../components/InlineStatus";
import { StateCard } from "../../../components/StateCard";
import { StatusBanner } from "../../../components/StatusBanner";
import "../../../google-forms.css";
import { useApplicationSession } from "../../../session";
import { useI18n } from "../../../i18n";
import { formatDateTime as formatDate } from "../../formatters";
import { googleFormsReturnPath, tableFromModule } from "../cadastroSearch";
import { CadastroWorkQuery, CadastroWorkState, confirmCadastroAction } from "../CadastroWork";
import { queryKeys } from "../../api/queryKeys";
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
} from "../../api/googleForms";
import { getOperationsCatalog } from "../../api/operations";

export function GoogleFormsPage({
  defaultModule,
}: {
  defaultModule?: GoogleFormsModule | undefined;
} = {}) {
  const { messages } = useI18n();
  const cadastroCopy = messages.tables.cadastro;
  const forms = messages.googleForms;
  const session = useApplicationSession();
  const queryClient = useQueryClient();
  const initialSearch = new URLSearchParams(window.location.search);
  const [tab, setTab] = useState<"sources" | "history">(
    initialSearch.get("tab") === "history" ? "history" : "sources",
  );
  const [selectedSourceID, setSelectedSourceID] = useState(initialSearch.get("source") ?? "");
  const canManage = session.user.role === "ADMIN" || session.user.role === "SUPERADMIN";
  const status = useQuery({
    queryKey: queryKeys.googleForms.status,
    queryFn: ({ signal }) => getGoogleFormsStatus(signal),
    enabled: canManage,
  });
  const sources = useQuery({
    queryKey: queryKeys.googleForms.sources,
    queryFn: ({ signal }) => listGoogleFormsSources(signal),
    enabled: canManage && status.data?.connected === true,
  });
  const syncs = useQuery({
    queryKey: queryKeys.googleForms.syncs,
    queryFn: ({ signal }) => listGoogleFormsSyncs(signal),
    enabled: canManage && status.data?.connected === true,
    refetchInterval: (query) =>
      query.state.data?.runs.some((value) => value.state === "QUEUED" || value.state === "RUNNING")
        ? 2_000
        : false,
  });
  const catalog = useQuery({
    queryKey: queryKeys.operations.catalog,
    queryFn: ({ signal }) => getOperationsCatalog(signal),
    enabled: canManage && status.data?.connected === true,
    staleTime: 60_000,
  });
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.googleForms.status }),
      queryClient.invalidateQueries({ queryKey: queryKeys.googleForms.sources }),
      queryClient.invalidateQueries({ queryKey: queryKeys.googleForms.syncs }),
      queryClient.invalidateQueries({ queryKey: queryKeys.operations.importsList }),
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
      <CadastroWorkState
        description={forms.adminRequiredDescription}
        kind="warning"
        title={forms.adminRequiredMessage}
      />
    );
  }

  if (status.isPending || status.isError) {
    return (
      <CadastroWorkQuery
        error={status.error}
        errorTitle={forms.statusErrorMessage}
        isError={status.isError}
        isPending={status.isPending}
        onRetry={() => void refresh()}
      />
    );
  }

  if (status.data && !status.data.enabled) {
    return (
      <CadastroWorkState
        description={cadastroCopy.formsIntegrationDisabledDesc}
        icon={<Link2Off aria-hidden size={28} strokeWidth={1.75} />}
        kind="warning"
        title={cadastroCopy.formsIntegrationDisabledTitle}
      />
    );
  }

  return (
    <Flex vertical gap="1.5rem">
      {oauthResult === "connected" ? (
        <StatusBanner
          description={forms.connectedDescription}
          title={forms.connectedMessage}
          tone="success"
        />
      ) : oauthResult === "denied" ? (
        <StatusBanner
          description={forms.deniedDescription}
          title={forms.deniedMessage}
          tone="warning"
        />
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
        <Flex aria-label={forms.sectionsAria} role="tablist">
          <Button aria-selected={tab === "sources"} onClick={() => navigate("sources")} role="tab">
            {forms.tabSources}
          </Button>
          <Button aria-selected={tab === "history"} onClick={() => navigate("history")} role="tab">
            {forms.tabHistory}
          </Button>
        </Flex>
      ) : null}
      {status.data?.connected && tab === "sources" && catalog.data ? (
        <SourceCreator
          defaultModule={defaultModule}
          modules={catalog.data.modules.filter((value) => value.can_import)}
          onCreated={() => void refresh()}
        />
      ) : null}
      {status.data?.connected && tab === "sources" ? (
        <section className="page-section">
          <Typography.Title level={2}>{forms.configuredSourcesTitle}</Typography.Title>
          <Typography.Paragraph>{forms.configuredSourcesDesc}</Typography.Paragraph>
          {sources.isError ? (
            <StatusBanner error={sources.error} title={forms.loadSourcesError} />
          ) : null}
          {(sources.data?.sources.length ?? 0) > 1 ? (
            <label>
              {forms.sourceSelectLabel}
              <select
                aria-label={forms.sourceSelectLabel}
                value={selectedSourceID}
                onChange={(event) => navigate("sources", event.target.value)}
              >
                <option value="">{forms.allSourcesOption}</option>
                {sources.data?.sources.map((source) => (
                  <option key={source.id} value={source.id}>
                    {source.title}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          {selectedSourceID && sources.data && !selectedSource ? (
            <StatusBanner
              description={forms.sourceNotFoundDesc}
              title={forms.sourceNotFoundMessage}
              tone="warning"
            />
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
              <StateCard
                compact
                description={forms.emptySourcesDesc}
                kind="empty"
                title={forms.emptySourcesTitle}
              />
            ) : sources.isLoading ? (
              <InlineStatus kind="loading" label={messages.common.status.loading} />
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
  );
}

function ConnectionSetup({ onConnected }: { onConnected: () => void }) {
  const { messages } = useI18n();
  const forms = messages.googleForms;
  const mutation = useMutation({
    mutationFn: () => beginGoogleFormsOAuth(googleFormsReturnPath()),
    onSuccess: onConnected,
  });
  return (
    <Card className="google-forms-panel">
      <Flex vertical gap="1rem">
        <div>
          <h2>{forms.connectGoogleTitle}</h2>
          <p className="google-forms-muted">{forms.connectGoogleDesc}</p>
        </div>
        {mutation.isError ? (
          <StatusBanner error={mutation.error} title={forms.connectAuthError} />
        ) : null}
        <Flex>
          <Button disabled={mutation.isPending} onClick={() => mutation.mutate()}>
            {mutation.isPending ? forms.connectPreparing : forms.connectButton}
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
  const { messages, t } = useI18n();
  const forms = messages.googleForms;
  const mutation = useMutation({
    mutationFn: () => disconnectGoogleForms(connection.version),
    onSuccess: onDisconnected,
  });
  return (
    <Card className="google-forms-panel">
      <Flex align="center" className="google-forms-heading">
        <div>
          <h2>{forms.connectionTitle}</h2>
          <p className="google-forms-muted">
            {t(forms.connectionUpdatedOn, { date: formatDate(connection.updated_at) })}
          </p>
        </div>
        <Flex>
          <FormsStatus value={connection.state} />
          <Button
            disabled={mutation.isPending}
            onClick={() => {
              void confirmCadastroAction({
                title: forms.disconnectTitle,
                content: forms.disconnectConfirm,
                okText: forms.disconnectButton,
                cancelText: messages.common.actions.cancel,
              }).then((ok) => {
                if (ok) mutation.mutate();
              });
            }}
          >
            {mutation.isPending ? forms.disconnecting : forms.disconnectButton}
          </Button>
        </Flex>
      </Flex>
      {mutation.isError ? (
        <StatusBanner error={mutation.error} title={forms.disconnectError} />
      ) : null}
    </Card>
  );
}

function SourceCreator({
  modules,
  defaultModule,
  onCreated,
}: {
  modules: Awaited<ReturnType<typeof getOperationsCatalog>>["modules"];
  defaultModule?: GoogleFormsModule | undefined;
  onCreated: () => void;
}) {
  const { messages } = useI18n();
  const forms = messages.googleForms;
  const [reference, setReference] = useState("");
  const [module, setModule] = useState<GoogleFormsModule>(
    defaultModule ?? modules[0]?.id ?? "PROFILES",
  );
  useEffect(() => {
    if (defaultModule) setModule(defaultModule);
  }, [defaultModule]);
  const mutation = useMutation({
    mutationFn: () => createGoogleFormsSource(reference, module),
    onSuccess: () => {
      setReference("");
      onCreated();
    },
  });
  return (
    <Card className="google-forms-panel">
      <form
        onSubmit={(event: FormEvent) => {
          event.preventDefault();
          mutation.mutate();
        }}
      >
        <Flex vertical gap="1rem">
          <div>
            <h2>{forms.addFormTitle}</h2>
            <p className="google-forms-muted">{forms.addFormDesc}</p>
          </div>
          <div className="google-forms-grid">
            <label>
              {forms.formRefLabel}
              <input
                required
                value={reference}
                onChange={(event) => setReference(event.target.value)}
              />
            </label>
            <label>
              {forms.targetModuleLabel}
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
            <StatusBanner error={mutation.error} title={forms.addFormError} />
          ) : null}
          <Flex>
            <Button disabled={!reference.trim() || mutation.isPending} htmlType="submit">
              {mutation.isPending ? forms.loadingSchema : forms.addFormButton}
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
  const { messages, t } = useI18n();
  const forms = messages.googleForms;
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
    <Card className="google-forms-source">
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
          <StatusBanner
            description={forms.schemaDriftDesc}
            title={forms.schemaDriftMessage}
            tone="warning"
          />
        ) : null}
        {source.state === "NEEDS_REAUTH" ? (
          <StatusBanner
            description={forms.needsReauthDesc}
            title={forms.needsReauthMessage}
            tone="warning"
          />
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
                  aria-label={t(forms.destinationFor, { title: question.title })}
                  disabled={busy}
                  value={mapping[question.id] ?? ""}
                  onChange={(event) =>
                    setMapping((current) => ({ ...current, [question.id]: event.target.value }))
                  }
                >
                  <option value="">{forms.doNotImport}</option>
                  {fields.map((field) => (
                    <option
                      disabled={Object.entries(mapping).some(
                        ([questionID, target]) => questionID !== question.id && target === field.id,
                      )}
                      key={field.id}
                      value={field.id}
                    >
                      {field.label}
                      {field.required ? forms.requiredSuffix : ""}
                    </option>
                  ))}
                </select>
              ) : (
                <span className="google-forms-unsupported">
                  {forms.unsupportedPrefix}
                  {sourceErrorLabel(forms, question.unsupported_code)}
                </span>
              )}
            </label>
          ))}
        </div>
        <div className="google-forms-grid">
          <label>
            {forms.syncLabel}
            <select
              disabled={busy}
              value={syncMode}
              onChange={(event) => setSyncMode(event.target.value as "MANUAL" | "POLL")}
            >
              <option value="MANUAL">{forms.syncManual}</option>
              <option value="POLL">{forms.syncPoll}</option>
            </select>
          </label>
          <label>
            {forms.intervalLabel}
            <select
              disabled={busy || syncMode !== "POLL"}
              value={interval}
              onChange={(event) => setIntervalValue(Number(event.target.value))}
            >
              <option value={300}>{forms.intervals.m5}</option>
              <option value={900}>{forms.intervals.m15}</option>
              <option value={3600}>{forms.intervals.h1}</option>
              <option value={21600}>{forms.intervals.h6}</option>
              <option value={86400}>{forms.intervals.h24}</option>
            </select>
          </label>
        </div>
        {mutationError ? (
          <StatusBanner error={mutationError} title={forms.updateSourceError} />
        ) : null}
        <Flex>
          <Button disabled={busy} onClick={() => saveMapping.mutate()}>
            {forms.saveMapping}
          </Button>
          <Button disabled={busy} onClick={() => refresh.mutate()}>
            {forms.refreshSchema}
          </Button>
          {source.state === "ACTIVE" ? (
            <>
              <Button disabled={busy} onClick={() => sync.mutate()}>
                {forms.syncNow}
              </Button>
              <Button disabled={busy} onClick={() => updateState.mutate(false)}>
                {forms.pause}
              </Button>
            </>
          ) : (
            <Button
              disabled={busy || source.state === "NEEDS_REAUTH" || source.state === "SCHEMA_DRIFT"}
              onClick={() => updateState.mutate(true)}
            >
              {forms.activate}
            </Button>
          )}
        </Flex>
        <p className="google-forms-muted">
          {forms.lastSyncPrefix}
          {source.last_synced_at ? formatDate(source.last_synced_at) : forms.lastSyncNever}
          {source.pagination_pending ? forms.paginationPending : ""}
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
  const { messages } = useI18n();
  const forms = messages.googleForms;
  const names = new Map(sources.map((value) => [value.id, value.title]));
  return (
    <section className="page-section">
      <Typography.Title level={2}>{forms.historyTitle}</Typography.Title>
      <Typography.Paragraph>{forms.historyDesc}</Typography.Paragraph>
      {error ? <StatusBanner error={error} title={forms.historyError} /> : null}
      {loading ? <InlineStatus kind="loading" label={forms.loadingSyncs} /> : null}
      <Flex vertical gap="0.75rem">
        {values.map((value) => {
          const source = sources.find((item) => item.id === value.source_id);
          return (
            <SyncRow
              key={value.id}
              module={source?.module ?? "PROFILES"}
              name={names.get(value.source_id) ?? forms.defaultSourceName}
              onUpdated={onUpdated}
              value={value}
            />
          );
        })}
        {!loading && values.length === 0 ? (
          <StateCard
            compact
            description={forms.emptySyncsDesc}
            kind="empty"
            title={forms.emptySyncsTitle}
          />
        ) : null}
      </Flex>
    </section>
  );
}

function SyncRow({
  value,
  name,
  module,
  onUpdated,
}: {
  value: GoogleFormsSync;
  name: string;
  module: GoogleFormsModule;
  onUpdated: () => void;
}) {
  const { messages, t } = useI18n();
  const forms = messages.googleForms;
  const mutation = useMutation({
    mutationFn: () => cancelGoogleFormsSync(value),
    onSuccess: onUpdated,
  });
  return (
    <Card className="google-forms-sync">
      <Flex align="center" className="google-forms-heading">
        <div>
          <strong>{name}</strong>
          <p className="google-forms-muted">
            {t(forms.syncCounts, {
              created: formatDate(value.created_at),
              staged: value.staged_count,
              duplicates: value.duplicate_count,
            })}
          </p>
        </div>
        <Flex>
          <FormsStatus value={value.state} />
          {value.operation_import_id ? (
            <Link
              search={{
                table: tableFromModule(module),
                mode: "xlsx",
                import: value.operation_import_id,
              }}
              to="/cadastro"
            >
              {forms.reviewCadastro}
            </Link>
          ) : null}
          {value.state === "QUEUED" || value.state === "RUNNING" ? (
            <Button disabled={mutation.isPending} onClick={() => mutation.mutate()}>
              {forms.cancelSync}
            </Button>
          ) : null}
        </Flex>
      </Flex>
      {value.error_code ? (
        <span className="google-forms-muted">
          {forms.errorCodePrefix}
          {value.error_code}
        </span>
      ) : null}
    </Card>
  );
}

function FormsStatus({ value }: { value: string }) {
  const { messages } = useI18n();
  const labels = messages.googleForms.states as Record<string, string>;
  const tone =
    value === "ACTIVE" || value === "COMPLETED"
      ? "success"
      : value === "FAILED" || value === "ERROR" || value === "NEEDS_REAUTH"
        ? "danger"
        : value === "SCHEMA_DRIFT" || value === "PAUSED" || value === "CANCELLED"
          ? "warning"
          : "info";
  return <Tag color={tone}>{labels[value] ?? value}</Tag>;
}

function sourceErrorLabel(
  forms: {
    unsupportedFileUpload: string;
    unsupportedMultipleAnswers: string;
    unsupportedQuestionGroup: string;
    unsupportedDefault: string;
  },
  value?: string,
) {
  if (value === "file_upload") return forms.unsupportedFileUpload;
  if (value === "multiple_answers") return forms.unsupportedMultipleAnswers;
  if (value === "question_group") return forms.unsupportedQuestionGroup;
  return forms.unsupportedDefault;
}

function replaceGoogleFormsSearch(tab: "sources" | "history", sourceID: string) {
  const target = new URL(window.location.href);
  target.searchParams.delete("google_forms");
  target.searchParams.set("mode", "forms");
  target.searchParams.set("tab", tab);
  if (sourceID) target.searchParams.set("source", sourceID);
  else target.searchParams.delete("source");
  window.history.replaceState({}, "", `${target.pathname}${target.search}${target.hash}`);
}
