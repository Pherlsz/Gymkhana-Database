import {
  createBill,
  createDocument,
  createProfile,
  updateBill,
  updateDocument,
  updateProfile,
  type BillRecord,
  type BillValuesRequest,
  type DocumentRecord,
  type DocumentValuesRequest,
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
}): Promise<{ ocrOwner: AttachmentOwner | null; version: number }> {
  let version = args.recordVersion;
  try {
    const saved = await saveRecordCustomValues({
      valueTargetKind: args.kind,
      definitionTargetKind: args.definitionKind,
      definitionTargetId: args.typeId,
      recordId: args.recordId,
      recordVersion: args.recordVersion,
      draft: args.draft,
      force: false,
    });
    if (saved) version = saved.version;
  } catch (caught) {
    throw withStoredRecord({ id: args.recordId, version: args.recordVersion }, caught);
  }
  if (!args.file) return { ocrOwner: null, version };
  const owner: AttachmentOwner = {
    owner_kind: args.kind === "document" ? "DOCUMENT" : "BILL",
    owner_id: args.recordId,
  };
  try {
    await uploadAttachment(owner, args.file, () => undefined);
  } catch (caught) {
    throw withStoredRecord({ id: args.recordId, version }, caught);
  }
  return { ocrOwner: owner, version };
}

type StoredRecord = { id: string; version: number };

function withStoredRecord(record: StoredRecord, cause: unknown): unknown {
  if (cause && typeof cause === "object") {
    Object.assign(cause, { storedRecord: record });
    return cause;
  }
  const error = new Error("persist failed");
  Object.assign(error, { storedRecord: record });
  return error;
}

export function storedRecordOf(error: unknown): StoredRecord | undefined {
  if (!error || typeof error !== "object" || !("storedRecord" in error)) return undefined;
  const stored = (error as { storedRecord?: StoredRecord }).storedRecord;
  if (!stored || typeof stored.id !== "string" || typeof stored.version !== "number") return undefined;
  return stored;
}

function documentWriteRequest(
  ownerProfileId: string,
  fields: DocumentFormFieldsState,
): DocumentValuesRequest {
  return {
    owner_profile_id: ownerProfileId,
    document_type_id: fields.docTypeId,
    identifier_value: fields.docIdentifier.trim(),
    document_date: fields.docDate ?? "",
    notes: fields.docNotes.trim(),
    medium: fields.docMedium,
    ...(fields.docMedium === "PHYSICAL" ? { idle_custody: fields.docCustody } : {}),
    ...(fields.docValidUntil ? { valid_until: fields.docValidUntil } : {}),
  };
}

export async function persistDocumentForm(args: {
  ownerProfileId: string;
  fields: DocumentFormFieldsState;
  existing?: StoredRecord | undefined;
}): Promise<{ record: DocumentRecord; ocrOwner: AttachmentOwner | null }> {
  const request = documentWriteRequest(args.ownerProfileId, args.fields);
  const record = args.existing
    ? await updateDocument(args.existing.id, { ...request, version: args.existing.version })
    : await createDocument(request);
  try {
    const followed = await persistCustomAndFile({
      kind: "document",
      definitionKind: "DOCUMENT_TYPE",
      typeId: record.type.id,
      recordId: record.id,
      recordVersion: record.version,
      draft: args.fields.customDraft,
      file: args.fields.file,
    });
    return {
      record: followed.version === record.version ? record : { ...record, version: followed.version },
      ocrOwner: followed.ocrOwner,
    };
  } catch (caught) {
    if (storedRecordOf(caught)) throw caught;
    throw withStoredRecord({ id: record.id, version: record.version }, caught);
  }
}

export async function persistPendingDocument(args: {
  ownerProfileId: string;
  doc: PendingDoc;
}): Promise<{ record: DocumentRecord; ocrOwner: AttachmentOwner | null }> {
  return persistDocumentForm({
    ownerProfileId: args.ownerProfileId,
    ...(args.doc.savedRecordId
      ? {
          existing: {
            id: args.doc.savedRecordId,
            version: args.doc.savedRecordVersion ?? 1,
          },
        }
      : {}),
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

function billWriteRequest(args: {
  ownerProfileId: string;
  fields: BillFormFieldsState;
  demographics: PersonDemographicsState;
  fallbackHolder: string;
}): BillValuesRequest {
  const printedHolder =
    args.fields.billPrintedHolder.trim() ||
    args.demographics.fullName.trim() ||
    args.fallbackHolder;
  const printedAddress =
    args.fields.billPrintedAddress.trim() || formatAddressLine(args.demographics);
  return {
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
  };
}

export async function persistBillForm(args: {
  ownerProfileId: string;
  fields: BillFormFieldsState;
  demographics: PersonDemographicsState;
  fallbackHolder: string;
  existing?: StoredRecord | undefined;
}): Promise<{ record: BillRecord; ocrOwner: AttachmentOwner | null }> {
  const request = billWriteRequest(args);
  const record = args.existing
    ? await updateBill(args.existing.id, {
        owner_profile_id: args.ownerProfileId,
        bill_type_id: request.bill_type_id,
        printed_holder_name: request.printed_holder_name,
        printed_address: request.printed_address,
        reference_value: request.reference_value,
        competence: request.competence,
        amount: request.amount,
        currency: request.currency,
        notes: request.notes,
        medium: request.medium,
        version: args.existing.version,
      })
    : await createBill(request);
  try {
    const followed = await persistCustomAndFile({
      kind: "bill",
      definitionKind: "BILL_TYPE",
      typeId: record.type.id,
      recordId: record.id,
      recordVersion: record.version,
      draft: args.fields.customDraft,
      file: args.fields.file,
    });
    return {
      record: followed.version === record.version ? record : { ...record, version: followed.version },
      ocrOwner: followed.ocrOwner,
    };
  } catch (caught) {
    if (storedRecordOf(caught)) throw caught;
    throw withStoredRecord({ id: record.id, version: record.version }, caught);
  }
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
    ...(args.bill.savedRecordId
      ? {
          existing: {
            id: args.bill.savedRecordId,
            version: args.bill.savedRecordVersion ?? 1,
          },
        }
      : {}),
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
