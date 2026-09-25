import {
  createBill,
  createDocument,
  createProfile,
  updateProfile,
  type BillRecord,
  type DocumentRecord,
  type Profile,
  type ProfileValuesRequest,
} from "../api/client";
import { uploadAttachment, type AttachmentOwner } from "../api/attachments";
import { saveRecordCustomValues, type CustomDraftValue } from "../../RecordCustomFields";
import type { BillFormFieldsState } from "./components/BillFormFields";
import type { DocumentFormFieldsState } from "./components/DocumentFormFields";
import type { PersonDemographicsState } from "./components/PersonDemographicsGroup";
import { formatAddressLine } from "./cadastroPayloads";
import { canonicalBillAmount } from "./cadastroValidate";
import type { PendingBill, PendingDoc } from "./types";

export async function resolveOrCreateProfile(args: {
  selectedProfile: Profile | null;
  payload: ProfileValuesRequest;
}): Promise<Profile> {
  if (args.selectedProfile) {
    return updateProfile(args.selectedProfile.id, {
      ...args.payload,
      version: args.selectedProfile.version,
    });
  }
  return createProfile(args.payload);
}

async function persistCustomAndFile(args: {
  kind: "document" | "bill";
  definitionKind: "DOCUMENT_TYPE" | "BILL_TYPE";
  typeId: string;
  recordId: string;
  recordVersion: number;
  draft: Record<string, CustomDraftValue>;
  file?: File | undefined;
}): Promise<AttachmentOwner | null> {
  await saveRecordCustomValues({
    valueTargetKind: args.kind,
    definitionTargetKind: args.definitionKind,
    definitionTargetId: args.typeId,
    recordId: args.recordId,
    recordVersion: args.recordVersion,
    draft: args.draft,
    force: false,
  });
  if (!args.file) return null;
  const owner: AttachmentOwner = {
    owner_kind: args.kind === "document" ? "DOCUMENT" : "BILL",
    owner_id: args.recordId,
  };
  await uploadAttachment(owner, args.file, () => undefined);
  return owner;
}

export async function persistDocumentForm(args: {
  ownerProfileId: string;
  fields: DocumentFormFieldsState;
}): Promise<{ record: DocumentRecord; ocrOwner: AttachmentOwner | null }> {
  const record = await createDocument({
    owner_profile_id: args.ownerProfileId,
    document_type_id: args.fields.docTypeId,
    identifier_value: args.fields.docIdentifier.trim(),
    document_date: args.fields.docDate ?? "",
    notes: args.fields.docNotes.trim(),
    medium: args.fields.docMedium,
    ...(args.fields.docMedium === "PHYSICAL" ? { idle_custody: args.fields.docCustody } : {}),
    ...(args.fields.docValidUntil ? { valid_until: args.fields.docValidUntil } : {}),
  });
  const ocrOwner = await persistCustomAndFile({
    kind: "document",
    definitionKind: "DOCUMENT_TYPE",
    typeId: record.type.id,
    recordId: record.id,
    recordVersion: record.version,
    draft: args.fields.customDraft,
    file: args.fields.file,
  });
  return { record, ocrOwner };
}

export async function persistPendingDocument(args: {
  ownerProfileId: string;
  doc: PendingDoc;
}): Promise<{ record: DocumentRecord; ocrOwner: AttachmentOwner | null }> {
  return persistDocumentForm({
    ownerProfileId: args.ownerProfileId,
    fields: {
      docTypeId: args.doc.typeId,
      docIdentifier: args.doc.number,
      docDate: args.doc.date,
      docValidUntil: args.doc.validUntil,
      docMedium: args.doc.medium,
      docCustody: args.doc.custody ?? "ORGANIZATION",
      docNotes: args.doc.notes,
      customDraft: args.doc.customDraft,
      file: args.doc.file,
    },
  });
}

export async function persistBillForm(args: {
  ownerProfileId: string;
  fields: BillFormFieldsState;
  demographics: PersonDemographicsState;
  fallbackHolder: string;
}): Promise<{ record: BillRecord; ocrOwner: AttachmentOwner | null }> {
  const printedHolder =
    args.fields.billPrintedHolder.trim() ||
    args.demographics.fullName.trim() ||
    args.fallbackHolder;
  const printedAddress =
    args.fields.billPrintedAddress.trim() || formatAddressLine(args.demographics);
  const record = await createBill({
    owner_profile_id: args.ownerProfileId,
    bill_type_id: args.fields.billTypeId,
    printed_holder_name: printedHolder,
    printed_address: printedAddress,
    reference_value: args.fields.billInstallation.trim(),
    competence: args.fields.billCompetence.trim(),
    amount: canonicalBillAmount(args.fields.billAmount),
    currency: "BRL",
    notes: args.fields.billNotes.trim(),
    medium: args.fields.billMedium,
  });
  const ocrOwner = await persistCustomAndFile({
    kind: "bill",
    definitionKind: "BILL_TYPE",
    typeId: record.type.id,
    recordId: record.id,
    recordVersion: record.version,
    draft: args.fields.customDraft,
    file: args.fields.file,
  });
  return { record, ocrOwner };
}

export async function persistPendingBill(args: {
  ownerProfileId: string;
  bill: PendingBill;
  demographics: PersonDemographicsState;
  fallbackHolder: string;
}): Promise<{ record: BillRecord; ocrOwner: AttachmentOwner | null }> {
  return persistBillForm({
    ownerProfileId: args.ownerProfileId,
    demographics: args.demographics,
    fallbackHolder: args.fallbackHolder,
    fields: {
      billTypeId: args.bill.typeId,
      billInstallation: args.bill.installation,
      billCompetence: args.bill.competence,
      billAmount: args.bill.amount,
      billPrintedHolder: args.bill.printedHolder ?? "",
      billPrintedAddress: args.bill.printedAddress ?? "",
      billMedium: args.bill.medium,
      billNotes: args.bill.notes,
      customDraft: args.bill.customDraft,
      file: args.bill.file,
    },
  });
}
