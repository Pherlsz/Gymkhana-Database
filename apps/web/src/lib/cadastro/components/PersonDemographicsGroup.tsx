import { AutoComplete, Button, DatePicker, Input, Select } from "antd";
import dayjs from "dayjs";
import { ChevronDown, ChevronUp } from "lucide-react";
import { useState } from "react";
import { useI18n } from "../../../i18n";
import { BRAZIL_STATES } from "../../tables/tableFilters";
import {
  BLOOD_TYPE_OPTIONS,
  GENDER_OPTIONS,
  MARITAL_STATUS_OPTIONS,
  NATIONALITY_OPTIONS,
  toAutoCompleteOptions,
} from "../data/presetOptions";
import { phoneInputDigits } from "../cadastroValidate";

export interface PersonDemographicsState {
  fullName: string;
  socialName: string;
  cpf: string;
  birthDate?: string | undefined;
  email: string;
  phone: string;
  landline: string;
  street: string;
  number: string;
  complement: string;
  neighborhood: string;
  city: string;
  state: string;
  postalCode: string;
  gender?: string | undefined;
  maritalStatus?: string | undefined;
  bloodType?: string | undefined;
  nationality: string;
  birthCity: string;
  birthCountry: string;
  placeOfOrigin: string;
}

export interface PersonDemographicsProps {
  state: PersonDemographicsState;
  onChange: (patch: Partial<PersonDemographicsState>) => void;
  disabled?: boolean;
  hideName?: boolean;
  defaultShowMore?: boolean;
  copy?: Record<string, string>;
}

const filterOpt = (input: string, option?: { value?: string }) =>
  (option?.value?.toLowerCase() ?? "").includes(input.toLowerCase());

function maskCpf(val: string) {
  const digits = val.replace(/\D/g, "").slice(0, 11);
  if (digits.length <= 3) return digits;
  if (digits.length <= 6) return `${digits.slice(0, 3)}.${digits.slice(3)}`;
  if (digits.length <= 9) return `${digits.slice(0, 3)}.${digits.slice(3, 6)}.${digits.slice(6)}`;
  return `${digits.slice(0, 3)}.${digits.slice(3, 6)}.${digits.slice(6, 9)}-${digits.slice(9, 11)}`;
}

function maskPhone(val: string) {
  const digits = phoneInputDigits(val);
  if (!digits) return "";
  if (digits.length <= 2) return `(${digits}`;
  if (digits.length <= 6) return `(${digits.slice(0, 2)}) ${digits.slice(2)}`;
  if (digits.length <= 10)
    return `(${digits.slice(0, 2)}) ${digits.slice(2, 6)}-${digits.slice(6)}`;
  return `(${digits.slice(0, 2)}) ${digits.slice(2, 7)}-${digits.slice(7, 11)}`;
}

function maskCep(val: string) {
  const digits = val.replace(/\D/g, "").slice(0, 8);
  if (digits.length <= 5) return digits;
  return `${digits.slice(0, 5)}-${digits.slice(5, 8)}`;
}

