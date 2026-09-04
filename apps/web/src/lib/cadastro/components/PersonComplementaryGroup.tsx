import { AutoComplete, Input, Select } from "antd";
import {
  CARD_BANK_OPTIONS,
  CARD_BRAND_OPTIONS,
  CLUB_MEMBERSHIP_OPTIONS,
  HEALTH_PLAN_OPTIONS,
  MEMBERSHIP_TYPE_OPTIONS,
  PET_OPTIONS,
  SECTOR_OPTIONS,
  SUPERMARKET_CLUB_OPTIONS,
  TEAM_OPTIONS,
  VEHICLE_COLOR_OPTIONS,
  toAutoCompleteOptions,
} from "../data/presetOptions";

export interface PersonComplementaryState {
  vehicleModel: string;
  vehicleColor: string;
  vehiclePlate: string;
  vehicleYear: string;
  healthPlan: string;
  bloodDonor?: boolean | null | undefined;
  organDonor?: boolean | null | undefined;
  team: string;
  sector: string;
  clubMembership: string;
  membershipType: string;
  collections: string;
  pet: string;
  supermarketClub: string;
  travelCountries: string;
  cardBrand: string;
  cardBank: string;
}

export interface PersonComplementaryGroupProps {
  state: PersonComplementaryState;
  onChange: (patch: Partial<PersonComplementaryState>) => void;
  disabled?: boolean;
  copy: {
    fieldVehicleModel: string;
    fieldVehicleColor: string;
    fieldVehiclePlate: string;
    fieldVehicleYear: string;
    fieldHealthPlan: string;
    fieldBloodDonor: string;
    fieldOrganDonor: string;
    fieldTeam: string;
    fieldSector: string;
    fieldClubMembership: string;
    fieldMembershipType: string;
    fieldCollections: string;
    fieldPet: string;
    fieldSupermarketClub: string;
    fieldTravelCountries: string;
    fieldCardBrand: string;
    fieldCardBank: string;
  };
}

const filterOpt = (input: string, option?: { value?: string }) =>
  (option?.value?.toLowerCase() ?? "").includes(input.toLowerCase());

