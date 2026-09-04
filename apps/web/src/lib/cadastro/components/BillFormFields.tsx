import { DatePicker, Input, Segmented, Select } from "antd";
import dayjs from "dayjs";
import type { ChangeEvent } from "react";
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
  copy: {
    fieldBillService: string;
    fieldBillProvider: string;
    placeholderBillProvider: string;
    fieldBillInstallation: string;
    placeholderBillInstallation: string;
    fieldBillCompetence: string;
    placeholderBillCompetence: string;
    fieldBillDueDate: string;
    fieldBillAmount: string;
    placeholderBillAmount: string;
    fieldBillPrintedHolder: string;
    placeholderBillPrintedHolder: string;
    fieldBillPrintedAddress: string;
    placeholderBillPrintedAddress: string;
    fieldDocMedium: string;
    tagPhysical: string;
    tagDigital: string;
    fieldDocNotes: string;
    placeholderBillNotes: string;
    ocrInlineDropzoneBill: string;
    pickerSelectType: string;
  };
}

export function BillFormFields({
  state,
  onChange,
  billTypes,
  onFileDrop,
  fileInputId,
  copy,
}: BillFormFieldsProps) {
  return (
    <div className="cadastro-bill-fields">
      <div className="cadastro-grid">
        {/* Tipo de Serviço */}
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

        {/* Fornecedor / Concessionária */}
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

        {/* Instalação / Código do Cliente */}
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

        {/* Competência / Mês */}
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

        {/* Vencimento */}
        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-due">
              {copy.fieldBillDueDate}
            </label>
            <DatePicker
              format="DD/MM/YYYY"
              id="cad-bill-due"
              style={{ width: "100%" }}
              value={state.billDueDate ? dayjs(state.billDueDate) : null}
              onChange={(d) => onChange({ billDueDate: d ? d.format("YYYY-MM-DD") : undefined })}
            />
          </div>
        </div>

        {/* Valor (R$) */}
        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-val">
              {copy.fieldBillAmount}
            </label>
            <Input
              id="cad-bill-val"
              placeholder={copy.placeholderBillAmount}
              value={state.billAmount}
              onChange={(e) => onChange({ billAmount: e.target.value })}
            />
          </div>
        </div>

        {/* Nome impresso no boleto */}
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

        {/* Endereço impresso */}
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

        {/* Meio (Físico / Digital) */}
        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label">{copy.fieldDocMedium}</label>
            <Segmented
              block
              options={[
                { label: copy.tagPhysical, value: "PHYSICAL" },
                { label: copy.tagDigital, value: "DIGITAL" },
              ]}
              value={state.billMedium}
              onChange={(v) => onChange({ billMedium: v as "PHYSICAL" | "DIGITAL" })}
            />
          </div>
        </div>

        {/* Observações */}
        <div className="cadastro-col-8">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-bill-notes">
              {copy.fieldDocNotes}
            </label>
            <Input.TextArea
              id="cad-bill-notes"
              placeholder={copy.placeholderBillNotes}
              rows={2}
              value={state.billNotes}
              onChange={(e) => onChange({ billNotes: e.target.value })}
            />
          </div>
        </div>
      </div>

      {/* Inline OCR Dropzone */}
      <OcrDropzoneInline
        inputId={fileInputId}
        label={copy.ocrInlineDropzoneBill}
        onFile={onFileDrop}
      />
    </div>
  );
}
