import { Zap } from "lucide-react";
import { useId, useState } from "react";
import { useI18n } from "../../../i18n";
import type { BillType } from "../../api/client";
import { isCompetenceMonth } from "../cadastroValidate";
import { INITIAL_BILL_FIELDS, type PendingBill } from "../types";
import { BillFormFields, type BillFormFieldsState } from "./BillFormFields";
import { CadastroStagedSection } from "./CadastroStagedSection";

export interface PendingBillsSectionProps {
  bills: PendingBill[];
  onAddBill: (bill: PendingBill) => void;
  onRemoveBill: (id: string) => void;
  billTypes: BillType[];
  attachmentsEnabled?: boolean | undefined;
  defaultOpen?: boolean | undefined;
  open?: boolean | undefined;
  onToggleOpen?: (() => void) | undefined;
}

export function PendingBillsSection({
  bills,
  onAddBill,
  onRemoveBill,
  billTypes,
  attachmentsEnabled,
  defaultOpen = true,
  open,
  onToggleOpen,
}: PendingBillsSectionProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const actions = messages.common.actions;
  const labels = messages.common.labels;

  const [addingBill, setAddingBill] = useState(false);
  const [billState, setBillState] = useState<BillFormFieldsState>(INITIAL_BILL_FIELDS);
  const [formError, setFormError] = useState<string | null>(null);
  const inlineBillFileId = useId();

  const handleConfirmAdd = () => {
    const type = billTypes.find((bt) => bt.id === billState.billTypeId) ?? billTypes[0];
    if (!type) {
      setFormError(copy.errorBillTypeRequired);
      return;
    }
    if (!billState.billInstallation.trim()) {
      setFormError(copy.errorBillInstallationRequired);
      return;
    }
    if (!isCompetenceMonth(billState.billCompetence)) {
      setFormError(copy.errorBillCompetenceRequired);
      return;
    }
    if (!billState.billAmount.trim()) {
      setFormError(copy.errorBillAmountRequired);
      return;
    }
    const newBill: PendingBill = {
      id: String(Date.now()),
      typeId: type.id,
      serviceName: type.label,
      typeKey: type.technical_key,
      installation: billState.billInstallation.trim(),
      competence: billState.billCompetence.trim(),
      amount: billState.billAmount.trim(),
      printedHolder: billState.billPrintedHolder.trim(),
      printedAddress: billState.billPrintedAddress.trim(),
      medium: billState.billMedium,
      notes: billState.billNotes.trim(),
      tag: billState.file ? "queued" : "manual",
      customDraft: billState.customDraft,
      file: billState.file,
    };
    onAddBill(newBill);
    setBillState(INITIAL_BILL_FIELDS);
    setFormError(null);
    setAddingBill(false);
  };

  return (
    <CadastroStagedSection
      addLabel={copy.btnAddBill}
      adding={addingBill}
      cancelLabel={actions.cancel}
      confirmLabel={actions.add}
      count={bills.length}
      defaultOpen={defaultOpen}
      emptyText={copy.sectionBillsEmpty}
      error={formError ?? undefined}
      hint={copy.sectionBillsHint}
      icon={<Zap size={18} strokeWidth={1.75} />}
      items={bills.map((b) => ({
        id: b.id,
        title: b.serviceName,
        subtitle: `${b.installation ? `${copy.fieldBillInstallation} ${b.installation}` : copy.docNumberEmpty}${b.amount ? ` · R$ ${b.amount}` : ""}`,
        tagText: b.tag === "queued" ? labels.ocr : copy.tagActive,
        isOcr: b.tag === "queued",
      }))}
      modifier="bills"
      onCancelAdd={() => {
        setAddingBill(false);
        setBillState(INITIAL_BILL_FIELDS);
        setFormError(null);
      }}
      onConfirmAdd={handleConfirmAdd}
      onRemoveItem={onRemoveBill}
      onStartAdd={() => {
        setFormError(null);
        setBillState({
          ...INITIAL_BILL_FIELDS,
          billTypeId: billTypes?.[0]?.id || "",
        });
        setAddingBill(true);
      }}
      onToggleOpen={onToggleOpen}
      open={open}
      removeLabel={actions.remove}
      title={copy.sectionBillsTitle}
    >
      <BillFormFields
        attachmentsEnabled={attachmentsEnabled}
        billTypes={billTypes}
        fileInputId={inlineBillFileId}
        state={billState}
        onChange={(patch) => setBillState((prev) => ({ ...prev, ...patch }))}
      />
    </CadastroStagedSection>
  );
}
