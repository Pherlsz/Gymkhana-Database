import type { BillValuesRequest, DocumentValuesRequest } from "../api/client";
import { physicalCustody } from "../records/RecordEditorCommon";
import type { BillFormFieldsState } from "./components/BillFormFields";
import type { DocumentFormFieldsState } from "./components/DocumentFormFields";

export function documentFormFromValues(values: DocumentValuesRequest): DocumentFormFieldsState {
  return {
    docTypeId: values.document_type_id,
    docIdentifier: values.identifier_value,
    docDate: values.document_date || undefined,
    docValidUntil: values.valid_until || undefined,
    docMedium: values.medium,
    docCustody: values.idle_custody ?? "ORGANIZATION",
    docNotes: values.notes ?? "",
    customDraft: {},
  };
}

export function documentValuesFromForm(
  ownerProfileId: string,
  state: DocumentFormFieldsState,
): DocumentValuesRequest {
  return {
    owner_profile_id: ownerProfileId,
    document_type_id: state.docTypeId,
    identifier_value: state.docIdentifier,
    document_date: state.docDate ?? "",
    notes: state.docNotes,
    medium: state.docMedium,
    ...physicalCustody(state.docMedium, state.docCustody),
    ...(state.docValidUntil ? { valid_until: state.docValidUntil } : {}),
  };
}

export function billFormFromValues(values: BillValuesRequest): BillFormFieldsState {
  return {
    billTypeId: values.bill_type_id,
    billInstallation: values.reference_value,
    billCompetence: values.competence,
    billAmount: values.amount,
    billPrintedHolder: values.printed_holder_name,
    billPrintedAddress: values.printed_address,
    billMedium: values.medium,
    billNotes: values.notes ?? "",
    customDraft: {},
  };
}

export function billValuesFromForm(
  ownerProfileId: string,
  state: BillFormFieldsState,
  extras: { currency: string; idleCustody?: "ORGANIZATION" | "OWNER" | undefined },
): BillValuesRequest {
  return {
    owner_profile_id: ownerProfileId,
    bill_type_id: state.billTypeId,
    printed_holder_name: state.billPrintedHolder,
    printed_address: state.billPrintedAddress,
    reference_value: state.billInstallation,
    competence: state.billCompetence,
    amount: state.billAmount,
    currency: extras.currency,
    notes: state.billNotes,
    medium: state.billMedium,
    ...physicalCustody(state.billMedium, extras.idleCustody),
  };
}
