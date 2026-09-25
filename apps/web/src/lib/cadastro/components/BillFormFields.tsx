import { Button, Input, Segmented, Select } from "antd";
import { ChevronDown, ChevronUp } from "lucide-react";
import { type ChangeEvent, useState } from "react";
import { RecordCustomFieldsSection, type CustomDraftValue } from "../../../RecordCustomFields";
import { useI18n } from "../../../i18n";
import type { BillType } from "../../api/client";
import { fileFromInput } from "../cadastroValidate";
import { OcrDropzoneInline } from "./OcrDropzoneInline";

export interface BillFormFieldsState {
  billTypeId: string;
  billInstallation: string;
  billCompetence: string;
  billAmount: string;
  billPrintedHolder: string;
  billPrintedAddress: string;
  billMedium: "PHYSICAL" | "DIGITAL";
  billNotes: string;
  customDraft: Record<string, CustomDraftValue>;
  file?: File | undefined;
}

export interface BillFormFieldsProps {
  state: BillFormFieldsState;
  onChange: (patch: Partial<BillFormFieldsState>) => void;
  billTypes: BillType[];
  fileInputId: string;
  lockType?: boolean | undefined;
  attachmentsEnabled?: boolean | undefined;
  disabled?: boolean | undefined;
  showDropzone?: boolean | undefined;
  copy?: Record<string, string>;
}

export function BillFormFields({
  state,
  onChange,
  billTypes,
  fileInputId,
  lockType,
  attachmentsEnabled,
  disabled,
  showDropzone = true,
  copy: customCopy,
}: BillFormFieldsProps) {
  const { messages } = useI18n();
  const labels = messages.common.labels;
  const copy = { ...messages.tables.cadastro, ...customCopy };
  const isDisabled = Boolean(disabled);
  const dropEnabled = attachmentsEnabled !== false;
  const [showExtras, setShowExtras] = useState(
    Boolean(
      state.billPrintedHolder || state.billPrintedAddress || Object.keys(state.customDraft).length,
    ),
  );

  const handleFile = (event: ChangeEvent<HTMLInputElement>) => {
    const file = fileFromInput(event);
    if (file) onChange({ file, billMedium: "DIGITAL" });
    event.target.value = "";
  };

  return (
    <div className="cadastro-bill-fields">
      {showDropzone ? (
        <OcrDropzoneInline
          actionText={copy.ocrBannerAction}
          badgeText={copy.ocrBannerBadge}
          description={copy.ocrBannerDescBill ?? copy.ocrBannerDesc}
          disabled={!dropEnabled || isDisabled}
          inputId={fileInputId}
          label={copy.ocrInlineDropzoneBill}
          queuedName={state.file?.name}
          title={copy.ocrBannerTitle}
          variant="banner"
          onFile={handleFile}
        />
      ) : null}

      <div className="cadastro-grid">
        <div className="cadastro-col-12">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-type">
              {copy.fieldBillService} <span className="cadastro-field__required">*</span>
            </label>
            <Select
              disabled={isDisabled || Boolean(lockType)}
              id="cad-bill-type"
              options={billTypes.map((t) => ({ value: t.id, label: t.label }))}
              placeholder={copy.pickerSelectType}
              style={{ width: "100%" }}
              value={state.billTypeId || undefined}
              onChange={(v) => {
                if (v) onChange({ billTypeId: v, customDraft: {} });
              }}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-inst">
              {copy.fieldBillInstallation} <span className="cadastro-field__required">*</span>
            </label>
            <Input
              disabled={isDisabled}
              id="cad-bill-inst"
              placeholder={copy.placeholderBillInstallation}
              value={state.billInstallation}
              onChange={(e) => onChange({ billInstallation: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-comp">
              {copy.fieldBillCompetence} <span className="cadastro-field__required">*</span>
            </label>
            <Input
              disabled={isDisabled}
              id="cad-bill-comp"
              placeholder={copy.placeholderBillCompetence}
              value={state.billCompetence}
              onChange={(e) => onChange({ billCompetence: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-val">
              {copy.fieldBillAmount} <span className="cadastro-field__required">*</span>
            </label>
            <Input
              disabled={isDisabled}
              id="cad-bill-val"
              placeholder={copy.placeholderBillAmount ?? "0,00"}
              prefix={
                <span
                  style={{ color: "var(--gym-color-text-muted)", fontSize: 13, marginRight: 2 }}
                >
                  R$
                </span>
              }
              value={state.billAmount}
              onChange={(e) => onChange({ billAmount: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label">{labels.medium}</label>
            <Segmented
              block
              disabled={isDisabled}
              options={[
                { label: labels.physical, value: "PHYSICAL" },
                { label: labels.digital, value: "DIGITAL" },
              ]}
              value={state.billMedium}
              onChange={(v) => onChange({ billMedium: v as "PHYSICAL" | "DIGITAL" })}
            />
          </div>
        </div>

        <div className="cadastro-col-12">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-notes">
              {labels.notes}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-bill-notes"
              placeholder={copy.placeholderBillNotes}
              value={state.billNotes}
              onChange={(e) => onChange({ billNotes: e.target.value })}
            />
          </div>
        </div>
      </div>

      <Button
        className="cadastro-toggle-btn"
        type="text"
        icon={
          showExtras ? (
            <ChevronUp size={14} strokeWidth={2} />
          ) : (
            <ChevronDown size={14} strokeWidth={2} />
          )
        }
        onClick={() => setShowExtras((prev) => !prev)}
      >
        {showExtras ? copy.toggleBillExtrasLess : copy.toggleBillExtras}
      </Button>

      {showExtras ? (
        <div className="cadastro-extras">
          <p className="cadastro-extras__hint">{copy.toggleBillExtrasHint}</p>
          <div className="cadastro-grid">
            <div className="cadastro-col-6">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-bill-print-holder">
                  {copy.fieldBillPrintedHolder}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-bill-print-holder"
                  placeholder={copy.placeholderBillPrintedHolder}
                  value={state.billPrintedHolder}
                  onChange={(e) => onChange({ billPrintedHolder: e.target.value })}
                />
              </div>
            </div>

            <div className="cadastro-col-6">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-bill-print-addr">
                  {copy.fieldBillPrintedAddress}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-bill-print-addr"
                  placeholder={copy.placeholderBillPrintedAddress}
                  value={state.billPrintedAddress}
                  onChange={(e) => onChange({ billPrintedAddress: e.target.value })}
                />
              </div>
            </div>
          </div>
          <RecordCustomFieldsSection
            definitionTargetId={state.billTypeId || undefined}
            definitionTargetKind="BILL_TYPE"
            disabled={isDisabled}
            draft={state.customDraft}
            onDraftChange={(customDraft) => onChange({ customDraft })}
          />
        </div>
      ) : null}
    </div>
  );
}
