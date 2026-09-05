import type { BillFormFieldsState } from "./components/BillFormFields";
import type { DocumentFormFieldsState } from "./components/DocumentFormFields";
import type { PersonComplementaryState } from "./components/PersonComplementaryGroup";
import type { PersonDemographicsState } from "./components/PersonDemographicsGroup";
import type { PersonFamilyState } from "./components/PersonFamilyGroup";

export interface PendingDoc {
  id: string;
  typeId: string;
  typeName: string;
  number: string;
  date?: string | undefined;
  validUntil?: string | undefined;
  medium: "PHYSICAL" | "DIGITAL";
  custody?: "ORGANIZATION" | "OWNER" | undefined;
  notes: string;
  tag: "manual" | "ocr";
}

export interface PendingBill {
  id: string;
  typeId: string;
  serviceName: string;
  provider: string;
  installation: string;
  competence: string;
  dueDate?: string | undefined;
  amount: string;
  printedHolder?: string | undefined;
  printedAddress?: string | undefined;
  medium: "PHYSICAL" | "DIGITAL";
  notes: string;
  tag: "manual" | "ocr";
}

export interface InlineDocState {
  typeId: string;
  number: string;
  notes: string;
  date?: string | undefined;
  validUntil?: string | undefined;
  medium: "PHYSICAL" | "DIGITAL";
  custody: "ORGANIZATION" | "OWNER";
}

export interface InlineBillState {
  typeId: string;
  provider: string;
  installation: string;
  competence: string;
  amount: string;
}

export const INITIAL_DEMOGRAPHICS: PersonDemographicsState = {
  fullName: "",
  socialName: "",
  cpf: "",
  birthDate: undefined,
  email: "",
  phone: "",
  landline: "",
  address: "",
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
};

export const INITIAL_BILL_FIELDS: BillFormFieldsState = {
  billTypeId: "",
  billProvider: "",
  billInstallation: "",
  billCompetence: "",
  billDueDate: undefined,
  billAmount: "",
  billPrintedHolder: "",
  billPrintedAddress: "",
  billMedium: "DIGITAL",
  billNotes: "",
};

export const INITIAL_INLINE_DOC: InlineDocState = {
  typeId: "",
  number: "",
  notes: "",
  date: undefined,
  validUntil: undefined,
  medium: "PHYSICAL",
  custody: "ORGANIZATION",
};

export const INITIAL_INLINE_BILL: InlineBillState = {
  typeId: "",
  provider: "",
  installation: "",
  competence: "",
  amount: "",
};

const OFFICIAL_DOC_PATTERNS = ["cpf", "rg", "cnh", "certid", "nascimento", "casamento"];

export function isOfficialDoc(typeName: string): boolean {
  const norm = typeName
    .toLowerCase()
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "");
  return OFFICIAL_DOC_PATTERNS.some((pattern) => norm.includes(pattern));
}

export interface CadastroSingleScreenProps {
  targetTable?: "people" | "documents" | "bills";
  onCancel: () => void;
  onSuccess?: (savedName: string) => void;
}
