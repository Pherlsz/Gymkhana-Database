import { Segmented } from "antd";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { CadastroStateCard } from "../../../components/StateCard";
import { useI18n } from "../../../i18n";
import {
  getOperationsCatalog,
  getOperationImport,
  getOperationImportReport,
  type OperationModule,
} from "../../api/operations";
import { ImportCreator, ImportWorkspace } from "../../../OperationsPage";
import { operationActive, operationTerminal } from "./importHelpers";

export function CadastroImportWizard({
  module,
  importId,
  onClose,
  onImportCreated,
}: {
  module: OperationModule;
  importId?: string | undefined;
  onClose: () => void;
  onImportCreated: (id: string) => void;
}) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const queryClient = useQueryClient();
  const [currentModule, setCurrentModule] = useState<OperationModule>(module);

  const catalog = useQuery({
    queryKey: ["operations-catalog"],
    queryFn: ({ signal }) => getOperationsCatalog(signal),
    staleTime: 60_000,
  });
  const selectedImport = useQuery({
    queryKey: ["operation-import", importId],
    queryFn: ({ signal }) => getOperationImport(importId!, signal),
    enabled: Boolean(importId),
    refetchInterval: (query) =>
      query.state.data && operationActive(query.state.data.state) ? 1_500 : false,
  });
  const selectedReport = useQuery({
    queryKey: ["operation-import-report", importId],
    queryFn: ({ signal }) => getOperationImportReport(importId!, signal),
    enabled: Boolean(
      importId && selectedImport.data && operationTerminal(selectedImport.data.state),
    ),
  });
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["operation-imports"] }),
      queryClient.invalidateQueries({ queryKey: ["operation-import", importId] }),
      queryClient.invalidateQueries({ queryKey: ["operation-import-report", importId] }),
    ]);
  };
  const modules = useMemo(
    () => catalog.data?.modules.filter((value) => value.can_import) ?? [],
    [catalog.data?.modules],
  );
  const isModuleAllowed = modules.some((value) => value.id === currentModule);

  if (catalog.isError) {
    return (
      <CadastroStateCard
        backLabel={copy.formsBackToCadastro}
        description={copy.massImportCatalogErrorDesc}
        kind="error"
        onRetry={() => void refresh()}
        title={copy.massImportCatalogError}
      />
    );
  }
  if (!catalog.data) {
    return (
      <CadastroStateCard
        backToCadastro={false}
        description={copy.massImportLoadingDesc}
        kind="loading"
        title={copy.massImportLoading}
      />
    );
  }
  if (!isModuleAllowed) {
    return (
      <CadastroStateCard
        backLabel={copy.formsBackToCadastro}
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
                  { label: copy.people, value: "profiles" },
                  { label: copy.documents, value: "documents" },
                  { label: copy.bills, value: "bills" },
                ]}
                value={currentModule}
                onChange={(val) => setCurrentModule(val as OperationModule)}
              />
            </div>
            <p className="cadastro-import-scope__notice">{copy.importScopeNotice}</p>
          </div>
          <ScopedImportCreator
            catalog={catalog.data}
            module={currentModule}
            onCreated={(value) => onImportCreated(value.id)}
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
        onClose={onClose}
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
