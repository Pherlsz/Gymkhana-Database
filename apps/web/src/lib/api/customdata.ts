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
  listCustomFields,
  listCustomOptions,
  replaceCustomValues,
  updateCustomEntity as updateCustomEntityRequest,
  updateCustomEntityType as updateCustomEntityTypeRequest,
  updateCustomField as updateCustomFieldRequest,
  updateCustomOption as updateCustomOptionRequest,
  type CustomEntity,
  type CustomEntityType,
  type CustomField,
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
  listCustomFields,
  listCustomOptions,
  replaceCustomValues,
};
export type {
  CustomEntity,
  CustomEntityType,
  CustomField,
  CustomOption,
  CustomTargetKind,
  CustomValueInput,
  CustomValueSet,
  CustomValueTargetKind,
};

export type CustomFieldKind = components["schemas"]["CustomFieldKind"];
export type CustomProfileCardinality = components["schemas"]["CustomProfileCardinality"];
export type CustomStoredValue = components["schemas"]["CustomStoredValue"];
export type CustomEntityTypePage = components["schemas"]["CustomEntityTypePageResponse"];
export type CustomFieldPage = components["schemas"]["CustomFieldPageResponse"];
export type CustomOptionPage = components["schemas"]["CustomOptionListResponse"];
export type CustomEntityPage = components["schemas"]["CustomEntityPageResponse"];
export type CustomEntityTypeValues = components["schemas"]["CustomEntityTypeValuesRequest"];
export type CustomFieldValues = components["schemas"]["CustomFieldValuesRequest"];
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

export function createCustomField(values: CustomFieldValues): Promise<CustomField> {
  return createCustomFieldRequest(values);
}

export function updateCustomField(
  id: string,
  version: number,
  values: CustomFieldValues,
): Promise<CustomField> {
  return updateCustomFieldRequest(id, { ...values, version });
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
