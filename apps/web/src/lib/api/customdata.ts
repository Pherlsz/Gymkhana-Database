import type { components } from "../../generated/api";
import {
  getCustomValues,
  listCustomFields as listCustomFieldsRequest,
  listCustomOptions,
  replaceCustomValues,
  type CustomField as GeneratedCustomField,
  type CustomOption,
  type CustomTargetKind,
  type CustomValueInput,
  type CustomValueSet,
  type CustomValueTargetKind,
} from "./client";

export { getCustomValues, listCustomOptions, replaceCustomValues };
export type {
  CustomOption,
  CustomTargetKind,
  CustomValueInput,
  CustomValueSet,
  CustomValueTargetKind,
};

export type CustomFieldKind = components["schemas"]["CustomFieldKind"] | "ATTACHMENT";
export type CustomField = Omit<GeneratedCustomField, "field_kind"> & {
  field_kind: CustomFieldKind;
};
export type CustomFieldPage = Omit<components["schemas"]["CustomFieldPageResponse"], "fields"> & {
  fields: CustomField[];
};

export async function listCustomFields(
  targetKind: CustomTargetKind,
  targetId?: string,
  signal?: AbortSignal,
): Promise<CustomFieldPage> {
  return (await listCustomFieldsRequest(targetKind, targetId, signal)) as CustomFieldPage;
}
