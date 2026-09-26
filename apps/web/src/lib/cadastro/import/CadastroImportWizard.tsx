import { Segmented } from "antd";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useI18n } from "../../../i18n";
import { APIRequestError } from "../../api/client";
import {
  deleteOperationImport,
  getOperationsCatalog,
  getOperationImport,
  getOperationImportReport,
  listOperationImports,
  type OperationImport,
  type OperationModule,
} from "../../api/operations";
import { queryKeys } from "../../api/queryKeys";
import { ImportCreator, ImportWorkspace } from "../../operations/ImportWorkspace";
import { operationActive, operationTerminal, stateLabel } from "../../operations/operationLabels";
import { CadastroWorkQuery, CadastroWorkState, confirmCadastroAction } from "../CadastroWork";

export function CadastroImportWizard({
  module,
  importId,
  onImportCreated,
  onModuleChange,
  onNotice,
}: {
  module: OperationModule;
  importId?: string | undefined;
  onImportCreated: (id: string) => void;
  onModuleChange?: (module: OperationModule) => void;
  onNotice?: (message: string) => void;
}) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const queryClient = useQueryClient();
  const [currentModule, setCurrentModule] = useState<OperationModule>(module);
  const [historyBusy, setHistoryBusy] = useState(false);

  useEffect(() => {
    setCurrentModule(module);
  }, [module]);

  const catalog = useQuery({
    queryKey: queryKeys.operations.catalog,
    queryFn: ({ signal }) => getOperationsCatalog(signal),
    staleTime: 60_000,
    retry: (count, error) => !isUnavailable(error) && count < 2,
  });
  const history = useQuery({
    queryKey: queryKeys.operations.importsList,
    queryFn: ({ signal }) => listOperationImports(signal),
    enabled: !importId && !catalog.isError,
    retry: (count, error) => !isUnavailable(error) && count < 2,
  });
  const selectedImport = useQuery({
    queryKey: queryKeys.operations.import(importId),
    queryFn: ({ signal }) => getOperationImport(importId!, signal),
    enabled: Boolean(importId),
    refetchInterval: (query) =>
      query.state.data && operationActive(query.state.data.state) ? 1_500 : false,
  });
  const selectedReport = useQuery({
    queryKey: queryKeys.operations.importReport(importId),
    queryFn: ({ signal }) => getOperationImportReport(importId!, signal),
    enabled: Boolean(
      importId && selectedImport.data && operationTerminal(selectedImport.data.state),
    ),
  });
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.operations.importsList }),
      queryClient.invalidateQueries({ queryKey: queryKeys.cadastro.entryImports }),
      queryClient.invalidateQueries({ queryKey: queryKeys.operations.import(importId) }),
      queryClient.invalidateQueries({ queryKey: queryKeys.operations.importReport(importId) }),
    ]);
  };
  const modules = useMemo(
    () => catalog.data?.modules.filter((value) => value.can_import) ?? [],
    [catalog.data?.modules],
  );
  const isModuleAllowed = modules.some((value) => value.id === currentModule);
  const recentImports = useMemo(
    () =>
      (history.data?.imports ?? [])
        .filter((value) => value.module === currentModule && value.source_kind === "XLSX")
        .toSorted((a, b) => b.created_at.localeCompare(a.created_at))
        .slice(0, 8),
    [currentModule, history.data?.imports],
  );
  const clearableImports = useMemo(
    () => recentImports.filter((value) => operationTerminal(value.state)),
    [recentImports],
  );
  const removeHistory = async (items: OperationImport[], confirm: boolean) => {
    if (items.length === 0) return;
    if (confirm) {
      const ok = await confirmCadastroAction({
        title: copy.importHistoryLabel,
        content: copy.importHistoryClearConfirm,
        okText: messages.common.actions.clear,
        cancelText: messages.common.actions.cancel,
      });
      if (!ok) return;
    }
    setHistoryBusy(true);
    try {
      const results = await Promise.allSettled(items.map((item) => deleteOperationImport(item.id)));
      const failed = results.find(
        (result): result is PromiseRejectedResult => result.status === "rejected",
      );
      if (failed) {
        onNotice?.(
          failed.reason instanceof Error ? failed.reason.message : copy.importHistoryClearError,
        );
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.operations.importsList }),
        queryClient.invalidateQueries({ queryKey: queryKeys.cadastro.entryImports }),
      ]);
    } finally {
      setHistoryBusy(false);
    }
  };

  const selectModule = (next: OperationModule) => {
    setCurrentModule(next);
    onModuleChange?.(next);
  };

  if (catalog.isError) {
    const unavailable = isUnavailable(catalog.error);
    if (unavailable) {
      return (
        <CadastroWorkState
          description={copy.massImportUnavailableR2}
          kind="warning"
          title={copy.massImportCatalogError}
        />
      );
    }
    return (
      <CadastroWorkQuery
        error={catalog.error}
        errorTitle={copy.massImportCatalogError}
        errorDescription={copy.massImportCatalogErrorDesc}
        isError
        onRetry={() => void refresh()}
      />
    );
  }
  if (!catalog.data) {
    return (
      <CadastroWorkQuery
        isPending
        loadingDescription={copy.massImportLoadingDesc}
        loadingTitle={copy.massImportLoading}
      />
    );
  }
  if (!isModuleAllowed) {
    return (
      <CadastroWorkState
        description={copy.massImportForbiddenDesc}
        kind="warning"
        title={copy.massImportForbidden}
      />
    );
  }

  if (!importId) {
    return (
      <div className="cadastro-panel">
        <div className="cadastro-import-flow">
          <div className="cadastro-import-scope">
            <div className="cadastro-import-scope__header">
              <span className="cadastro-import-scope__label">{copy.importTargetLabel}</span>
              <Segmented
                options={[
                  { label: copy.people, value: "PROFILES" },
                  { label: copy.documents, value: "DOCUMENTS" },
                  { label: copy.bills, value: "BILLS" },
                ]}
                value={currentModule}
                onChange={(val) => selectModule(val as OperationModule)}
              />
            </div>
            <p className="cadastro-import-scope__notice">{copy.importScopeNotice}</p>
          </div>
          <ScopedImportCreator
            catalog={catalog.data}
            module={currentModule}
            onCreated={(value) => onImportCreated(value.id)}
          />
          <ImportHistoryList
            busy={historyBusy}
            clearLabel={messages.common.actions.clear}
            emptyLabel={copy.entryBulkFootNone}
            items={recentImports}
            label={copy.importHistoryLabel}
            loading={history.isLoading}
            removeAria={copy.importHistoryRemoveAria}
            onClear={() => void removeHistory(clearableImports, true)}
            onOpen={(value) => onImportCreated(value.id)}
            onRemove={(value) => void removeHistory([value], false)}
          />
        </div>
      </div>
    );
  }

  return (
    <div className="cadastro-panel">
      <ImportWorkspace
        catalog={catalog.data.modules}
        error={selectedImport.error}
        loading={selectedImport.isLoading}
        report={selectedReport.data}
        reportError={selectedReport.error}
        reportLoading={selectedReport.isLoading}
        value={selectedImport.data}
        onUpdated={refresh}
      />
    </div>
  );
}

