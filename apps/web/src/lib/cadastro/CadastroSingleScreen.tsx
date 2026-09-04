import { Button, DatePicker, Input, Segmented, Select, message } from "antd";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Car,
  Check,
  ChevronDown,
  FileText,
  Plus,
  Trash2,
  User,
  Users,
  Zap,
} from "lucide-react";
import { type ChangeEvent, useId, useMemo, useState } from "react";
import dayjs from "dayjs";
import { useI18n } from "../../i18n";
import {
  type BillType,
  type DocumentType,
  type Profile,
  createBill,
  createDocument,
  createProfile,
  listBillTypes,
  listDocumentTypes,
} from "../api/client";
import { MINIMUM_REQUIREMENT_QUERY } from "./CadastroMinimumRequirement";
import { announceSaved } from "./cadastroFeedback";
import {
  BillFormFields,
  CadastroStickyBar,
  DocumentFormFields,
  HolderSearchSelect,
  HolderSelectedCard,
  OcrDropzoneInline,
  PersonComplementaryGroup,
  PersonDemographicsGroup,
  PersonFamilyGroup,
} from "./components";

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

const OFFICIAL_DOC_PATTERNS = ["cpf", "rg", "cnh", "certid", "nascimento", "casamento"];

function isOfficialDoc(typeName: string): boolean {
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

export function CadastroSingleScreen({
  targetTable = "people",
  onCancel,
  onSuccess,
}: CadastroSingleScreenProps) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const queryClient = useQueryClient();

  const mode = targetTable;

  // Types queries
  const documentTypes = useQuery({
    queryKey: ["document-types"],
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const billTypes = useQuery({
    queryKey: ["bill-types"],
    queryFn: ({ signal }) => listBillTypes(signal),
  });

  // Section collapse states
  const [openPrimary, setOpenPrimary] = useState(true);
  const [openHolder, setOpenHolder] = useState(true);
  const [openDocs, setOpenDocs] = useState(true);
  const [openBills, setOpenBills] = useState(true);

  // Titular / Profile state
  const [holderName, setHolderName] = useState("");
  const [selectedProfile, setSelectedProfile] = useState<Profile | null>(null);
  const [cpf, setCpf] = useState("");
  const [birthDate, setBirthDate] = useState<string | undefined>(undefined);
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [address, setAddress] = useState("");
  const [postalCode, setPostalCode] = useState("");
  const [socialName, setSocialName] = useState("");
  const [landline, setLandline] = useState("");

  // Extra Personal / Demographics
  const [gender, setGender] = useState<string | undefined>(undefined);
  const [maritalStatus, setMaritalStatus] = useState<string | undefined>(undefined);
  const [bloodType, setBloodType] = useState<string | undefined>(undefined);
  const [nationality, setNationality] = useState("");
  const [birthCity, setBirthCity] = useState("");
  const [birthCountry, setBirthCountry] = useState("");
  const [placeOfOrigin, setPlaceOfOrigin] = useState("");

  // Family / Filiação
  const [openFamily, setOpenFamily] = useState(false);
  const [fatherName, setFatherName] = useState("");
  const [fatherBirthDate, setFatherBirthDate] = useState<string | undefined>(undefined);
  const [motherName, setMotherName] = useState("");
  const [motherBirthDate, setMotherBirthDate] = useState<string | undefined>(undefined);
  const [weddingDate, setWeddingDate] = useState<string | undefined>(undefined);
  const [parentsWeddingDate, setParentsWeddingDate] = useState<string | undefined>(undefined);

  // Complementary / Vehicle / Health / Gymkhana
  const [openComplementary, setOpenComplementary] = useState(false);
  const [vehicleModel, setVehicleModel] = useState("");
  const [vehicleColor, setVehicleColor] = useState("");
  const [vehiclePlate, setVehiclePlate] = useState("");
  const [vehicleYear, setVehicleYear] = useState("");
  const [healthPlan, setHealthPlan] = useState("");
  const [bloodDonor, setBloodDonor] = useState<boolean | undefined>(undefined);
  const [organDonor, setOrganDonor] = useState<boolean | undefined>(undefined);
  const [team, setTeam] = useState("");
  const [sector, setSector] = useState("");
  const [clubMembership, setClubMembership] = useState("");
  const [membershipType, setMembershipType] = useState("");
  const [collections, setCollections] = useState("");
  const [pet, setPet] = useState("");
  const [supermarketClub, setSupermarketClub] = useState("");
  const [travelCountries, setTravelCountries] = useState("");
  const [cardBrand, setCardBrand] = useState("");
  const [cardBank, setCardBank] = useState("");

  // Standalone Document Mode State (when mode === "documents")
  const [docTypeId, setDocTypeId] = useState<string>("");
  const [docNumber, setDocNumber] = useState("");
  const [docDate, setDocDate] = useState<string | undefined>(undefined);
  const [docValidUntil, setDocValidUntil] = useState<string | undefined>(undefined);
  const [docMedium, setDocMedium] = useState<"PHYSICAL" | "DIGITAL">("PHYSICAL");
  const [docCustody, setDocCustody] = useState<"ORGANIZATION" | "OWNER">("ORGANIZATION");
  const [docNotes, setDocNotes] = useState("");

  // Standalone Bill Mode State (when mode === "bills")
  const [billTypeId, setBillTypeId] = useState<string>("");
  const [billProvider, setBillProvider] = useState("");
  const [billInstallation, setBillInstallation] = useState("");
  const [billCompetence, setBillCompetence] = useState("");
  const [billDueDate, setBillDueDate] = useState<string | undefined>(undefined);
  const [billAmount, setBillAmount] = useState("");
  const [billPrintedHolder, setBillPrintedHolder] = useState("");
  const [billPrintedAddress, setBillPrintedAddress] = useState("");
  const [billMedium, setBillMedium] = useState<"PHYSICAL" | "DIGITAL">("DIGITAL");
  const [billNotes, setBillNotes] = useState("");

  // Staged Documents & Bills in Person mode
  const [documents, setDocuments] = useState<PendingDoc[]>([]);
  const [bills, setBills] = useState<PendingBill[]>([]);

  // Inline Add Form states (in Person Mode)
  const [addingDoc, setAddingDoc] = useState(false);
  const [inlineDocTypeId, setInlineDocTypeId] = useState<string>("");
  const [inlineDocNumber, setInlineDocNumber] = useState("");
  const [inlineDocNotes, setInlineDocNotes] = useState("");
  const [inlineDocDate, setInlineDocDate] = useState<string | undefined>(undefined);
  const [inlineDocValidUntil, setInlineDocValidUntil] = useState<string | undefined>(undefined);
  const [inlineDocMedium, setInlineDocMedium] = useState<"PHYSICAL" | "DIGITAL">("PHYSICAL");
  const [inlineDocCustody, setInlineDocCustody] = useState<"ORGANIZATION" | "OWNER">(
    "ORGANIZATION",
  );

  const [addingBill, setAddingBill] = useState(false);
  const [inlineBillTypeId, setInlineBillTypeId] = useState<string>("");
  const [inlineBillProvider, setInlineBillProvider] = useState("");
  const [inlineBillInstallation, setInlineBillInstallation] = useState("");
  const [inlineBillCompetence, setInlineBillCompetence] = useState("");
  const [inlineBillAmount, setInlineBillAmount] = useState("");

  const [saving, setSaving] = useState(false);

  // File input IDs
  const standaloneDocFileId = useId();
  const standaloneBillFileId = useId();
  const inlineDocFileId = useId();
  const inlineBillFileId = useId();

  // Apply a selected profile's data across form state
  const applyProfileToState = (p: Profile) => {
    setSelectedProfile(p);
    setHolderName(p.full_name);
    setCpf(p.cpf || "");
    setEmail(p.email || "");
    setPhone(p.mobile_phone || "");
    setLandline(p.landline_phone || "");
    setSocialName(p.social_name || "");
    if (p.birth_date) setBirthDate(p.birth_date);
    if (p.gender) setGender(p.gender);
    if (p.marital_status) setMaritalStatus(p.marital_status);
    if (p.blood_type) setBloodType(p.blood_type);
    if (p.nationality) setNationality(p.nationality);
    if (p.birth_city) setBirthCity(p.birth_city);
    if (p.birth_country) setBirthCountry(p.birth_country);
    if (p.place_of_origin) setPlaceOfOrigin(p.place_of_origin);
    if (p.father_name) setFatherName(p.father_name);
    if (p.father_birth_date) setFatherBirthDate(p.father_birth_date);
    if (p.mother_name) setMotherName(p.mother_name);
    if (p.mother_birth_date) setMotherBirthDate(p.mother_birth_date);
    if (p.wedding_date) setWeddingDate(p.wedding_date);
    if (p.parents_wedding_date) setParentsWeddingDate(p.parents_wedding_date);
    if (p.address?.street) {
      setAddress(
        `${p.address.street}${p.address.number ? `, ${p.address.number}` : ""}${p.address.city ? ` — ${p.address.city}/${p.address.state || ""}` : ""
          }`.trim(),
      );
      if (p.address.postal_code) setPostalCode(p.address.postal_code);
    }
    if (p.vehicle_model) setVehicleModel(p.vehicle_model);
    if (p.vehicle_color) setVehicleColor(p.vehicle_color);
    if (p.vehicle_plate) setVehiclePlate(p.vehicle_plate);
    if (p.vehicle_year) setVehicleYear(String(p.vehicle_year));
    if (p.health_plan) setHealthPlan(p.health_plan);
    if (p.blood_donor !== undefined) setBloodDonor(p.blood_donor);
    if (p.organ_donor !== undefined) setOrganDonor(p.organ_donor);
    if (p.team) setTeam(p.team);
    if (p.sector) setSector(p.sector);
    if (p.club_membership) setClubMembership(p.club_membership);
    if (p.membership_type) setMembershipType(p.membership_type);
    if (p.collections) setCollections(p.collections);
    if (p.pet) setPet(p.pet);
    if (p.supermarket_club) setSupermarketClub(p.supermarket_club);
    if (p.travel_countries) setTravelCountries(p.travel_countries);
    if (p.card_brand) setCardBrand(p.card_brand);
    if (p.card_bank) setCardBank(p.card_bank);
  };

  // Profile lookup clear handler
  const handleClearProfile = () => {
    setSelectedProfile(null);
    setHolderName("");
  };

  // Requisito Mínimo calculation for Person mode
  const matchingOfficialDocs = useMemo(() => {
    const list: string[] = [];
    if (selectedProfile?.document_identifiers) {
      Object.keys(selectedProfile.document_identifiers).forEach((key) => {
        if (isOfficialDoc(key)) list.push(key);
      });
    }
    documents.forEach((d) => {
      if (isOfficialDoc(d.typeName) && !list.includes(d.typeName)) {
        list.push(d.typeName);
      }
    });
    if (cpf.trim().length >= 11 && !list.includes("CPF")) {
      list.push("CPF");
    }
    return list;
  }, [cpf, documents, selectedProfile]);

  const hasMinimumRequirement = matchingOfficialDocs.length > 0;

  // Add inline document (Person mode)
  const handleConfirmAddInlineDoc = () => {
    const type = documentTypes.data?.types.find((t) => t.id === inlineDocTypeId);
    const typeName = type?.label || copy.docFallbackDefault;
    const newDoc: PendingDoc = {
      id: String(Date.now()),
      typeId: inlineDocTypeId || documentTypes.data?.types?.[0]?.id || "",
      typeName,
      number: inlineDocNumber.trim(),
      date: inlineDocDate,
      validUntil: inlineDocValidUntil,
      medium: inlineDocMedium,
      custody: inlineDocCustody,
      notes: inlineDocNotes.trim(),
      tag: "manual",
    };
    setDocuments((prev) => [...prev, newDoc]);
    setInlineDocNumber("");
    setInlineDocNotes("");
    setInlineDocDate(undefined);
    setInlineDocValidUntil(undefined);
    setAddingDoc(false);
  };

  const handleInlineDocFileDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fallbackType = documentTypes.data?.types?.[0];
    const newDoc: PendingDoc = {
      id: String(Date.now()),
      typeId: inlineDocTypeId || fallbackType?.id || "",
      typeName: fallbackType?.label || copy.docFallbackDefault,
      number: "01234567890",
      notes: copy.ocrExtractedNote.replace("{filename}", file.name),
      medium: "DIGITAL",
      tag: "ocr",
    };
    setDocuments((prev) => [...prev, newDoc]);
    setAddingDoc(false);
    e.target.value = "";
  };

  // Add inline bill (Person mode)
  const handleConfirmAddInlineBill = () => {
    const type = billTypes.data?.types.find((t) => t.id === inlineBillTypeId);
    const serviceName = type?.label || copy.billFallbackDefault;
    const newBill: PendingBill = {
      id: String(Date.now()),
      typeId: inlineBillTypeId || billTypes.data?.types?.[0]?.id || "",
      serviceName,
      provider: inlineBillProvider.trim(),
      installation: inlineBillInstallation.trim(),
      competence: inlineBillCompetence.trim(),
      amount: inlineBillAmount.trim(),
      medium: "DIGITAL",
      notes: "",
      tag: "manual",
    };
    setBills((prev) => [...prev, newBill]);
    setInlineBillProvider("");
    setInlineBillInstallation("");
    setInlineBillCompetence("");
    setInlineBillAmount("");
    setAddingBill(false);
  };

  const handleInlineBillFileDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fallbackType = billTypes.data?.types?.[0];
    const newBill: PendingBill = {
      id: String(Date.now()),
      typeId: inlineBillTypeId || fallbackType?.id || "",
      serviceName: fallbackType?.label || copy.billFallbackDefault,
      provider: "Concessionária",
      installation: "400123456",
      competence: "2026-09",
      amount: "150,00",
      medium: "DIGITAL",
      notes: copy.ocrExtractedNote.replace("{filename}", file.name),
      tag: "ocr",
    };
    setBills((prev) => [...prev, newBill]);
    setAddingBill(false);
    e.target.value = "";
  };

  // Standalone OCR file drop handlers
  const handleStandaloneDocOcrDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setDocNumber("12.345.678-9");
    setDocDate("2022-05-10");
    setDocValidUntil("2032-05-10");
    setDocNotes(copy.ocrExtractedNote.replace("{filename}", file.name));
    setDocMedium("DIGITAL");
    void message.success(copy.ocrSuccessDoc);
    e.target.value = "";
  };

  const handleStandaloneBillOcrDrop = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setBillProvider("Enel SP");
    setBillInstallation("400123456");
    setBillCompetence("2026-09");
    setBillAmount("142,50");
    setBillPrintedHolder(holderName || "Mariana Albuquerque");
    setBillPrintedAddress("Rua das Acácias, 120 — São Paulo/SP");
    setBillNotes(copy.ocrExtractedNote.replace("{filename}", file.name));
    setBillMedium("DIGITAL");
    if (!holderName) {
      setHolderName("Mariana Albuquerque");
    }
    void message.success(copy.ocrSuccessBill);
    e.target.value = "";
  };

  // Helper to ensure or create profile
  const resolveOrCreateProfile = async (fallbackName: string): Promise<string> => {
    if (selectedProfile) return selectedProfile.id;
    const finalName = holderName.trim() || fallbackName.trim() || copy.holderFallbackDefault;
    const profile = await createProfile({
      full_name: finalName,
      social_name: socialName.trim(),
      cpf: cpf.trim(),
      email: email.trim(),
      mobile_phone: phone.trim(),
      landline_phone: landline.trim(),
      ...(birthDate ? { birth_date: birthDate } : {}),
      ...(gender ? { gender } : {}),
      ...(maritalStatus ? { marital_status: maritalStatus } : {}),
      ...(bloodType ? { blood_type: bloodType } : {}),
      ...(nationality.trim() ? { nationality: nationality.trim() } : {}),
      ...(birthCity.trim() ? { birth_city: birthCity.trim() } : {}),
      ...(birthCountry.trim() ? { birth_country: birthCountry.trim() } : {}),
      ...(placeOfOrigin.trim() ? { place_of_origin: placeOfOrigin.trim() } : {}),
      ...(fatherName.trim() ? { father_name: fatherName.trim() } : {}),
      ...(fatherBirthDate ? { father_birth_date: fatherBirthDate } : {}),
      ...(motherName.trim() ? { mother_name: motherName.trim() } : {}),
      ...(motherBirthDate ? { mother_birth_date: motherBirthDate } : {}),
      ...(weddingDate ? { wedding_date: weddingDate } : {}),
      ...(parentsWeddingDate ? { parents_wedding_date: parentsWeddingDate } : {}),
      ...(vehicleModel.trim() ? { vehicle_model: vehicleModel.trim() } : {}),
      ...(vehicleColor.trim() ? { vehicle_color: vehicleColor.trim() } : {}),
      ...(vehiclePlate.trim() ? { vehicle_plate: vehiclePlate.trim() } : {}),
      ...(Number.isFinite(parseInt(vehicleYear.trim(), 10)) && parseInt(vehicleYear.trim(), 10) > 0
        ? { vehicle_year: parseInt(vehicleYear.trim(), 10) }
        : {}),
      ...(healthPlan.trim() ? { health_plan: healthPlan.trim() } : {}),
      ...(bloodDonor !== undefined ? { blood_donor: bloodDonor } : {}),
      ...(organDonor !== undefined ? { organ_donor: organDonor } : {}),
      ...(team.trim() ? { team: team.trim() } : {}),
      ...(sector.trim() ? { sector: sector.trim() } : {}),
      ...(clubMembership.trim() ? { club_membership: clubMembership.trim() } : {}),
      ...(membershipType.trim() ? { membership_type: membershipType.trim() } : {}),
      ...(collections.trim() ? { collections: collections.trim() } : {}),
      ...(pet.trim() ? { pet: pet.trim() } : {}),
      ...(supermarketClub.trim() ? { supermarket_club: supermarketClub.trim() } : {}),
      ...(travelCountries.trim() ? { travel_countries: travelCountries.trim() } : {}),
      ...(cardBrand.trim() ? { card_brand: cardBrand.trim() } : {}),
      ...(cardBank.trim() ? { card_bank: cardBank.trim() } : {}),
      address: {
        street: address.trim(),
        number: "",
        complement: "",
        neighborhood: "",
        city: "",
        state: "",
        postal_code: postalCode.trim(),
      },
      notes: "",
    });
    return profile.id;
  };

  // Save handler
  const handleSave = async () => {
    setSaving(true);
    try {
      if (mode === "documents") {
        const resolvedProfileId = await resolveOrCreateProfile(
          holderName || copy.holderFallbackDefault,
        );
        const typeId = docTypeId || documentTypes.data?.types?.[0]?.id || "";
        await createDocument({
          owner_profile_id: resolvedProfileId,
          document_type_id: typeId,
          identifier_value: docNumber.trim() || "—",
          document_date: docDate || (new Date().toISOString().split("T")[0] as string),
          ...(docValidUntil ? { valid_until: docValidUntil } : {}),
          medium: docMedium,
          ...(docMedium === "PHYSICAL" ? { idle_custody: docCustody } : {}),
          notes: docNotes.trim(),
        });

        void queryClient.invalidateQueries({ queryKey: ["documents"] });
        void queryClient.invalidateQueries({ queryKey: ["profiles"] });
        void queryClient.invalidateQueries({ queryKey: MINIMUM_REQUIREMENT_QUERY });

        const typeLabel =
          documentTypes.data?.types?.find((t) => t.id === typeId)?.label || copy.docFallbackDefault;
        const msg = copy.savedSuccessDoc
          .replace("{type}", typeLabel)
          .replace("{name}", holderName.trim() || copy.holderFallbackDefault);
        announceSaved(undefined, msg);
        if (onSuccess) onSuccess(holderName.trim());
        else onCancel();
        return;
      }

      if (mode === "bills") {
        const resolvedProfileId = await resolveOrCreateProfile(
          billPrintedHolder || holderName || copy.holderFallbackDefault,
        );
        const typeId = billTypeId || billTypes.data?.types?.[0]?.id || "";
        await createBill({
          owner_profile_id: resolvedProfileId,
          bill_type_id: typeId,
          printed_holder_name:
            billPrintedHolder.trim() || holderName.trim() || copy.holderFallbackDefault,
          printed_address: billPrintedAddress.trim() || address.trim() || copy.addressNotProvided,
          reference_value: billInstallation.trim() || "—",
          competence: billCompetence.trim() || "2026-09",
          amount: billAmount.trim() || "0",
          currency: "BRL",
          notes: billNotes.trim() || billProvider.trim(),
          medium: billMedium,
        });

        void queryClient.invalidateQueries({ queryKey: ["bills"] });
        void queryClient.invalidateQueries({ queryKey: ["profiles"] });

        const msg = copy.savedSuccessBill.replace(
          "{name}",
          holderName.trim() || billPrintedHolder.trim() || copy.holderFallbackDefault,
        );
        announceSaved(undefined, msg);
        if (onSuccess) onSuccess(holderName.trim());
        else onCancel();
        return;
      }

      // mode === "people"
      if (!holderName.trim()) {
        void message.error(copy.fieldHolderPlaceholder);
        setSaving(false);
        return;
      }

      const resolvedProfileId = await resolveOrCreateProfile(holderName.trim());

      for (const doc of documents) {
        if (doc.typeId) {
          await createDocument({
            owner_profile_id: resolvedProfileId,
            document_type_id: doc.typeId,
            identifier_value: doc.number || "—",
            document_date: doc.date || (new Date().toISOString().split("T")[0] as string),
            ...(doc.validUntil ? { valid_until: doc.validUntil } : {}),
            notes: doc.notes,
            medium: doc.medium,
            ...(doc.medium === "PHYSICAL" && doc.custody ? { idle_custody: doc.custody } : {}),
          });
        }
      }

      for (const b of bills) {
        if (b.typeId) {
          await createBill({
            owner_profile_id: resolvedProfileId,
            bill_type_id: b.typeId,
            printed_holder_name: holderName.trim(),
            printed_address: address.trim() || copy.addressNotProvided,
            reference_value: b.installation || "—",
            competence: b.competence || "2026-09",
            amount: b.amount || "0",
            currency: "BRL",
            notes: b.provider || "",
            medium: b.medium,
          });
        }
      }

      void queryClient.invalidateQueries({ queryKey: ["profiles"] });
      void queryClient.invalidateQueries({ queryKey: ["documents"] });
      void queryClient.invalidateQueries({ queryKey: ["bills"] });
      void queryClient.invalidateQueries({ queryKey: MINIMUM_REQUIREMENT_QUERY });

      const successMsg = copy.savedSuccessPerson.replace("{name}", holderName.trim());
      announceSaved(undefined, successMsg);

      if (onSuccess) onSuccess(holderName.trim());
      else onCancel();
    } catch {
      void message.error(copy.savingRecord);
    } finally {
      setSaving(false);
    }
  };

  const crumbCurrentTitle =
    mode === "documents"
      ? copy.crumbNewDocument
      : mode === "bills"
        ? copy.crumbNewBill
        : copy.crumbNewPerson;

  return (
    <div className="cadastro-single">
      {/* Breadcrumb navigation */}
      <nav aria-label={copy.crumbHome} className="cadastro-crumb">
        <button className="cadastro-crumb__btn" type="button" onClick={onCancel}>
          ← {copy.crumbHome}
        </button>
        <span className="cadastro-crumb__sep">/</span>
        <span className="cadastro-crumb__current">{crumbCurrentTitle}</span>
      </nav>

      {/* ============================================================ */}
      {/* MODE 1: DOCUMENT MODE (table === "documents")                */}
      {/* ============================================================ */}
      {mode === "documents" && (
        <>
          {/* Seção 1: Dados Completos do Documento */}
          <section className={`cadastro-group ${openPrimary ? "" : "cadastro-group--collapsed"}`}>
            <header className="cadastro-group__header" onClick={() => setOpenPrimary((p) => !p)}>
              <div className="cadastro-group__title-area">
                <span className="cadastro-group__icon">
                  <FileText size={18} strokeWidth={1.75} />
                </span>
                <h2 className="cadastro-group__title">{copy.sectionDocData}</h2>
              </div>
              <ChevronDown className="cadastro-group__chevron" size={18} />
            </header>

            {openPrimary && (
              <div className="cadastro-group__body">
                <p className="cadastro-group__hint">{copy.sectionDocDataHint}</p>
                <DocumentFormFields
                  copy={copy}
                  documentTypes={documentTypes.data?.types ?? []}
                  fileInputId={standaloneDocFileId}
                  state={{
                    docTypeId,
                    docIdentifier: docNumber,
                    docDate,
                    docValidUntil,
                    docMedium,
                    docCustody,
                    docNotes,
                  }}
                  onChange={(patch) => {
                    if (patch.docTypeId !== undefined) setDocTypeId(patch.docTypeId);
                    if (patch.docIdentifier !== undefined) setDocNumber(patch.docIdentifier);
                    if (patch.docDate !== undefined) setDocDate(patch.docDate);
                    if (patch.docValidUntil !== undefined) setDocValidUntil(patch.docValidUntil);
                    if (patch.docMedium !== undefined) setDocMedium(patch.docMedium);
                    if (patch.docCustody !== undefined) setDocCustody(patch.docCustody);
                    if (patch.docNotes !== undefined) setDocNotes(patch.docNotes);
                  }}
                  onFileDrop={handleStandaloneDocOcrDrop}
                />
              </div>
            )}
          </section>

          {/* Seção 2: Titular / Dono do Documento */}
          <section className={`cadastro-group ${openHolder ? "" : "cadastro-group--collapsed"}`}>
            <header className="cadastro-group__header" onClick={() => setOpenHolder((p) => !p)}>
              <div className="cadastro-group__title-area">
                <span className="cadastro-group__icon">
                  <User size={18} strokeWidth={1.75} />
                </span>
                <h2 className="cadastro-group__title">{copy.sectionHolderTitle}</h2>
              </div>
              <ChevronDown className="cadastro-group__chevron" size={18} />
            </header>

            {openHolder && (
              <div className="cadastro-group__body">
                <p className="cadastro-group__hint">{copy.sectionHolderHint}</p>

                <div className="cadastro-grid">
                  <div className="cadastro-col-8">
                    <div className="cadastro-field">
                      <label className="cadastro-field__label" htmlFor="cad-doc-holder">
                        {copy.fieldHolderName}
                      </label>
                      {selectedProfile ? (
                        <HolderSelectedCard
                          autoFilledNotice={copy.holderAutoFilledNotice}
                          changeText={copy.holderChangeButton}
                          onChange={() => setSelectedProfile(null)}
                          onUnlink={handleClearProfile}
                          profile={selectedProfile}
                          selectedTitle={copy.holderSelectedTitle}
                          unlinkText={copy.holderUnlinkButton}
                        />
                      ) : (
                        <HolderSearchSelect
                          ariaLabel={copy.fieldHolderName}
                          createNewOptionText={copy.holderCreateNewOption}
                          id="cad-doc-holder"
                          onClear={handleClearProfile}
                          onSelectNewName={(name) => {
                            setSelectedProfile(null);
                            setHolderName(name);
                          }}
                          onSelectProfile={(p) => applyProfileToState(p)}
                          placeholder={copy.holderSelectPlaceholder}
                          selectedProfile={selectedProfile}
                          value={holderName}
                        />
                      )}
                    </div>
                  </div>

                  <div className="cadastro-col-4">
                    <div className="cadastro-field">
                      <label className="cadastro-field__label" htmlFor="cad-doc-holder-cpf">
                        {copy.fieldCpf}
                      </label>
                      <Input
                        disabled={Boolean(selectedProfile)}
                        id="cad-doc-holder-cpf"
                        placeholder={copy.placeholderCpf}
                        value={cpf}
                        onChange={(e) => setCpf(e.target.value)}
                      />
                    </div>
                  </div>
                </div>
              </div>
            )}
          </section>
        </>
      )}

      {/* ============================================================ */}
      {/* MODE 2: BILL MODE (table === "bills")                         */}
      {/* ============================================================ */}
      {mode === "bills" && (
        <>
          {/* Seção 1: Dados Completos da Fatura */}
          <section className={`cadastro-group ${openPrimary ? "" : "cadastro-group--collapsed"}`}>
            <header className="cadastro-group__header" onClick={() => setOpenPrimary((p) => !p)}>
              <div className="cadastro-group__title-area">
                <span className="cadastro-group__icon">
                  <Zap size={18} strokeWidth={1.75} />
                </span>
                <h2 className="cadastro-group__title">{copy.sectionBillData}</h2>
              </div>
              <ChevronDown className="cadastro-group__chevron" size={18} />
            </header>

            {openPrimary && (
              <div className="cadastro-group__body">
                <p className="cadastro-group__hint">{copy.sectionBillDataHint}</p>
                <BillFormFields
                  billTypes={billTypes.data?.types ?? []}
                  copy={copy}
                  fileInputId={standaloneBillFileId}
                  state={{
                    billTypeId,
                    billProvider,
                    billInstallation,
                    billCompetence,
                    billDueDate,
                    billAmount,
                    billPrintedHolder,
                    billPrintedAddress,
                    billMedium,
                    billNotes,
                  }}
                  onChange={(patch) => {
                    if (patch.billTypeId !== undefined) setBillTypeId(patch.billTypeId);
                    if (patch.billProvider !== undefined) setBillProvider(patch.billProvider);
                    if (patch.billInstallation !== undefined) setBillInstallation(patch.billInstallation);
                    if (patch.billCompetence !== undefined) setBillCompetence(patch.billCompetence);
                    if (patch.billDueDate !== undefined) setBillDueDate(patch.billDueDate);
                    if (patch.billAmount !== undefined) setBillAmount(patch.billAmount);
                    if (patch.billPrintedHolder !== undefined) {
                      setBillPrintedHolder(patch.billPrintedHolder);
                      if (!holderName) setHolderName(patch.billPrintedHolder);
                    }
                    if (patch.billPrintedAddress !== undefined) setBillPrintedAddress(patch.billPrintedAddress);
                    if (patch.billMedium !== undefined) setBillMedium(patch.billMedium);
                    if (patch.billNotes !== undefined) setBillNotes(patch.billNotes);
                  }}
                  onFileDrop={handleStandaloneBillOcrDrop}
                />
              </div>
            )}
          </section>

          {/* Seção 2: Titular Vinculado */}
          <section className={`cadastro-group ${openHolder ? "" : "cadastro-group--collapsed"}`}>
            <header className="cadastro-group__header" onClick={() => setOpenHolder((p) => !p)}>
              <div className="cadastro-group__title-area">
                <span className="cadastro-group__icon">
                  <User size={18} strokeWidth={1.75} />
                </span>
                <h2 className="cadastro-group__title">{copy.sectionHolderTitle}</h2>
              </div>
              <ChevronDown className="cadastro-group__chevron" size={18} />
            </header>

            {openHolder && (
              <div className="cadastro-group__body">
                <p className="cadastro-group__hint">{copy.sectionHolderHint}</p>

                <div className="cadastro-grid">
                  <div className="cadastro-col-12">
                    <div className="cadastro-field">
                      <label className="cadastro-field__label" htmlFor="cad-bill-holder">
                        {copy.fieldHolderName}
                      </label>
                      {selectedProfile ? (
                        <HolderSelectedCard
                          autoFilledNotice={copy.holderAutoFilledNotice}
                          changeText={copy.holderChangeButton}
                          onChange={() => setSelectedProfile(null)}
                          onUnlink={handleClearProfile}
                          profile={selectedProfile}
                          selectedTitle={copy.holderSelectedTitle}
                          unlinkText={copy.holderUnlinkButton}
                        />
                      ) : (
                        <HolderSearchSelect
                          ariaLabel={copy.fieldHolderName}
                          createNewOptionText={copy.holderCreateNewOption}
                          id="cad-bill-holder"
                          onClear={handleClearProfile}
                          onSelectNewName={(name) => {
                            setSelectedProfile(null);
                            setHolderName(name);
                          }}
                          onSelectProfile={(p) => applyProfileToState(p)}
                          placeholder={copy.holderSelectPlaceholder}
                          selectedProfile={selectedProfile}
                          value={holderName}
                        />
                      )}
                    </div>
                  </div>
                </div>
              </div>
            )}
          </section>
        </>
      )}

      {/* ============================================================ */}
      {/* MODE 3: PERSON MODE (table === "people")                      */}
      {/* ============================================================ */}
      {mode === "people" && (
        <>
          {/* Seção 1: Identidade & Contato */}
          <section className={`cadastro-group ${openPrimary ? "" : "cadastro-group--collapsed"}`}>
            <header className="cadastro-group__header" onClick={() => setOpenPrimary((p) => !p)}>
              <div className="cadastro-group__title-area">
                <span className="cadastro-group__icon">
                  <User size={18} strokeWidth={1.75} />
                </span>
                <h2 className="cadastro-group__title">{copy.sectionIdentityTitle}</h2>

                {/* Requisito Mínimo: Status Pill Positivo */}
                <span
                  className={`minreq-pill ${hasMinimumRequirement ? "minreq-pill--satisfied" : "minreq-pill--pending"
                    }`}
                >
                  {hasMinimumRequirement ? (
                    <>
                      <Check size={14} strokeWidth={2.5} />
                      <span>
                        {copy.minReqSatisfied.replace(
                          "{types}",
                          matchingOfficialDocs.slice(0, 2).join(", "),
                        )}
                      </span>
                    </>
                  ) : (
                    <span>{copy.minReqPending}</span>
                  )}
                </span>
              </div>
              <ChevronDown className="cadastro-group__chevron" size={18} />
            </header>

            {openPrimary && (
              <div className="cadastro-group__body">
                <p className="cadastro-group__hint">{copy.sectionIdentityHint}</p>

                <div className="cadastro-grid">
                  {/* Busca e Vínculo de Titular */}
                  <div className="cadastro-col-12">
                    <div className="cadastro-field">
                      <label className="cadastro-field__label" htmlFor="cad-p-holder-select">
                        {copy.holderSelectPlaceholder}
                      </label>
                      {selectedProfile ? (
                        <HolderSelectedCard
                          autoFilledNotice={copy.holderAutoFilledNotice}
                          changeText={copy.holderChangeButton}
                          onChange={() => setSelectedProfile(null)}
                          onUnlink={handleClearProfile}
                          profile={selectedProfile}
                          selectedTitle={copy.holderSelectedTitle}
                          unlinkText={copy.holderUnlinkButton}
                        />
                      ) : (
                        <HolderSearchSelect
                          ariaLabel={copy.holderSelectPlaceholder}
                          createNewOptionText={copy.holderCreateNewOption}
                          id="cad-p-holder-select"
                          onClear={handleClearProfile}
                          onSelectNewName={(name) => {
                            setSelectedProfile(null);
                            setHolderName(name);
                          }}
                          onSelectProfile={(p) => applyProfileToState(p)}
                          placeholder={copy.holderSelectPlaceholder}
                          selectedProfile={selectedProfile}
                          value={holderName}
                        />
                      )}
                    </div>
                  </div>
                </div>

                <PersonDemographicsGroup
                  copy={copy}
                  disabled={Boolean(selectedProfile)}
                  onChange={(patch) => {
                    if (patch.fullName !== undefined) setHolderName(patch.fullName);
                    if (patch.socialName !== undefined) setSocialName(patch.socialName);
                    if (patch.cpf !== undefined) setCpf(patch.cpf);
                    if (patch.birthDate !== undefined) setBirthDate(patch.birthDate);
                    if (patch.email !== undefined) setEmail(patch.email);
                    if (patch.phone !== undefined) setPhone(patch.phone);
                    if (patch.landline !== undefined) setLandline(patch.landline);
                    if (patch.address !== undefined) setAddress(patch.address);
                    if (patch.postalCode !== undefined) setPostalCode(patch.postalCode);
                    if (patch.gender !== undefined) setGender(patch.gender);
                    if (patch.maritalStatus !== undefined) setMaritalStatus(patch.maritalStatus);
                    if (patch.bloodType !== undefined) setBloodType(patch.bloodType);
                    if (patch.nationality !== undefined) setNationality(patch.nationality);
                    if (patch.birthCity !== undefined) setBirthCity(patch.birthCity);
                    if (patch.birthCountry !== undefined) setBirthCountry(patch.birthCountry);
                    if (patch.placeOfOrigin !== undefined) setPlaceOfOrigin(patch.placeOfOrigin);
                  }}
                  state={{
                    fullName: holderName,
                    socialName,
                    cpf,
                    birthDate,
                    email,
                    phone,
                    landline,
                    address,
                    postalCode,
                    gender,
                    maritalStatus,
                    bloodType,
                    nationality,
                    birthCity,
                    birthCountry,
                    placeOfOrigin,
                  }}
                />
              </div>
            )}
          </section>

          {/* Seção Nova: Família e Filiação */}
          <section className={`cadastro-group ${openFamily ? "" : "cadastro-group--collapsed"}`}>
            <header className="cadastro-group__header" onClick={() => setOpenFamily((prev) => !prev)}>
              <div className="cadastro-group__title-area">
                <span className="cadastro-group__icon">
                  <Users size={18} strokeWidth={1.75} />
                </span>
                <h2 className="cadastro-group__title">{copy.sectionFamilyTitle}</h2>
              </div>
              <ChevronDown className="cadastro-group__chevron" size={18} />
            </header>

            {openFamily && (
              <div className="cadastro-group__body">
                <p className="cadastro-group__hint">{copy.sectionFamilyHint}</p>
                <PersonFamilyGroup
                  copy={copy}
                  disabled={Boolean(selectedProfile)}
                  onChange={(patch) => {
                    if (patch.fatherName !== undefined) setFatherName(patch.fatherName);
                    if (patch.fatherBirthDate !== undefined) setFatherBirthDate(patch.fatherBirthDate);
                    if (patch.motherName !== undefined) setMotherName(patch.motherName);
                    if (patch.motherBirthDate !== undefined) setMotherBirthDate(patch.motherBirthDate);
                    if (patch.weddingDate !== undefined) setWeddingDate(patch.weddingDate);
                    if (patch.parentsWeddingDate !== undefined) setParentsWeddingDate(patch.parentsWeddingDate);
                  }}
                  state={{
                    fatherName,
                    fatherBirthDate,
                    motherName,
                    motherBirthDate,
                    weddingDate,
                    parentsWeddingDate,
                  }}
                />
              </div>
            )}
          </section>

          {/* Seção Nova: Dados Complementares */}
          <section className={`cadastro-group ${openComplementary ? "" : "cadastro-group--collapsed"}`}>
            <header
              className="cadastro-group__header"
              onClick={() => setOpenComplementary((prev) => !prev)}
            >
              <div className="cadastro-group__title-area">
                <span className="cadastro-group__icon">
                  <Car size={18} strokeWidth={1.75} />
                </span>
                <h2 className="cadastro-group__title">{copy.sectionComplementaryTitle}</h2>
              </div>
              <ChevronDown className="cadastro-group__chevron" size={18} />
            </header>

            {openComplementary && (
              <div className="cadastro-group__body">
                <p className="cadastro-group__hint">{copy.sectionComplementaryHint}</p>
                <PersonComplementaryGroup
                  copy={copy}
                  disabled={Boolean(selectedProfile)}
                  onChange={(patch) => {
                    if (patch.vehicleModel !== undefined) setVehicleModel(patch.vehicleModel);
                    if (patch.vehicleColor !== undefined) setVehicleColor(patch.vehicleColor);
                    if (patch.vehiclePlate !== undefined) setVehiclePlate(patch.vehiclePlate);
                    if (patch.vehicleYear !== undefined) setVehicleYear(patch.vehicleYear);
                    if (patch.healthPlan !== undefined) setHealthPlan(patch.healthPlan);
                    if (patch.bloodDonor !== undefined) setBloodDonor(patch.bloodDonor ?? undefined);
                    if (patch.organDonor !== undefined) setOrganDonor(patch.organDonor ?? undefined);
                    if (patch.team !== undefined) setTeam(patch.team);
                    if (patch.sector !== undefined) setSector(patch.sector);
                    if (patch.clubMembership !== undefined) setClubMembership(patch.clubMembership);
                    if (patch.membershipType !== undefined) setMembershipType(patch.membershipType);
                    if (patch.collections !== undefined) setCollections(patch.collections);
                    if (patch.pet !== undefined) setPet(patch.pet);
                    if (patch.supermarketClub !== undefined) setSupermarketClub(patch.supermarketClub);
                    if (patch.travelCountries !== undefined) setTravelCountries(patch.travelCountries);
                    if (patch.cardBrand !== undefined) setCardBrand(patch.cardBrand);
                    if (patch.cardBank !== undefined) setCardBank(patch.cardBank);
                  }}
                  state={{
                    vehicleModel,
                    vehicleColor,
                    vehiclePlate,
                    vehicleYear,
                    healthPlan,
                    bloodDonor,
                    organDonor,
                    team,
                    sector,
                    clubMembership,
                    membershipType,
                    collections,
                    pet,
                    supermarketClub,
                    travelCountries,
                    cardBrand,
                    cardBank,
                  }}
                />
              </div>
            )}
          </section>

          {/* Seção 2: Anexar Documentos Iniciais */}
          <section className={`cadastro-group ${openDocs ? "" : "cadastro-group--collapsed"}`}>
            <header className="cadastro-group__header" onClick={() => setOpenDocs((prev) => !prev)}>
              <div className="cadastro-group__title-area">
                <span className="cadastro-group__icon">
                  <FileText size={18} strokeWidth={1.75} />
                </span>
                <h2 className="cadastro-group__title">{copy.sectionDocumentsTitle}</h2>
                <span className="cadastro-group__badge">{documents.length}</span>
              </div>
              <ChevronDown className="cadastro-group__chevron" size={18} />
            </header>

            {openDocs && (
              <div className="cadastro-group__body">
                <p className="cadastro-group__hint">{copy.sectionDocumentsHint}</p>

                <div className="strip">
                  {documents.length === 0 ? (
                    <div className="emptyrow">{copy.sectionDocumentsEmpty}</div>
                  ) : (
                    documents.map((doc) => (
                      <div key={doc.id} className="srow">
                        <span className="mk">
                          <FileText size={16} strokeWidth={1.75} />
                        </span>
                        <div className="tx">
                          <b>{doc.typeName}</b>
                          <span>
                            {doc.number ? `nº ${doc.number}` : copy.docNumberEmpty}
                            {doc.notes ? ` · ${doc.notes}` : ""}
                          </span>
                        </div>
                        <span className="grow" />
                        <span className={`tag ${doc.tag === "ocr" ? "gold" : ""}`}>
                          {doc.tag === "ocr" ? copy.tagOcr : copy.tagPhysical}
                        </span>
                        <div className="acts">
                          <button
                            type="button"
                            onClick={() =>
                              setDocuments((prev) => prev.filter((d) => d.id !== doc.id))
                            }
                          >
                            <Trash2 size={13} style={{ marginRight: 4 }} />
                            {copy.actionRemove}
                          </button>
                        </div>
                      </div>
                    ))
                  )}
                </div>

                {addingDoc ? (
                  <div className="inline-addform">
                    <div className="cadastro-grid">
                      <div className="cadastro-col-4">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">{copy.fieldDocType}</label>
                          <Select
                            options={
                              documentTypes.data?.types?.map((t: DocumentType) => ({
                                value: t.id,
                                label: t.label,
                              })) ?? []
                            }
                            value={inlineDocTypeId || documentTypes.data?.types?.[0]?.id}
                            onChange={(v) => {
                              if (v) setInlineDocTypeId(v);
                            }}
                          />
                        </div>
                      </div>
                      <div className="cadastro-col-4">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">{copy.fieldDocNumber}</label>
                          <Input
                            placeholder={copy.placeholderDocNumber}
                            value={inlineDocNumber}
                            onChange={(e) => setInlineDocNumber(e.target.value)}
                          />
                        </div>
                      </div>
                      <div className="cadastro-col-4">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">{copy.fieldDocDate}</label>
                          <DatePicker
                            format="DD/MM/YYYY"
                            style={{ width: "100%" }}
                            value={inlineDocDate ? dayjs(inlineDocDate) : null}
                            onChange={(d) =>
                              setInlineDocDate(d ? d.format("YYYY-MM-DD") : undefined)
                            }
                          />
                        </div>
                      </div>
                      <div className="cadastro-col-4">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">{copy.fieldDocValidUntil}</label>
                          <DatePicker
                            format="DD/MM/YYYY"
                            style={{ width: "100%" }}
                            value={inlineDocValidUntil ? dayjs(inlineDocValidUntil) : null}
                            onChange={(d) =>
                              setInlineDocValidUntil(d ? d.format("YYYY-MM-DD") : undefined)
                            }
                          />
                        </div>
                      </div>
                      <div className="cadastro-col-4">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">{copy.fieldDocMedium}</label>
                          <Segmented
                            block
                            options={[
                              { label: copy.tagPhysical, value: "PHYSICAL" },
                              { label: copy.tagDigital, value: "DIGITAL" },
                            ]}
                            value={inlineDocMedium}
                            onChange={(v) => setInlineDocMedium(v as "PHYSICAL" | "DIGITAL")}
                          />
                        </div>
                      </div>
                      {inlineDocMedium === "PHYSICAL" && (
                        <div className="cadastro-col-4">
                          <div className="cadastro-field">
                            <label className="cadastro-field__label">{copy.fieldDocCustody}</label>
                            <Select
                              options={[
                                { label: copy.fieldDocCustodyOrg, value: "ORGANIZATION" },
                                { label: copy.fieldDocCustodyOwner, value: "OWNER" },
                              ]}
                              value={inlineDocCustody}
                              onChange={(v) => setInlineDocCustody(v)}
                            />
                          </div>
                        </div>
                      )}
                      <div className="cadastro-col-4">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">{copy.fieldDocNotes}</label>
                          <Input
                            placeholder={copy.placeholderDocNotes}
                            value={inlineDocNotes}
                            onChange={(e) => setInlineDocNotes(e.target.value)}
                          />
                        </div>
                      </div>
                    </div>

                    <OcrDropzoneInline
                      inputId={inlineDocFileId}
                      label={copy.ocrInlineDropzoneDoc}
                      onFile={handleInlineDocFileDrop}
                    />

                    <div className="inline-actions">
                      <Button onClick={() => setAddingDoc(false)}>{copy.btnCancelAdd}</Button>
                      <Button type="primary" onClick={handleConfirmAddInlineDoc}>
                        {copy.btnConfirmAdd}
                      </Button>
                    </div>
                  </div>
                ) : (
                  <button
                    className="addrow"
                    type="button"
                    onClick={() => {
                      setInlineDocTypeId(documentTypes.data?.types?.[0]?.id || "");
                      setAddingDoc(true);
                    }}
                  >
                    <Plus size={15} strokeWidth={2} />
                    <span>{copy.btnAddDocument}</span>
                  </button>
                )}
              </div>
            )}
          </section>

          {/* Seção 3: Anexar Contas Iniciais */}
          <section className={`cadastro-group ${openBills ? "" : "cadastro-group--collapsed"}`}>
            <header
              className="cadastro-group__header"
              onClick={() => setOpenBills((prev) => !prev)}
            >
              <div className="cadastro-group__title-area">
                <span className="cadastro-group__icon">
                  <Zap size={18} strokeWidth={1.75} />
                </span>
                <h2 className="cadastro-group__title">{copy.sectionBillsTitle}</h2>
                <span className="cadastro-group__badge">{bills.length}</span>
              </div>
              <ChevronDown className="cadastro-group__chevron" size={18} />
            </header>

            {openBills && (
              <div className="cadastro-group__body">
                <p className="cadastro-group__hint">{copy.sectionBillsHint}</p>

                <div className="strip">
                  {bills.length === 0 ? (
                    <div className="emptyrow">{copy.sectionBillsEmpty}</div>
                  ) : (
                    bills.map((b) => (
                      <div key={b.id} className="srow">
                        <span className="mk">
                          <Zap size={16} strokeWidth={1.75} />
                        </span>
                        <div className="tx">
                          <b>
                            {b.serviceName}
                            {b.provider ? ` — ${b.provider}` : ""}
                          </b>
                          <span>
                            {b.installation
                              ? `${copy.fieldBillInstallation} ${b.installation}`
                              : copy.docNumberEmpty}
                            {b.amount ? ` · R$ ${b.amount}` : ""}
                          </span>
                        </div>
                        <span className="grow" />
                        <span className={`tag ${b.tag === "ocr" ? "gold" : "ok"}`}>
                          {b.tag === "ocr" ? copy.tagOcr : copy.tagActive}
                        </span>
                        <div className="acts">
                          <button
                            type="button"
                            onClick={() =>
                              setBills((prev) => prev.filter((item) => item.id !== b.id))
                            }
                          >
                            <Trash2 size={13} style={{ marginRight: 4 }} />
                            {copy.actionRemove}
                          </button>
                        </div>
                      </div>
                    ))
                  )}
                </div>

                {addingBill ? (
                  <div className="inline-addform">
                    <div className="cadastro-grid">
                      <div className="cadastro-col-4">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">{copy.fieldBillService}</label>
                          <Select
                            options={
                              billTypes.data?.types?.map((t: BillType) => ({
                                value: t.id,
                                label: t.label,
                              })) ?? []
                            }
                            value={inlineBillTypeId || billTypes.data?.types?.[0]?.id}
                            onChange={(v) => {
                              if (v) setInlineBillTypeId(v);
                            }}
                          />
                        </div>
                      </div>
                      <div className="cadastro-col-4">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">{copy.fieldBillProvider}</label>
                          <Input
                            placeholder={copy.placeholderBillProvider}
                            value={inlineBillProvider}
                            onChange={(e) => setInlineBillProvider(e.target.value)}
                          />
                        </div>
                      </div>
                      <div className="cadastro-col-4">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">
                            {copy.fieldBillInstallation}
                          </label>
                          <Input
                            placeholder={copy.placeholderBillInstallation}
                            value={inlineBillInstallation}
                            onChange={(e) => setInlineBillInstallation(e.target.value)}
                          />
                        </div>
                      </div>
                      <div className="cadastro-col-6">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">
                            {copy.fieldBillCompetence}
                          </label>
                          <Input
                            placeholder={copy.placeholderBillCompetence}
                            value={inlineBillCompetence}
                            onChange={(e) => setInlineBillCompetence(e.target.value)}
                          />
                        </div>
                      </div>
                      <div className="cadastro-col-6">
                        <div className="cadastro-field">
                          <label className="cadastro-field__label">{copy.fieldBillAmount}</label>
                          <Input
                            placeholder={copy.placeholderBillAmount}
                            value={inlineBillAmount}
                            onChange={(e) => setInlineBillAmount(e.target.value)}
                          />
                        </div>
                      </div>
                    </div>

                    <OcrDropzoneInline
                      inputId={inlineBillFileId}
                      label={copy.ocrInlineDropzoneBill}
                      onFile={handleInlineBillFileDrop}
                    />

                    <div className="inline-actions">
                      <Button onClick={() => setAddingBill(false)}>{copy.btnCancelAdd}</Button>
                      <Button type="primary" onClick={handleConfirmAddInlineBill}>
                        {copy.btnConfirmAdd}
                      </Button>
                    </div>
                  </div>
                ) : (
                  <button
                    className="addrow"
                    type="button"
                    onClick={() => {
                      setInlineBillTypeId(billTypes.data?.types?.[0]?.id || "");
                      setAddingBill(true);
                    }}
                  >
                    <Plus size={15} strokeWidth={2} />
                    <span>{copy.btnAddBill}</span>
                  </button>
                )}
              </div>
            )}
          </section>
        </>
      )}

      {/* Sticky Action Bar */}
      <CadastroStickyBar
        cancelButtonText={copy.actionCancel}
        onCancel={onCancel}
        onSave={handleSave}
        saveButtonText={
          saving
            ? copy.savingRecord
            : mode === "documents"
              ? copy.actionSaveDocument
              : mode === "bills"
                ? copy.actionSaveBill
                : copy.actionSavePerson
        }
        saving={saving}
        summaryText={
          mode === "people"
            ? copy.summaryCount
                .replace("{docs}", String(documents.length))
                .replace("{bills}", String(bills.length))
            : undefined
        }
      />
    </div>
  );
}
