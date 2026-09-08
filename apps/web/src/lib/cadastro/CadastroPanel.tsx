import { lazy, Suspense } from "react";
import { Spin } from "antd";
import type { CadastroMode } from "./cadastroSearch";
import { moduleFromTable } from "./cadastroSearch";
import type { OperationModule } from "../api/operations";
import { CadastroManualPanel } from "./CadastroManualPanel";

export { CadastroOcrSection } from "./CadastroOcrSection";

const CadastroImportWizard = lazy(() =>
  import("./import/CadastroImportWizard").then((m) => ({ default: m.CadastroImportWizard })),
);

const CadastroOcrWorkspace = lazy(() =>
  import("./CadastroOcrWorkspace").then((m) => ({ default: m.CadastroOcrWorkspace })),
);

export function CadastroPanel({
  table,
  cadastro,
  importId,
  recordsOwner,
  recordId,
  typeId,
  onImportChange,
  onOwner,
  onClearOwner,
  onRecord,
  onCreateInstead,
}: {
  table: "people" | "documents" | "bills";
  cadastro: CadastroMode;
  importId?: string | undefined;
  recordsOwner?: string | undefined;
  recordId?: string | undefined;
  typeId?: string | undefined;
  onImportChange: (importId?: string) => void;
  onOwner: (ownerProfileId: string) => void;
  onClearOwner: () => void;
  onRecord: (next: { table: "documents" | "bills"; id: string }) => void;
  onCreateInstead: () => void;
}) {
  const module = moduleFromTable(table) satisfies OperationModule;

  if (cadastro === "xlsx") {
    return (
      <Suspense fallback={<Spin size="large" style={{ display: "block", margin: "3rem auto" }} />}>
        <CadastroImportWizard
          importId={importId}
          module={module}
          onClose={() => onImportChange(undefined)}
          onImportCreated={(id) => onImportChange(id)}
        />
      </Suspense>
    );
  }

  return (
    <div className="cadastro-panel">
      {cadastro === "manual" && table !== "people" ? (
        <CadastroManualPanel
          recordsOwner={recordsOwner}
          table={table}
          onClearOwner={onClearOwner}
          onOwner={onOwner}
        />
      ) : null}

      {cadastro === "ocr" ? (
        <Suspense
          fallback={<Spin size="large" style={{ display: "block", margin: "3rem auto" }} />}
        >
          <CadastroOcrWorkspace
            ownerId={recordsOwner}
            recordId={recordId}
            table={table}
            typeId={typeId}
            onCreateInstead={onCreateInstead}
            onOwner={onOwner}
            onRecord={onRecord}
          />
        </Suspense>
      ) : null}
    </div>
  );
}
