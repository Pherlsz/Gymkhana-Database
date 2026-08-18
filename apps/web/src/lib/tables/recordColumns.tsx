import type { CustomField } from "../api/client";
import { dataColumn, extraColumn, identityColumn, numericColumn } from "./peopleColumns";
import type { SpreadsheetColumn } from "./SpreadsheetTable";
import type { TableRow } from "./tableRows";

export type RecordSheetCopy = {
  columns: {
    identifier: string;
    reference: string;
    type: string;
    owner: string;
    status: string;
    medium: string;
    idleCustody: string;
    validUntil: string;
    date: string;
    currentHolder: string;
    notes: string;
    competence: string;
    amount: string;
    printedHolder: string;
    printedAddress: string;
    currency: string;
  };
  status: { AVAILABLE: string; IN_USE: string };
  medium: { PHYSICAL: string; DIGITAL: string };
  idleCustody: { ORGANIZATION: string; OWNER: string };
  boolean: { yes: string; no: string };
};

export function buildDocumentColumns(
  copy: RecordSheetCopy,
  extraFields: CustomField[],
): SpreadsheetColumn<TableRow>[] {
  return [
    identityColumn(
      "identifier",
      copy.columns.identifier,
      "identifier_value",
      "spreadsheet-table__numeric",
    ),
    dataColumn("type", copy.columns.type, 160, "type_label"),
    dataColumn("owner", copy.columns.owner, 200),
    dataColumn("status", copy.columns.status, 120),
    dataColumn("medium", copy.columns.medium, 120),
    dataColumn("idle_custody", copy.columns.idleCustody, 140),
    dataColumn("valid_until", copy.columns.validUntil, 120),
    dataColumn("date", copy.columns.date, 120, "document_date"),
    dataColumn("current_holder", copy.columns.currentHolder, 180),
    dataColumn("notes", copy.columns.notes, 200),
    ...extraFields.map((field) => extraColumn(field)),
  ];
}

export function buildBillColumns(
  copy: RecordSheetCopy,
  extraFields: CustomField[],
): SpreadsheetColumn<TableRow>[] {
  return [
    identityColumn(
      "reference",
      copy.columns.reference,
      "reference_value",
      "spreadsheet-table__numeric",
    ),
    dataColumn("type", copy.columns.type, 160, "type_label"),
    dataColumn("owner", copy.columns.owner, 200),
    dataColumn("competence", copy.columns.competence, 130, "competence"),
    numericColumn("amount", copy.columns.amount, 120, "amount"),
    dataColumn("status", copy.columns.status, 120),
    dataColumn("medium", copy.columns.medium, 120),
    dataColumn("idle_custody", copy.columns.idleCustody, 140),
    dataColumn("printed_holder_name", copy.columns.printedHolder, 180),
    dataColumn("printed_address", copy.columns.printedAddress, 220),
    dataColumn("currency", copy.columns.currency, 80),
    dataColumn("notes", copy.columns.notes, 200),
    ...extraFields.map((field) => extraColumn(field)),
  ];
}
