import { AutoComplete, DatePicker, Input } from "antd";
import dayjs from "dayjs";
import { useState } from "react";
import {
  BLOOD_TYPE_OPTIONS,
  GENDER_OPTIONS,
  MARITAL_STATUS_OPTIONS,
  NATIONALITY_OPTIONS,
  toAutoCompleteOptions,
} from "../data/presetOptions";

export interface PersonDemographicsState {
  fullName: string;
  socialName: string;
  cpf: string;
  birthDate?: string | undefined;
  email: string;
  phone: string;
  landline: string;
  address: string;
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
  copy: {
    fieldHolderName: string;
    fieldHolderPlaceholder: string;
    fieldSocialName: string;
    fieldCpf: string;
    placeholderCpf: string;
    fieldBirthDate: string;
    fieldEmail: string;
    placeholderEmail: string;
    fieldPhone: string;
    placeholderPhone: string;
    fieldLandline: string;
    placeholderLandline: string;
    fieldAddress: string;
    placeholderAddress: string;
    fieldPostalCode: string;
    fieldGender: string;
    genderMale: string;
    genderFemale: string;
    genderOther: string;
    genderUninformed: string;
    fieldMaritalStatus: string;
    maritalSingle: string;
    maritalMarried: string;
    maritalDivorced: string;
    maritalWidowed: string;
    maritalCivilUnion: string;
    fieldBloodType: string;
    fieldNationality: string;
    fieldBirthCity: string;
    fieldBirthCountry: string;
    fieldPlaceOfOrigin: string;
    toggleMoreDetails: string;
    toggleLessDetails: string;
  };
}

const filterOpt = (input: string, option?: { value?: string }) =>
  (option?.value?.toLowerCase() ?? "").includes(input.toLowerCase());

