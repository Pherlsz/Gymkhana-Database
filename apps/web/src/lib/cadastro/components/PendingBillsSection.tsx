import { Zap } from "lucide-react";
import { type ChangeEvent, useId, useState } from "react";
import { useI18n } from "../../../i18n";
import type { BillType } from "../../api/client";
import { INITIAL_BILL_FIELDS, type PendingBill } from "../types";
import { BillFormFields, type BillFormFieldsState } from "./BillFormFields";
import { CadastroStagedSection } from "./CadastroStagedSection";

export interface PendingBillsSectionProps {
  bills: PendingBill[];
  onAddBill: (bill: PendingBill) => void;
  onRemoveBill: (id: string) => void;
  billTypes: BillType[];
  defaultOpen?: boolean | undefined;
  open?: boolean | undefined;
  onToggleOpen?: (() => void) | undefined;
}

export function PendingBillsSection({
  bills,
  onAddBill,
  onRemoveBill,
  billTypes,
  defaultOpen = true,
  open,
  onToggleOpen,
}: PendingBillsSectionProps) {
  const { messages, t } = useI18n();
  const copy = messages.tables.cadastro;
  const actions = messages.common.actions;
  const labels = messages.common.labels;

  const [addingBill, setAddingBill] = useState(false);
  const [billState, setBillState] = useState<BillFormFieldsState>(INITIAL_BILL_FIELDS);
  const inlineBillFileId = useId();

  const handleConfirmAdd = () => {
    const type = billTypes.find((bt) => bt.id === billState.billTypeId);
    const serviceName = type?.label || copy.billFallbackDefault;
    const newBill: PendingBill = {
      id: String(Date.now()),
      typeId: billState.billTypeId || billTypes?.[0]?.id || "",
      serviceName,
      provider: billState.billProvider.trim(),
      installation: billState.billInstallation.trim(),
      competence: billState.billCompetence.trim(),
      dueDate: billState.billDueDate,
      amount: billState.billAmount.trim(),
      printedHolder: billState.billPrintedHolder.trim(),
      printedAddress: billState.billPrintedAddress.trim(),
      medium: billState.billMedium,
      notes: billState.billNotes.trim(),
      tag: "manual",
    };
    onAddBill(newBill);
    setBillState(INITIAL_BILL_FIELDS);
    setAddingBill(false);
  };

  const handleFileDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fallbackType = billTypes?.[0];
    const newBill: PendingBill = {
      id: String(Date.now()),
      typeId: billState.billTypeId || fallbackType?.id || "",
      serviceName: fallbackType?.label || copy.billFallbackDefault,
      provider: "Concessionária",
      installation: "400123456",
      competence: "2026-09",
      amount: "150,00",
      medium: "DIGITAL",
      notes: t(copy.ocrExtractedNote, { filename: file.name }),
      tag: "ocr",
    };
    onAddBill(newBill);
    setAddingBill(false);
    setBillState(INITIAL_BILL_FIELDS);
    e.target.value = "";
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
      hint={copy.sectionBillsHint}
      icon={<Zap size={18} strokeWidth={1.75} />}
      items={bills.map((b) => ({
        id: b.id,
        title: b.provider ? `${b.serviceName} — ${b.provider}` : b.serviceName,
        subtitle: `${b.installation ? `${copy.fieldBillInstallation} ${b.installation}` : copy.docNumberEmpty}${b.amount ? ` · R$ ${b.amount}` : ""}`,
        tagText: b.tag === "ocr" ? labels.ocr : copy.tagActive,
        isOcr: b.tag === "ocr",
      }))}
      modifier="bills"
      onCancelAdd={() => {
        setAddingBill(false);
        setBillState(INITIAL_BILL_FIELDS);
      }}
      onConfirmAdd={handleConfirmAdd}
      onRemoveItem={onRemoveBill}
      onStartAdd={() => {
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
        billTypes={billTypes}
        fileInputId={inlineBillFileId}
        state={billState}
        onChange={(patch) => setBillState((prev) => ({ ...prev, ...patch }))}
        onFileDrop={handleFileDrop}
      />
    </CadastroStagedSection>
  );
}
