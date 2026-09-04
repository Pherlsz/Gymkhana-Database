import { useI18n } from "../../i18n";
import { CadastroOwnerPicker } from "./CadastroOwnerPicker";

export function CadastroManualPanel({
  table,
  recordsOwner,
  onOwner,
  onClearOwner,
}: {
  table: "documents" | "bills";
  recordsOwner?: string | undefined;
  onOwner: (ownerProfileId: string) => void;
  onClearOwner: () => void;
}) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const tableLabel = table === "documents" ? copy.documents : copy.bills;
  return (
    <div className="cadastro-manual">
      {recordsOwner ? null : (
        <p className="cadastro-manual__lead">
          {copy.manualLeadOwned.replace("{table}", tableLabel)}
        </p>
      )}
      <CadastroOwnerPicker ownerId={recordsOwner} onClear={onClearOwner} onSelect={onOwner} />
    </div>
  );
}