function ScopedImportCreator({
  catalog,
  module,
  onCreated,
}: {
  catalog: Awaited<ReturnType<typeof getOperationsCatalog>>;
  module: OperationModule;
  onCreated: (value: { id: string }) => void;
}) {
  const scopedCatalog = useMemo(
    () => ({
      ...catalog,
      modules: catalog.modules.filter((value) => value.id === module),
    }),
    [catalog, module],
  );
  return <ImportCreator catalog={scopedCatalog} onCreated={onCreated} />;
}

function ImportHistoryList({
  busy,
  clearLabel,
  emptyLabel,
  items,
  label,
  loading,
  onClear,
  onOpen,
  onRemove,
  removeAria,
}: {
  busy: boolean;
  clearLabel: string;
  emptyLabel: string;
  items: OperationImport[];
  label: string;
  loading: boolean;
  onClear: () => void;
  onOpen: (value: OperationImport) => void;
  onRemove: (value: OperationImport) => void;
  removeAria: string;
}) {
  const { messages, t } = useI18n();
  const canClear = items.some((value) => operationTerminal(value.state));
  return (
    <section className="cadastro-import-history">
      <div className="cadastro-import-history__head">
        <h2 className="cadastro-import-history__label">{label}</h2>
        {canClear ? (
          <button
            className="cadastro-import-history__clear"
            disabled={busy}
            type="button"
            onClick={onClear}
          >
            {clearLabel}
          </button>
        ) : null}
      </div>
      {loading ? (
        <p className="cadastro-import-history__empty">{messages.operations.workspace.reportWait}</p>
      ) : items.length === 0 ? (
        <p className="cadastro-import-history__empty">{emptyLabel}</p>
      ) : (
        <ul className="cadastro-import-history__list">
          {items.map((value) => {
            const name = value.original_filename || value.id;
            return (
              <li key={value.id} className="cadastro-import-history__row">
                <button
                  className="cadastro-import-history__item"
                  type="button"
                  onClick={() => onOpen(value)}
                >
                  <span className="cadastro-import-history__name">{name}</span>
                  <span className="cadastro-import-history__meta">
                    {stateLabel(value.state, messages)} · {timeAgo(value.created_at)}
                  </span>
                </button>
                {operationTerminal(value.state) ? (
                  <button
                    aria-label={t(removeAria, { name })}
                    className="cadastro-import-history__remove"
                    disabled={busy}
                    type="button"
                    onClick={() => onRemove(value)}
                  >
                    <X aria-hidden size={14} strokeWidth={2} />
                  </button>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function isUnavailable(error: unknown) {
  return error instanceof APIRequestError && error.status === 503;
}

function timeAgo(iso: string): string {
  const delta = Date.now() - new Date(iso).getTime();
  const minutes = Math.max(0, Math.round(delta / 60_000));
  if (minutes < 1) return "agora";
  if (minutes < 60) return `${minutes} min`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} h`;
  return `${Math.round(hours / 24)} d`;
}
