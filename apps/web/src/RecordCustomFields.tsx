// Inline custom-field support for record editors (DocumentEditor, BillEditor).
// Reuses the dynamic form already rendered by CustomValuesPanel: field
// definitions come from listCustomFields(targetKind, typeId) — i.e. the rows
// of custom_field_definitions seeded per document/bill type — and values are
// persisted through PUT /api/v1/custom-values/{target_kind}/{target_id}.
import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
// .custom-values__grid / __wide / __choices live here; importing keeps the
// styles attached to this component (Vite dedupes the stylesheet).
import "./customdata.css";
import {
  CustomFieldInputGrid,
  customDataError,
  customInputsFromDraft,
  draftFromValueSet,
  type CustomDraftValue,
} from "./CustomValuesPanel";

export type { CustomDraftValue };
import {
  getCustomValues,
  listCustomFields,
  replaceCustomValues,
  type CustomField,
  type CustomTargetKind,
  type CustomValueTargetKind,
} from "./lib/api/customdata";

// Fields are editable scalars only: attachments have their own panel.
export function scalarFieldsOf(fields: CustomField[]): CustomField[] {
  return fields.filter((field) => field.active && field.field_kind !== "ATTACHMENT");
}

export function useTypeCustomFields(
  definitionTargetKind: CustomTargetKind,
  definitionTargetId: string | undefined,
) {
  const query = useQuery({
    queryKey: ["custom-fields", definitionTargetKind, definitionTargetId ?? "global"],
    queryFn: ({ signal }) => listCustomFields(definitionTargetKind, definitionTargetId, signal),
  });
  return { scalarFields: scalarFieldsOf(query.data?.fields ?? []), query };
}

// Seeds the draft from stored values once they load (view/edit of an existing
// record). For new records the query stays disabled and the draft stays {}.
export function useSeedRecordDraft(
  valueTargetKind: CustomValueTargetKind,
  recordId: string | undefined,
  setDraft: (value: Record<string, CustomDraftValue>) => void,
) {
  const query = useQuery({
    queryKey: ["custom-values", valueTargetKind, recordId ?? "new"],
    queryFn: ({ signal }) => getCustomValues(valueTargetKind, recordId!, signal),
    enabled: Boolean(recordId),
  });
  useEffect(() => {
    if (query.data) setDraft(draftFromValueSet(query.data));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data]);
  return query;
}

export function RecordCustomFieldsSection(props: {
  definitionTargetKind: CustomTargetKind;
  definitionTargetId: string | undefined;
  disabled: boolean;
  draft: Record<string, CustomDraftValue>;
  onDraftChange: (value: Record<string, CustomDraftValue>) => void;
}) {
  const { scalarFields, query } = useTypeCustomFields(
    props.definitionTargetKind,
    props.definitionTargetId,
  );
  if (query.isLoading || scalarFields.length === 0) return null;
  return (
    <div className="record-form__wide">
      <CustomFieldInputGrid
        fields={scalarFields}
        draft={props.draft}
        disabled={props.disabled}
        onChange={props.onDraftChange}
      />
    </div>
  );
}

// Persists the draft against a saved record. The PUT endpoint deletes stored
// values that are absent from the payload, so on edits always send the full
// draft (force=true) — including emptied fields; on creates skip when empty.
// Returns the updated value set, or null when nothing was written.
export async function saveRecordCustomValues(args: {
  valueTargetKind: CustomValueTargetKind;
  definitionTargetKind: CustomTargetKind;
  definitionTargetId: string | undefined;
  recordId: string;
  recordVersion: number;
  draft: Record<string, CustomDraftValue>;
  force: boolean;
}): Promise<import("./lib/api/customdata").CustomValueSet | null> {
  const response = await listCustomFields(args.definitionTargetKind, args.definitionTargetId);
  const inputs = customInputsFromDraft(scalarFieldsOf(response.fields ?? []), args.draft);
  if (!args.force && inputs.length === 0) return null;
  return replaceCustomValues(args.valueTargetKind, args.recordId, args.recordVersion, inputs);
}

export function recordCustomFieldError(error: unknown): string {
  return customDataError(error);
}
