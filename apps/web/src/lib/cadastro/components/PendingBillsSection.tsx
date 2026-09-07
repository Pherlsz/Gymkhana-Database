import { Button, Input, Select } from "antd";
import { Plus, Trash2, Zap } from "lucide-react";
import { type ChangeEvent, useId, useState } from "react";
import { useI18n } from "../../../i18n";
import type { BillType } from "../../api/client";
import { INITIAL_INLINE_BILL, type InlineBillState, type PendingBill } from "../types";
import { CadastroSection } from "./CadastroSection";
import { OcrDropzoneInline } from "./OcrDropzoneInline";

export interface PendingBillsSectionProps {
  bills: PendingBill[];
  onAddBill: (bill: PendingBill) => void;
  onRemoveBill: (id: string) => void;
  billTypes: BillType[];
  defaultOpen?: boolean;
  open?: boolean;
  onToggleOpen?: () => void;
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
  const [inlineBill, setInlineBill] = useState<InlineBillState>(INITIAL_INLINE_BILL);
  const inlineBillFileId = useId();

  const handleConfirmAdd = () => {
    const type = billTypes.find((t) => t.id === inlineBill.typeId);
    const serviceName = type?.label || copy.billFallbackDefault;
    const newBill: PendingBill = {
      id: String(Date.now()),
      typeId: inlineBill.typeId || billTypes?.[0]?.id || "",
      serviceName,
      provider: inlineBill.provider.trim(),
      installation: inlineBill.installation.trim(),
      competence: inlineBill.competence.trim(),
      amount: inlineBill.amount.trim(),
      medium: "DIGITAL",
      notes: "",
      tag: "manual",
    };
    onAddBill(newBill);
    setInlineBill(INITIAL_INLINE_BILL);
    setAddingBill(false);
  };

  const handleFileDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fallbackType = billTypes?.[0];
    const newBill: PendingBill = {
      id: String(Date.now()),
      typeId: inlineBill.typeId || fallbackType?.id || "",
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
    e.target.value = "";
  };

  return (
    <CadastroSection
      badge={<span className="cadastro-group__badge">{bills.length}</span>}
      defaultOpen={defaultOpen}
      hint={copy.sectionBillsHint}
      icon={<Zap size={18} strokeWidth={1.75} />}
      onToggleOpen={onToggleOpen}
      open={open}
      title={copy.sectionBillsTitle}
    >
      <div className="strip">
        {bills.length === 0 ? (
          <div className="emptyrow">{copy.sectionBillsEmpty}</div>
        ) : (
          bills.map((b) => (
            <div key={b.id} className="srow">
              <span className="mk">
                <Zap size={16} strokeWidth={1.75} />
              </span>
              <div className="tx">
                <b>
                  {b.serviceName}
                  {b.provider ? ` — ${b.provider}` : ""}
                </b>
                <span>
                  {b.installation
                    ? `${copy.fieldBillInstallation} ${b.installation}`
                    : copy.docNumberEmpty}
                  {b.amount ? ` · R$ ${b.amount}` : ""}
                </span>
              </div>
              <span className="grow" />
              <span className={`tag ${b.tag === "ocr" ? "gold" : "ok"}`}>
                {b.tag === "ocr" ? labels.ocr : copy.tagActive}
              </span>
              <div className="acts">
                <Button
                  danger
                  type="text"
                  size="small"
                  icon={<Trash2 size={13} />}
                  onClick={() => onRemoveBill(b.id)}
                >
                  {actions.remove}
                </Button>
              </div>
            </div>
          ))
        )}
      </div>

      {addingBill ? (
        <div className="inline-addform">
          <div className="cadastro-grid">
            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldBillService}</label>
                <Select
                  options={
                    billTypes.map((t: BillType) => ({
                      value: t.id,
                      label: t.label,
                    })) ?? []
                  }
                  value={inlineBill.typeId || billTypes?.[0]?.id}
                  onChange={(v) => {
                    if (v) setInlineBill((prev) => ({ ...prev, typeId: v }));
                  }}
                />
              </div>
            </div>
            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldBillProvider}</label>
                <Input
                  placeholder={copy.placeholderBillProvider}
                  value={inlineBill.provider}
                  onChange={(e) => setInlineBill((prev) => ({ ...prev, provider: e.target.value }))}
                />
              </div>
            </div>
            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldBillInstallation}</label>
                <Input
                  placeholder={copy.placeholderBillInstallation}
                  value={inlineBill.installation}
                  onChange={(e) =>
                    setInlineBill((prev) => ({ ...prev, installation: e.target.value }))
                  }
                />
              </div>
            </div>
            <div className="cadastro-col-6">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldBillCompetence}</label>
                <Input
                  placeholder={copy.placeholderBillCompetence}
                  value={inlineBill.competence}
                  onChange={(e) =>
                    setInlineBill((prev) => ({ ...prev, competence: e.target.value }))
                  }
                />
              </div>
            </div>
            <div className="cadastro-col-6">
              <div className="cadastro-field">
                <label className="cadastro-field__label">{copy.fieldBillAmount}</label>
                <Input
                  placeholder={copy.placeholderBillAmount}
                  value={inlineBill.amount}
                  onChange={(e) => setInlineBill((prev) => ({ ...prev, amount: e.target.value }))}
                />
              </div>
            </div>
          </div>

          <OcrDropzoneInline
            inputId={inlineBillFileId}
            label={copy.ocrInlineDropzoneBill}
            onFile={handleFileDrop}
          />

          <div className="inline-actions">
            <Button onClick={() => setAddingBill(false)}>{actions.cancel}</Button>
            <Button type="primary" onClick={handleConfirmAdd}>
              {actions.add}
            </Button>
          </div>
        </div>
      ) : (
        <Button
          className="addrow"
          type="dashed"
          icon={<Plus size={15} strokeWidth={2} />}
          onClick={() => {
            setInlineBill((prev) => ({
              ...prev,
              typeId: billTypes?.[0]?.id || "",
            }));
            setAddingBill(true);
          }}
        >
          {copy.btnAddBill}
        </Button>
      )}
    </CadastroSection>
  );
}
