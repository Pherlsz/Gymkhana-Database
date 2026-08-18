import type { CustomField } from "../api/client";
import { badgesFromRow, DocumentBadges } from "./documentBadges";
import { SHEET_COLUMN_WIDTH } from "./sheetDefaults";
import type { SpreadsheetColumn } from "./SpreadsheetTable";
import { DETAILS_FIELD_KEYS, PEOPLE_SHEET_DOC_KEYS, type TableRow } from "./tableRows";

export const PEOPLE_DOC_KEY_SET = new Set<string>(PEOPLE_SHEET_DOC_KEYS);
export const DETAILS_FIELD_KEY_SET = new Set<string>(DETAILS_FIELD_KEYS);

type ColumnCopy = {
  fullName: string;
  digitSumName: string;
  socialName: string;
  gender: string;
  birthDate: string;
  age: string;
  zodiacSign: string;
  bloodType: string;
  rg: string;
  digitSumRg: string;
  cpf: string;
  digitSumCpf: string;
  voterId: string;
  cnh: string;
  digitSumCnh: string;
  ctps: string;
  ctpsSeries: string;
  pis: string;
  crea: string;
  oab: string;
  studentId: string;
  susCard: string;
  citizenCard: string;
  passport: string;
  street: string;
  number: string;
  complement: string;
  neighborhood: string;
  city: string;
  state: string;
  postalCode: string;
  email: string;
  mobile: string;
  digitSumPhone: string;
  landline: string;
  maritalStatus: string;
  nationality: string;
  birthCity: string;
  weddingDate: string;
  fatherName: string;
  fatherBirthDate: string;
  motherName: string;
  motherBirthDate: string;
  vehicleModel: string;
  vehicleColor: string;
  vehiclePlate: string;
  vehicleYear: string;
  healthPlan: string;
  bloodDonor: string;
  organDonor: string;
  team: string;
  sector: string;
  collections: string;
  clubMembership: string;
  membershipType: string;
  placeOfOrigin: string;
  birthCountry: string;
  parentsWeddingDate: string;
  supermarketClub: string;
  pet: string;
  travelCountries: string;
  cardBrand: string;
  cardBank: string;
  documents: string;
  withOwner: string;
};

export function identityColumn(
  key: string,
  title: string,
  sortField: string,
): SpreadsheetColumn<TableRow> {
  return {
    key,
    title,
    label: title,
    width: SHEET_COLUMN_WIDTH.identity,
    sortField,
    dataIndex: key,
    className: "spreadsheet-table__identity",
  };
}

export function dataColumn(
  key: string,
  title: string,
  width: number,
  sortField?: string,
  dataIndex = key,
): SpreadsheetColumn<TableRow> {
  return {
    key,
    title,
    label: title,
    width,
    dataIndex,
    ...(sortField ? { sortField } : {}),
  };
}

export function extraColumn(field: CustomField): SpreadsheetColumn<TableRow> {
  return dataColumn(
    `custom:${field.technical_key}`,
    field.label,
    SHEET_COLUMN_WIDTH.default,
    undefined,
    field.technical_key,
  );
}

export function buildPeopleColumns(
  copy: ColumnCopy,
  extraFields: CustomField[],
): SpreadsheetColumn<TableRow>[] {
  const text = (key: string, title: string, width: number, sortField?: string) =>
    dataColumn(key, title, width, sortField);

  return [
    identityColumn("full_name", copy.fullName, "full_name"),
    {
      key: "documents",
      title: copy.documents,
      label: copy.documents,
      width: SHEET_COLUMN_WIDTH.documents,
      dataIndex: "document_badges",
      className: "spreadsheet-table__badges",
      render: (row) => (
        <DocumentBadges badges={badgesFromRow(row)} withOwnerLabel={copy.withOwner} />
      ),
    },
    dataColumn("cpf", copy.cpf, 140, "cpf"),
    text("rg", copy.rg, 140),
    text("street", copy.street, 180),
    text("number", copy.number, 80),
    text("city", copy.city, 160, "address_city"),
    text("postal_code", copy.postalCode, 110),
    text("email", copy.email, 200, "email"),
    text("mobile", copy.mobile, 140),
    text("birth_date", copy.birthDate, 150),
    text("voter_id", copy.voterId, 150),
    text("cnh", copy.cnh, 140),
    text("ctps", copy.ctps, 140),
    text("ctps_series", copy.ctpsSeries, 120),
    text("pis", copy.pis, 140),
    text("crea", copy.crea, 140),
    text("oab", copy.oab, 140),
    text("student_id", copy.studentId, 170),
    text("sus_card", copy.susCard, 150),
    text("citizen_card", copy.citizenCard, 150),
    text("passport", copy.passport, 140),
    text("team", copy.team, 140),
    text("sector", copy.sector, 140),
    text("club_membership", copy.clubMembership, 140),
    text("social_name", copy.socialName, 160),
    text("gender", copy.gender, 90),
    text("blood_type", copy.bloodType, 120),
    text("complement", copy.complement, 140),
    text("neighborhood", copy.neighborhood, 140),
    text("state", copy.state, 72),
    text("landline", copy.landline, 140),
    text("marital_status", copy.maritalStatus, 140),
    text("nationality", copy.nationality, 140),
    text("birth_city", copy.birthCity, 160),
    text("wedding_date", copy.weddingDate, SHEET_COLUMN_WIDTH.default),
    text("father_name", copy.fatherName, 180),
    text("father_birth_date", copy.fatherBirthDate, SHEET_COLUMN_WIDTH.default),
    text("mother_name", copy.motherName, 180),
    text("mother_birth_date", copy.motherBirthDate, SHEET_COLUMN_WIDTH.default),
    text("vehicle_model", copy.vehicleModel, 150),
    text("vehicle_color", copy.vehicleColor, 120),
    text("vehicle_plate", copy.vehiclePlate, 120),
    text("vehicle_year", copy.vehicleYear, 90),
    text("health_plan", copy.healthPlan, 160),
    text("blood_donor", copy.bloodDonor, SHEET_COLUMN_WIDTH.default),
    text("organ_donor", copy.organDonor, SHEET_COLUMN_WIDTH.default),
    text("collections", copy.collections, 160),
    text("membership_type", copy.membershipType, 150),
    text("place_of_origin", copy.placeOfOrigin, 160),
    text("birth_country", copy.birthCountry, 150),
    text("parents_wedding_date", copy.parentsWeddingDate, SHEET_COLUMN_WIDTH.default),
    text("supermarket_club", copy.supermarketClub, 170),
    text("pet", copy.pet, 140),
    text("travel_countries", copy.travelCountries, 180),
    text("card_brand", copy.cardBrand, 150),
    text("card_bank", copy.cardBank, 150),
    ...extraFields
      .filter(
        (field) =>
          !DETAILS_FIELD_KEY_SET.has(field.technical_key) &&
          !PEOPLE_DOC_KEY_SET.has(field.technical_key),
      )
      .map((field) => extraColumn(field)),
  ];
}