export function PersonDemographicsGroup({
  state,
  onChange,
  disabled,
  copy,
}: PersonDemographicsProps) {
  const [showMoreDetails, setShowMoreDetails] = useState(false);
  const isDisabled = Boolean(disabled);

  const nationalityOptions = toAutoCompleteOptions(NATIONALITY_OPTIONS);
  const bloodTypeOptions = toAutoCompleteOptions(BLOOD_TYPE_OPTIONS);
  const maritalOptions = toAutoCompleteOptions(MARITAL_STATUS_OPTIONS);
  const genderOptions = toAutoCompleteOptions(GENDER_OPTIONS);

  return (
    <>
      <div className="cadastro-grid">
        {/* Nome Completo */}
        <div className="cadastro-col-8">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-holder">
              {copy.fieldHolderName} *
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-holder"
              placeholder={copy.fieldHolderPlaceholder}
              value={state.fullName}
              onChange={(e) => onChange({ fullName: e.target.value })}
            />
          </div>
        </div>

        {/* CPF */}
        <div className="cadastro-col-4">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-cpf">
              {copy.fieldCpf}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-cpf"
              placeholder={copy.placeholderCpf}
              value={state.cpf}
              onChange={(e) => onChange({ cpf: e.target.value })}
            />
          </div>
        </div>

        {/* Email */}
        <div className="cadastro-col-6">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-email">
              {copy.fieldEmail}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-email"
              placeholder={copy.placeholderEmail}
              type="email"
              value={state.email}
              onChange={(e) => onChange({ email: e.target.value })}
            />
          </div>
        </div>

        {/* Celular */}
        <div className="cadastro-col-3">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-phone">
              {copy.fieldPhone}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-phone"
              placeholder={copy.placeholderPhone}
              value={state.phone}
              onChange={(e) => onChange({ phone: e.target.value })}
            />
          </div>
        </div>

        {/* Data de Nascimento */}
        <div className="cadastro-col-3">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-birth">
              {copy.fieldBirthDate}
            </label>
            <DatePicker
              disabled={isDisabled}
              format="DD/MM/YYYY"
              id="cad-p-birth"
              style={{ width: "100%" }}
              value={state.birthDate ? dayjs(state.birthDate) : null}
              onChange={(d) => onChange({ birthDate: d ? d.format("YYYY-MM-DD") : undefined })}
            />
          </div>
        </div>

        {/* Endereço */}
        <div className="cadastro-col-9">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-addr">
              {copy.fieldAddress}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-addr"
              placeholder={copy.placeholderAddress}
              value={state.address}
              onChange={(e) => onChange({ address: e.target.value })}
            />
          </div>
        </div>

        {/* CEP */}
        <div className="cadastro-col-3">
          <div className="cadastro-field">
            <label className="cadastro-field__label" htmlFor="cad-p-cep">
              {copy.fieldPostalCode}
            </label>
            <Input
              disabled={isDisabled}
              id="cad-p-cep"
              value={state.postalCode}
              onChange={(e) => onChange({ postalCode: e.target.value })}
            />
          </div>
        </div>

        {/* Mais Detalhes (Demográficos e Identificação) */}
        {showMoreDetails && (
          <>
            <div className="cadastro-col-6">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-p-soc">
                  {copy.fieldSocialName}
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
                  {copy.fieldLandline}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-p-land"
                  placeholder={copy.placeholderLandline}
                  value={state.landline}
                  onChange={(e) => onChange({ landline: e.target.value })}
                />
              </div>
            </div>

            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-gender">
                  {copy.fieldGender}
                </label>
                <AutoComplete
                  allowClear
                  disabled={isDisabled}
                  filterOption={filterOpt}
                  id="cad-demo-gender"
                  options={genderOptions}
                  placeholder={copy.genderUninformed}
                  style={{ width: "100%" }}
                  value={state.gender || undefined}
                  onChange={(v) => onChange({ gender: v || undefined })}
                />
              </div>
            </div>

            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-marital">
                  {copy.fieldMaritalStatus}
                </label>
                <AutoComplete
                  allowClear
                  disabled={isDisabled}
                  filterOption={filterOpt}
                  id="cad-demo-marital"
                  options={maritalOptions}
                  placeholder={copy.fieldMaritalStatus}
                  style={{ width: "100%" }}
                  value={state.maritalStatus || undefined}
                  onChange={(v) => onChange({ maritalStatus: v || undefined })}
                />
              </div>
            </div>

            <div className="cadastro-col-4">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-blood-type">
                  {copy.fieldBloodType}
                </label>
                <AutoComplete
                  allowClear
                  disabled={isDisabled}
                  filterOption={filterOpt}
                  id="cad-demo-blood-type"
                  options={bloodTypeOptions}
                  placeholder={copy.fieldBloodType}
                  style={{ width: "100%" }}
                  value={state.bloodType || undefined}
                  onChange={(v) => onChange({ bloodType: v || undefined })}
                />
              </div>
            </div>

            <div className="cadastro-col-3">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-nationality">
                  {copy.fieldNationality}
                </label>
                <AutoComplete
                  allowClear
                  disabled={isDisabled}
                  filterOption={filterOpt}
                  id="cad-demo-nationality"
                  options={nationalityOptions}
                  placeholder={copy.fieldNationality}
                  style={{ width: "100%" }}
                  value={state.nationality}
                  onChange={(val) => onChange({ nationality: val })}
                />
              </div>
            </div>

            <div className="cadastro-col-3">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-birth-city">
                  {copy.fieldBirthCity}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-demo-birth-city"
                  placeholder={copy.fieldBirthCity}
                  value={state.birthCity}
                  onChange={(e) => onChange({ birthCity: e.target.value })}
                />
              </div>
            </div>

            <div className="cadastro-col-3">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-birth-country">
                  {copy.fieldBirthCountry}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-demo-birth-country"
                  placeholder={copy.fieldBirthCountry}
                  value={state.birthCountry}
                  onChange={(e) => onChange({ birthCountry: e.target.value })}
                />
              </div>
            </div>

            <div className="cadastro-col-3">
              <div className="cadastro-field">
                <label className="cadastro-field__label" htmlFor="cad-demo-origin">
                  {copy.fieldPlaceOfOrigin}
                </label>
                <Input
                  disabled={isDisabled}
                  id="cad-demo-origin"
                  placeholder={copy.fieldPlaceOfOrigin}
                  value={state.placeOfOrigin}
                  onChange={(e) => onChange({ placeOfOrigin: e.target.value })}
                />
              </div>
            </div>
          </>
        )}
      </div>

      <button
        className="cadastro-toggle-btn"
        type="button"
        onClick={() => setShowMoreDetails((prev) => !prev)}
      >
        {showMoreDetails ? copy.toggleLessDetails : copy.toggleMoreDetails}
      </button>
    </>
  );
}