export function PersonDemographicsGroup({
  state,
  onChange,
  disabled,
  hideName,
  defaultShowMore,
  copy: customCopy,
}: PersonDemographicsProps) {
  const { messages } = useI18n();
  const labels = messages.common.labels;
  const placeholders = messages.common.placeholders;
  const copy = { ...messages.tables.cadastro, ...customCopy };
  const [showMoreDetails, setShowMoreDetails] = useState(() => Boolean(defaultShowMore));
  const isDisabled = Boolean(disabled);

  const nationalityOptions = toAutoCompleteOptions(NATIONALITY_OPTIONS);
  const bloodTypeOptions = toAutoCompleteOptions(BLOOD_TYPE_OPTIONS);
  const maritalOptions = toAutoCompleteOptions(MARITAL_STATUS_OPTIONS);
  const genderOptions = toAutoCompleteOptions(GENDER_OPTIONS);
  const ufOptions = BRAZIL_STATES.map((value) => ({ value, label: value }));

  return (
    <>
      <div className="cadastro-grid">
        {hideName ? null : (
          <div className="cadastro-col-8">
            <div className="cadastro-field">
              <label className="cadastro-field__label" htmlFor="cad-p-holder">
                <span>{copy.fieldHolderName}</span>
                <span className="cadastro-field__required">*</span>
              </label>
              <Input
                disabled={isDisabled}
                id="cad-p-holder"
                placeholder={placeholders.holderName}
                value={state.fullName}
                onChange={(e) => onChange({ fullName: e.target.value })}
              />
            </div>
          </div>
        )}

        <div className={hideName ? "cadastro-col-12" : "cadastro-col-4"}>
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-cpf">
              {labels.cpf}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-cpf"
              maxLength={14}
              placeholder={placeholders.cpf}
              value={state.cpf}
              onChange={(e) => onChange({ cpf: maskCpf(e.target.value) })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-email">
              {labels.email}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-email"
              placeholder={placeholders.email}
              type="email"
              value={state.email}
              onChange={(e) => onChange({ email: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-3">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-phone">
              {copy.fieldPhone}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-phone"
              maxLength={15}
              placeholder={placeholders.phone}
              value={maskPhone(state.phone)}
              onChange={(e) => onChange({ phone: maskPhone(e.target.value) })}
            />
          </div>
        </div>

        <div className="cadastro-col-3">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-birth">
              {labels.birthDate}
            </label>
            <DatePicker
              disabled={isDisabled}
              format="DD/MM/YYYY"
              id="cad-p-birth"
              placeholder={copy.placeholderDate}
              style={{ width: "100%" }}
              value={state.birthDate ? dayjs(state.birthDate) : null}
              onChange={(d) => onChange({ birthDate: d ? d.format("YYYY-MM-DD") : undefined })}
            />
          </div>
        </div>

        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-street">
              {labels.streetAddress}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-street"
              placeholder={copy.placeholderAddress}
              value={state.street}
              onChange={(e) => onChange({ street: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-3">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-number">
              {labels.number}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-number"
              value={state.number}
              onChange={(e) => onChange({ number: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-3">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-complement">
              {labels.complement}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-complement"
              value={state.complement}
              onChange={(e) => onChange({ complement: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-neighborhood">
              {labels.neighborhood}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-neighborhood"
              value={state.neighborhood}
              onChange={(e) => onChange({ neighborhood: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-city">
              {labels.city}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-city"
              value={state.city}
              onChange={(e) => onChange({ city: e.target.value })}
            />
          </div>
        </div>

        <div className="cadastro-col-2">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-state">
              {labels.state}
            </label>
            <Select
              allowClear
              disabled={isDisabled}
              id="cad-p-state"
              options={ufOptions}
              placeholder={labels.state}
              style={{ width: "100%" }}
              value={state.state || undefined}
              onChange={(value) => onChange({ state: value ?? "" })}
            />
          </div>
        </div>

        <div className="cadastro-col-2">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-cep">
              {labels.cep}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-cep"
              maxLength={9}
              placeholder={placeholders.cep}
              value={state.postalCode}
              onChange={(e) => onChange({ postalCode: maskCep(e.target.value) })}
            />
          </div>
        </div>

        {showMoreDetails && (
          <>
            <div className="cadastro-col-6">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-p-soc">
                  {labels.socialName}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-p-soc"
                  value={state.socialName}
                  onChange={(e) => onChange({ socialName: e.target.value })}
                />
              </div>
            </div>

            <div className="cadastro-col-6">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-p-land">
                  {labels.landline}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-p-land"
                  placeholder={placeholders.landline}
                  value={state.landline}
                  onChange={(e) => onChange({ landline: e.target.value })}
                />
              </div>
            </div>

            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-gender">
                  {labels.gender}
                </label>
                <AutoComplete
                  allowClear
                  disabled={isDisabled}
                  filterOption={filterOpt}
                  id="cad-demo-gender"
                  options={genderOptions}
                  placeholder={labels.notProvided}
                  style={{ width: "100%" }}
                  value={state.gender || undefined}
                  onChange={(v) => onChange({ gender: v || undefined })}
                />
              </div>
            </div>

            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-marital">
                  {labels.maritalStatus}
                </label>
                <AutoComplete
                  allowClear
                  disabled={isDisabled}
                  filterOption={filterOpt}
                  id="cad-demo-marital"
                  options={maritalOptions}
                  placeholder={labels.maritalStatus}
                  style={{ width: "100%" }}
                  value={state.maritalStatus || undefined}
                  onChange={(v) => onChange({ maritalStatus: v || undefined })}
                />
              </div>
            </div>

            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-blood-type">
                  {labels.bloodType}
                </label>
                <AutoComplete
                  allowClear
                  disabled={isDisabled}
                  filterOption={filterOpt}
                  id="cad-demo-blood-type"
                  options={bloodTypeOptions}
                  placeholder={labels.bloodType}
                  style={{ width: "100%" }}
                  value={state.bloodType || undefined}
                  onChange={(v) => onChange({ bloodType: v || undefined })}
                />
              </div>
            </div>

            <div className="cadastro-col-3">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-nationality">
                  {labels.nationality}
                </label>
                <AutoComplete
                  allowClear
                  disabled={isDisabled}
                  filterOption={filterOpt}
                  id="cad-demo-nationality"
                  options={nationalityOptions}
                  placeholder={labels.nationality}
                  style={{ width: "100%" }}
                  value={state.nationality}
                  onChange={(val) => onChange({ nationality: val })}
                />
              </div>
            </div>

            <div className="cadastro-col-3">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-birth-city">
                  {labels.birthCity}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-demo-birth-city"
                  placeholder={labels.birthCity}
                  value={state.birthCity}
                  onChange={(e) => onChange({ birthCity: e.target.value })}
                />
              </div>
            </div>

            <div className="cadastro-col-3">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-birth-country">
                  {labels.birthCountry}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-demo-birth-country"
                  placeholder={labels.birthCountry}
                  value={state.birthCountry}
                  onChange={(e) => onChange({ birthCountry: e.target.value })}
                />
              </div>
            </div>

            <div className="cadastro-col-3">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-origin">
                  {labels.placeOfOrigin}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-demo-origin"
                  placeholder={labels.placeOfOrigin}
                  value={state.placeOfOrigin}
                  onChange={(e) => onChange({ placeOfOrigin: e.target.value })}
                />
              </div>
            </div>
          </>
        )}
      </div>

      <Button
        className="cadastro-toggle-btn"
        type="text"
        icon={
          showMoreDetails ? (
            <ChevronUp size={14} strokeWidth={2} />
          ) : (
            <ChevronDown size={14} strokeWidth={2} />
          )
        }
        onClick={() => setShowMoreDetails((prev) => !prev)}
      >
        {showMoreDetails ? copy.toggleLessDetails : copy.toggleMoreDetails}
      </Button>
    </>
  );
}
