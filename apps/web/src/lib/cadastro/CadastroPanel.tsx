import { StateCard } from "../../components/StateCard";
import { useQuery } from "@tanstack/react-query";
import { useI18n } from "../../i18n";
import { getOCRCapability } from "../api/ocr";
import type { AttachmentOwner } from "../api/attachments";
import { CadastroImportWizard } from "./import/CadastroImportWizard";
import type { CadastroMode } from "./cadastroSearch";
import { moduleFromTable } from "./cadastroSearch";
import type { OperationModule } from "../api/operations";
import { CadastroManualPanel } from "./CadastroManualPanel";
import { CadastroOcrWorkspace } from "./CadastroOcrWorkspace";
import { OcrReviewPanel } from "./OcrReviewPanel";
import { useAttachmentsEnabled } from "./useAttachmentsEnabled";

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
      <CadastroImportWizard
        importId={importId}
        module={module}
        onClose={() => onImportChange(undefined)}
        onImportCreated={(id) => onImportChange(id)}
      />
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
        <CadastroOcrWorkspace
          ownerId={recordsOwner}
          recordId={recordId}
          table={table}
          typeId={typeId}
          onCreateInstead={onCreateInstead}
          onOwner={onOwner}
          onRecord={onRecord}
        />
      ) : null}
    </div>
  );
}

export function CadastroOcrSection({ owner }: { owner: AttachmentOwner }) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const attachmentsEnabled = useAttachmentsEnabled();
  const ocrCapability = useQuery({
    queryKey: ["ocr-capability"],
    queryFn: ({ signal }) => getOCRCapability(signal),
  });
  const enabled = (attachmentsEnabled.data ?? false) && (ocrCapability.data?.enabled ?? false);
  if (!enabled) {
    return (
      <StateCard
        compact
        description={copy.ocrUnavailableR2}
        kind="warning"
        title={copy.ocrUnavailable}
      />
    );
  }
  return <OcrReviewPanel owner={owner} />;
}
