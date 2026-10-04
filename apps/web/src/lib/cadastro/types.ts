import type { CustomDraftValue } from "../../RecordCustomFields";
import type { BillFormFieldsState } from "./components/BillFormFields";
import type { DocumentFormFieldsState } from "./components/DocumentFormFields";
import type { PersonComplementaryState } from "./components/PersonComplementaryGroup";
import type { PersonDemographicsState } from "./components/PersonDemographicsGroup";
import type { PersonFamilyState } from "./components/PersonFamilyGroup";
import { OFFICIAL_DOCUMENT_TYPE_KEYS } from "./cadastroMinimumRequirement";

export type PendingTag = "manual" | "queued";

export interface PendingDoc {
  id: string;
  typeId: string;
  typeName: string;
  typeKey: string;
  number: string;
  date?: string | undefined;
  validUntil?: string | undefined;
  medium: "PHYSICAL" | "DIGITAL";
  custody?: "ORGANIZATION" | "OWNER" | undefined;
  notes: string;
  tag: PendingTag;
  customDraft: Record<string, CustomDraftValue>;
  file?: File | undefined;
  /** Set after the record is stored, so a later failure does not create it again. */
  savedRecordId?: string | undefined;
  saveStatus?: "saved" | "error" | undefined;
}

export interface PendingBill {
  id: string;
  typeId: string;
  serviceName: string;
  typeKey: string;
  installation: string;
  competence: string;
  amount: string;
  printedHolder?: string | undefined;
  printedAddress?: string | undefined;
  medium: "PHYSICAL" | "DIGITAL";
  notes: string;
  tag: PendingTag;
  customDraft: Record<string, CustomDraftValue>;
  file?: File | undefined;
  /** Set after the record is stored, so a later failure does not create it again. */
  savedRecordId?: string | undefined;
  saveStatus?: "saved" | "error" | undefined;
}

export const INITIAL_DEMOGRAPHICS: PersonDemographicsState = {
  fullName: "",
  socialName: "",
  cpf: "",
  birthDate: undefined,
  email: "",
  phone: "",
  landline: "",
  street: "",
  number: "",
  complement: "",
  neighborhood: "",
  city: "",
  state: "",
  postalCode: "",
  gender: undefined,
  maritalStatus: undefined,
  bloodType: undefined,
  nationality: "",
  birthCity: "",
  birthCountry: "",
  placeOfOrigin: "",
};

export const INITIAL_FAMILY: PersonFamilyState = {
  fatherName: "",
  fatherBirthDate: undefined,
  motherName: "",
  motherBirthDate: undefined,
  weddingDate: undefined,
  parentsWeddingDate: undefined,
};

export const INITIAL_COMPLEMENTARY: PersonComplementaryState = {
  vehicleModel: "",
  vehicleColor: "",
  vehiclePlate: "",
  vehicleYear: "",
  healthPlan: "",
  bloodDonor: undefined,
  organDonor: undefined,
  team: "",
  sector: "",
  clubMembership: "",
  membershipType: "",
  collections: "",
  pet: "",
  supermarketClub: "",
  travelCountries: "",
  cardBrand: "",
  cardBank: "",
};

export const INITIAL_DOC_FIELDS: DocumentFormFieldsState = {
  docTypeId: "",
  docIdentifier: "",
  docDate: undefined,
  docValidUntil: undefined,
  docMedium: "PHYSICAL",
  docCustody: "ORGANIZATION",
  docNotes: "",
  customDraft: {},
  file: undefined,
};

export const INITIAL_BILL_FIELDS: BillFormFieldsState = {
  billTypeId: "",
  billInstallation: "",
  billCompetence: "",
  billAmount: "",
  billPrintedHolder: "",
  billPrintedAddress: "",
  billMedium: "DIGITAL",
  billNotes: "",
  customDraft: {},
  file: undefined,
};

export function isOfficialDocKey(technicalKey: string): boolean {
  return (OFFICIAL_DOCUMENT_TYPE_KEYS as readonly string[]).includes(technicalKey);
}

export function isOfficialDoc(typeName: string): boolean {
  const norm = typeName
    .toLowerCase()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "");
  return ["cpf", "rg", "cnh", "certid", "nascimento", "casamento"].some((pattern) =>
    norm.includes(pattern),
  );
}

export interface CadastroSingleScreenProps {
  targetTable?: "people" | "documents" | "bills";
  typeId?: string | undefined;
  onCancel: () => void;
  onSuccess?: (savedName: string) => void;
}
