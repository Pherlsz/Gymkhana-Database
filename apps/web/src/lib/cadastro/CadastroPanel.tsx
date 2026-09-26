import { moduleFromTable } from "./cadastroSearch";
import type { OperationModule } from "../api/operations";
import { CadastroImportWizard } from "./import/CadastroImportWizard";

export function CadastroPanel({
  table,
  importId,
  onImportChange,
  onModuleChange,
  onNotice,
}: {
  table: "people" | "documents" | "bills";
  importId?: string | undefined;
  onImportChange: (importId?: string) => void;
  onModuleChange: (module: OperationModule) => void;
  onNotice?: (message: string) => void;
}) {
  const module = moduleFromTable(table) satisfies OperationModule;

  return (
    <CadastroImportWizard
      importId={importId}
      module={module}
      onImportCreated={(id) => onImportChange(id)}
      onModuleChange={onModuleChange}
      {...(onNotice ? { onNotice } : {})}
    />
  );
}