export function PersonComplementaryGroup({
  state,
  onChange,
  disabled,
  copy,
}: PersonComplementaryGroupProps) {
  const vehicleColorOpts = toAutoCompleteOptions(VEHICLE_COLOR_OPTIONS);
  const healthPlanOpts = toAutoCompleteOptions(HEALTH_PLAN_OPTIONS);
  const teamOpts = toAutoCompleteOptions(TEAM_OPTIONS);
  const sectorOpts = toAutoCompleteOptions(SECTOR_OPTIONS);
  const clubOpts = toAutoCompleteOptions(CLUB_MEMBERSHIP_OPTIONS);
  const membershipTypeOpts = toAutoCompleteOptions(MEMBERSHIP_TYPE_OPTIONS);
  const cardBrandOpts = toAutoCompleteOptions(CARD_BRAND_OPTIONS);
  const cardBankOpts = toAutoCompleteOptions(CARD_BANK_OPTIONS);
  const supermarketOpts = toAutoCompleteOptions(SUPERMARKET_CLUB_OPTIONS);
  const petOpts = toAutoCompleteOptions(PET_OPTIONS);

  const isDisabled = Boolean(disabled);

  return (
    <div className="cadastro-grid">
      {/* Subseção: Veículo */}
      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-vehicle-model">
            {copy.fieldVehicleModel}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-vehicle-model"
            placeholder={copy.fieldVehicleModel}
            value={state.vehicleModel}
            onChange={(e) => onChange({ vehicleModel: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-vehicle-color">
            {copy.fieldVehicleColor}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-vehicle-color"
            options={vehicleColorOpts}
            placeholder={copy.fieldVehicleColor}
            style={{ width: "100%" }}
            value={state.vehicleColor}
            onChange={(val) => onChange({ vehicleColor: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-vehicle-plate">
            {copy.fieldVehiclePlate}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-vehicle-plate"
            placeholder={copy.fieldVehiclePlate}
            value={state.vehiclePlate}
            onChange={(e) => onChange({ vehiclePlate: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-vehicle-year">
            {copy.fieldVehicleYear}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-vehicle-year"
            placeholder={copy.fieldVehicleYear}
            value={state.vehicleYear}
            onChange={(e) => onChange({ vehicleYear: e.target.value })}
          />
        </div>
      </div>

      {/* Subseção: Saúde */}
      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-health-plan">
            {copy.fieldHealthPlan}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-health-plan"
            options={healthPlanOpts}
            placeholder={copy.fieldHealthPlan}
            style={{ width: "100%" }}
            value={state.healthPlan}
            onChange={(val) => onChange({ healthPlan: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-blood-donor">
            {copy.fieldBloodDonor}
          </label>
          <Select
            allowClear
            disabled={isDisabled}
            id="cad-blood-donor"
            placeholder="Não informado"
            style={{ width: "100%" }}
            value={state.bloodDonor === null ? undefined : state.bloodDonor ? "true" : "false"}
            onChange={(val) =>
              onChange({
                bloodDonor: val === "true" ? true : val === "false" ? false : null,
              })
            }
            options={[
              { value: "true", label: "Sim" },
              { value: "false", label: "Não" },
            ]}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-organ-donor">
            {copy.fieldOrganDonor}
          </label>
          <Select
            allowClear
            disabled={isDisabled}
            id="cad-organ-donor"
            placeholder="Não informado"
            style={{ width: "100%" }}
            value={state.organDonor === null ? undefined : state.organDonor ? "true" : "false"}
            onChange={(val) =>
              onChange({
                organDonor: val === "true" ? true : val === "false" ? false : null,
              })
            }
            options={[
              { value: "true", label: "Sim" },
              { value: "false", label: "Não" },
            ]}
          />
        </div>
      </div>

      {/* Subseção: Gincana e Equipe */}
      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-team">
            {copy.fieldTeam}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-team"
            options={teamOpts}
            placeholder={copy.fieldTeam}
            style={{ width: "100%" }}
            value={state.team}
            onChange={(val) => onChange({ team: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-sector">
            {copy.fieldSector}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-sector"
            options={sectorOpts}
            placeholder={copy.fieldSector}
            style={{ width: "100%" }}
            value={state.sector}
            onChange={(val) => onChange({ sector: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-club">
            {copy.fieldClubMembership}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-club"
            options={clubOpts}
            placeholder={copy.fieldClubMembership}
            style={{ width: "100%" }}
            value={state.clubMembership}
            onChange={(val) => onChange({ clubMembership: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-membership-type">
            {copy.fieldMembershipType}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-membership-type"
            options={membershipTypeOpts}
            placeholder={copy.fieldMembershipType}
            style={{ width: "100%" }}
            value={state.membershipType}
            onChange={(val) => onChange({ membershipType: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-collections">
            {copy.fieldCollections}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-collections"
            placeholder={copy.fieldCollections}
            value={state.collections}
            onChange={(e) => onChange({ collections: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-pet">
            {copy.fieldPet}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-pet"
            options={petOpts}
            placeholder={copy.fieldPet}
            style={{ width: "100%" }}
            value={state.pet}
            onChange={(val) => onChange({ pet: val })}
          />
        </div>
      </div>

      {/* Subseção: Diversos / Financeiro */}
      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-supermarket">
            {copy.fieldSupermarketClub}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-supermarket"
            options={supermarketOpts}
            placeholder={copy.fieldSupermarketClub}
            style={{ width: "100%" }}
            value={state.supermarketClub}
            onChange={(val) => onChange({ supermarketClub: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-travel-countries">
            {copy.fieldTravelCountries}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-travel-countries"
            placeholder={copy.fieldTravelCountries}
            value={state.travelCountries}
            onChange={(e) => onChange({ travelCountries: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-card-brand">
            {copy.fieldCardBrand}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-card-brand"
            options={cardBrandOpts}
            placeholder={copy.fieldCardBrand}
            style={{ width: "100%" }}
            value={state.cardBrand}
            onChange={(val) => onChange({ cardBrand: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-card-bank">
            {copy.fieldCardBank}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-card-bank"
            options={cardBankOpts}
            placeholder={copy.fieldCardBank}
            style={{ width: "100%" }}
            value={state.cardBank}
            onChange={(val) => onChange({ cardBank: val })}
          />
        </div>
      </div>
    </div>
  );
}
