import { AutoComplete, Input, Select } from "antd";
import { useI18n } from "../../../i18n";
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
  copy?: Record<string, string>;
}

const filterOpt = (input: string, option?: { value?: string }) =>
  (option?.value?.toLowerCase() ?? "").includes(input.toLowerCase());

export function PersonComplementaryGroup({
  state,
  onChange,
  disabled,
}: PersonComplementaryGroupProps) {
  const { messages } = useI18n();
  const labels = messages.common.labels;

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
      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-vehicle-model">
            {labels.vehicleModel}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-vehicle-model"
            placeholder={labels.vehicleModel}
            value={state.vehicleModel}
            onChange={(e) => onChange({ vehicleModel: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-vehicle-color">
            {labels.vehicleColor}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-vehicle-color"
            options={vehicleColorOpts}
            placeholder={labels.vehicleColor}
            style={{ width: "100%" }}
            value={state.vehicleColor}
            onChange={(val) => onChange({ vehicleColor: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-vehicle-plate">
            {labels.vehiclePlate}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-vehicle-plate"
            placeholder={labels.vehiclePlate}
            value={state.vehiclePlate}
            onChange={(e) => onChange({ vehiclePlate: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-vehicle-year">
            {labels.vehicleYear}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-vehicle-year"
            placeholder={labels.vehicleYear}
            value={state.vehicleYear}
            onChange={(e) => onChange({ vehicleYear: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-health-plan">
            {labels.healthPlan}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-health-plan"
            options={healthPlanOpts}
            placeholder={labels.healthPlan}
            style={{ width: "100%" }}
            value={state.healthPlan}
            onChange={(val) => onChange({ healthPlan: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-blood-donor">
            {labels.bloodDonor}
          </label>
          <Select
            allowClear
            disabled={isDisabled}
            id="cad-blood-donor"
            placeholder={labels.notProvided}
            style={{ width: "100%" }}
            value={state.bloodDonor === null ? undefined : state.bloodDonor ? "true" : "false"}
            onChange={(val) =>
              onChange({
                bloodDonor: val === "true" ? true : val === "false" ? false : null,
              })
            }
            options={[
              { value: "true", label: labels.yes },
              { value: "false", label: labels.no },
            ]}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-organ-donor">
            {labels.organDonor}
          </label>
          <Select
            allowClear
            disabled={isDisabled}
            id="cad-organ-donor"
            placeholder={labels.notProvided}
            style={{ width: "100%" }}
            value={state.organDonor === null ? undefined : state.organDonor ? "true" : "false"}
            onChange={(val) =>
              onChange({
                organDonor: val === "true" ? true : val === "false" ? false : null,
              })
            }
            options={[
              { value: "true", label: labels.yes },
              { value: "false", label: labels.no },
            ]}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-team">
            {labels.team}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-team"
            options={teamOpts}
            placeholder={labels.team}
            style={{ width: "100%" }}
            value={state.team}
            onChange={(val) => onChange({ team: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-sector">
            {labels.sector}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-sector"
            options={sectorOpts}
            placeholder={labels.sector}
            style={{ width: "100%" }}
            value={state.sector}
            onChange={(val) => onChange({ sector: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-club">
            {labels.clubMembership}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-club"
            options={clubOpts}
            placeholder={labels.clubMembership}
            style={{ width: "100%" }}
            value={state.clubMembership}
            onChange={(val) => onChange({ clubMembership: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-membership-type">
            {labels.membershipType}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-membership-type"
            options={membershipTypeOpts}
            placeholder={labels.membershipType}
            style={{ width: "100%" }}
            value={state.membershipType}
            onChange={(val) => onChange({ membershipType: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-collections">
            {labels.collections}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-collections"
            placeholder={labels.collections}
            value={state.collections}
            onChange={(e) => onChange({ collections: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-4">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-pet">
            {labels.pet}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-pet"
            options={petOpts}
            placeholder={labels.pet}
            style={{ width: "100%" }}
            value={state.pet}
            onChange={(val) => onChange({ pet: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-supermarket">
            {labels.supermarketClub}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-supermarket"
            options={supermarketOpts}
            placeholder={labels.supermarketClub}
            style={{ width: "100%" }}
            value={state.supermarketClub}
            onChange={(val) => onChange({ supermarketClub: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-travel-countries">
            {labels.travelCountries}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-travel-countries"
            placeholder={labels.travelCountries}
            value={state.travelCountries}
            onChange={(e) => onChange({ travelCountries: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-card-brand">
            {labels.cardBrand}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-card-brand"
            options={cardBrandOpts}
            placeholder={labels.cardBrand}
            style={{ width: "100%" }}
            value={state.cardBrand}
            onChange={(val) => onChange({ cardBrand: val })}
          />
        </div>
      </div>

      <div className="cadastro-col-3">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-card-bank">
            {labels.cardBank}
          </label>
          <AutoComplete
            allowClear
            disabled={isDisabled}
            filterOption={filterOpt}
            id="cad-card-bank"
            options={cardBankOpts}
            placeholder={labels.cardBank}
            style={{ width: "100%" }}
            value={state.cardBank}
            onChange={(val) => onChange({ cardBank: val })}
          />
        </div>
      </div>
    </div>
  );
}
