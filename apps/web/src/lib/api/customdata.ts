import type { components } from "../../generated/api";
import {
  APIRequestError,
  createCustomEntity as createCustomEntityRequest,
  createCustomEntityType as createCustomEntityTypeRequest,
  createCustomField as createCustomFieldRequest,
  createCustomOption as createCustomOptionRequest,
  deleteCustomEntity,
  deleteCustomEntityType,
  deleteCustomField,
  deleteCustomOption,
  getCustomValues,
  listCustomEntities,
  listCustomEntityTypes,
  listCustomFields as listCustomFieldsRequest,
  listCustomOptions,
  replaceCustomValues,
  updateCustomEntity as updateCustomEntityRequest,
  updateCustomEntityType as updateCustomEntityTypeRequest,
  updateCustomField as updateCustomFieldRequest,
  updateCustomOption as updateCustomOptionRequest,
  type CustomEntity,
  type CustomEntityType,
  type CustomField as GeneratedCustomField,
  type CustomOption,
  type CustomTargetKind,
  type CustomValueInput,
  type CustomValueSet,
  type CustomValueTargetKind,
} from "./client";

export {
  APIRequestError,
  deleteCustomEntity,
  deleteCustomEntityType,
  deleteCustomField,
  deleteCustomOption,
  getCustomValues,
  listCustomEntities,
  listCustomEntityTypes,
  listCustomOptions,
  replaceCustomValues,
};
export type {
  CustomEntity,
  CustomEntityType,
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
export type CustomProfileCardinality = components["schemas"]["CustomProfileCardinality"];
export type CustomStoredValue = components["schemas"]["CustomStoredValue"];
export type CustomEntityTypePage = components["schemas"]["CustomEntityTypePageResponse"];
export type CustomFieldPage = Omit<components["schemas"]["CustomFieldPageResponse"], "fields"> & {
  fields: CustomField[];
};
export type CustomOptionPage = components["schemas"]["CustomOptionListResponse"];
export type CustomEntityPage = components["schemas"]["CustomEntityPageResponse"];
export type CustomEntityTypeValues = components["schemas"]["CustomEntityTypeValuesRequest"];
export type CustomFieldValues = Omit<
  components["schemas"]["CustomFieldValuesRequest"],
  "field_kind"
> & {
  field_kind: CustomFieldKind;
};
export type CustomOptionValues = components["schemas"]["CustomOptionValuesRequest"];

export function createCustomEntityType(values: CustomEntityTypeValues): Promise<CustomEntityType> {
  return createCustomEntityTypeRequest(values);
}

export function updateCustomEntityType(
  id: string,
  version: number,
  values: CustomEntityTypeValues,
): Promise<CustomEntityType> {
  return updateCustomEntityTypeRequest(id, { ...values, version });
}

export async function listCustomFields(
  targetKind: CustomTargetKind,
  targetId?: string,
  signal?: AbortSignal,
): Promise<CustomFieldPage> {
  return (await listCustomFieldsRequest(targetKind, targetId, signal)) as CustomFieldPage;
}

export async function createCustomField(values: CustomFieldValues): Promise<CustomField> {
  return (await createCustomFieldRequest(
    values as components["schemas"]["CustomFieldValuesRequest"],
  )) as CustomField;
}

export async function updateCustomField(
  id: string,
  version: number,
  values: CustomFieldValues,
): Promise<CustomField> {
  return (await updateCustomFieldRequest(id, {
    ...(values as components["schemas"]["CustomFieldValuesRequest"]),
    version,
  })) as CustomField;
}

export function createCustomOption(
  fieldId: string,
  values: CustomOptionValues,
): Promise<CustomOption> {
  return createCustomOptionRequest(fieldId, values);
}

export function updateCustomOption(
  fieldId: string,
  optionId: string,
  version: number,
  values: CustomOptionValues,
): Promise<CustomOption> {
  return updateCustomOptionRequest(fieldId, optionId, { ...values, version });
}

export function createCustomEntity(request: {
  entity_type_id: string;
  owner_profile_id?: string;
  values: CustomValueInput[];
}): Promise<CustomEntity> {
  return createCustomEntityRequest(request);
}

export function updateCustomEntity(
  id: string,
  version: number,
  values: CustomValueInput[],
): Promise<CustomEntity> {
  return updateCustomEntityRequest(id, { version, values });
}
