import { DatePicker, Input, Segmented, Select } from "antd";
import dayjs from "dayjs";
import type { ChangeEvent } from "react";
import { useI18n } from "../../../i18n";
import type { BillType } from "../../api/client";
import { OcrDropzoneInline } from "./OcrDropzoneInline";

export interface BillFormFieldsState {
  billTypeId: string;
  billProvider: string;
  billInstallation: string;
  billCompetence: string;
  billDueDate?: string | undefined;
  billAmount: string;
  billPrintedHolder: string;
  billPrintedAddress: string;
  billMedium: "PHYSICAL" | "DIGITAL";
  billNotes: string;
}

export interface BillFormFieldsProps {
  state: BillFormFieldsState;
  onChange: (patch: Partial<BillFormFieldsState>) => void;
  billTypes: BillType[];
  onFileDrop: (event: ChangeEvent<HTMLInputElement>) => void;
  fileInputId: string;
  copy?: Record<string, string>;
}

export function BillFormFields({
  state,
  onChange,
  billTypes,
  onFileDrop,
  fileInputId,
  copy: customCopy,
}: BillFormFieldsProps) {
  const { messages } = useI18n();
  const labels = messages.common.labels;
  const copy = { ...messages.tables.cadastro, ...customCopy };
  return (
    <div className="cadastro-bill-fields">
      <div className="cadastro-grid">
        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-type">
              {copy.fieldBillService} <span className="cadastro-field__required">*</span>
            </label>
            <Select
              id="cad-bill-type"
              options={billTypes.map((t) => ({ value: t.id, label: t.label }))}
              placeholder={copy.pickerSelectType}
              style={{ width: "100%" }}
              value={state.billTypeId || billTypes[0]?.id}
              onChange={(v) => {
                if (v) onChange({ billTypeId: v });
              }}
            />
          </div>
        </div>

        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-prov">
              {copy.fieldBillProvider} <span className="cadastro-field__required">*</span>
            </label>
            <Input
              id="cad-bill-prov"
              placeholder={copy.placeholderBillProvider}
              value={state.billProvider}
              onChange={(e) => onChange({ billProvider: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-inst">
              {copy.fieldBillInstallation} <span className="cadastro-field__required">*</span>
            </label>
            <Input
              id="cad-bill-inst"
              placeholder={copy.placeholderBillInstallation}
              value={state.billInstallation}
              onChange={(e) => onChange({ billInstallation: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-comp">
              {copy.fieldBillCompetence}
            </label>
            <Input
              id="cad-bill-comp"
              placeholder={copy.placeholderBillCompetence}
              value={state.billCompetence}
              onChange={(e) => onChange({ billCompetence: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-due">
              {copy.fieldBillDueDate}
            </label>
            <DatePicker
              format="DD/MM/YYYY"
              id="cad-bill-due"
              placeholder={copy.placeholderDate}
              style={{ width: "100%" }}
              value={state.billDueDate ? dayjs(state.billDueDate) : null}
              onChange={(d) => onChange({ billDueDate: d ? d.format("YYYY-MM-DD") : undefined })}
            />
          </div>
        </div>

        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-val">
              {copy.fieldBillAmount}
            </label>
            <Input
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
            <label className="cadastro-field__label" htmlFor="cad-bill-print-holder">
              {copy.fieldBillPrintedHolder}
            </label>
            <Input
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
              id="cad-bill-print-addr"
              placeholder={copy.placeholderBillPrintedAddress}
              value={state.billPrintedAddress}
              onChange={(e) => onChange({ billPrintedAddress: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label">{labels.medium}</label>
            <Segmented
              block
              options={[
                { label: labels.physical, value: "PHYSICAL" },
                { label: labels.digital, value: "DIGITAL" },
              ]}
              value={state.billMedium}
              onChange={(v) => onChange({ billMedium: v as "PHYSICAL" | "DIGITAL" })}
            />
          </div>
        </div>

        <div className="cadastro-col-8">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-notes">
              {labels.notes}
            </label>
            <Input
              id="cad-bill-notes"
              placeholder={copy.placeholderBillNotes}
              value={state.billNotes}
              onChange={(e) => onChange({ billNotes: e.target.value })}
            />
          </div>
        </div>
      </div>

      <OcrDropzoneInline
        inputId={fileInputId}
        label={copy.ocrInlineDropzoneBill}
        onFile={onFileDrop}
      />
    </div>
  );
}
