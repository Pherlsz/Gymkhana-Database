import { Alert, Button, Card, Flex, Select } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useI18n } from "./i18n";
import { SearchField } from "./components/SearchField";
import {
  CustomFieldInputGrid,
  customDataError,
  customInputsFromDraft,
  draftFromStoredValues,
  type CustomDraftValue,
} from "./CustomValuesPanel";
import {
  createCustomEntity,
  deleteCustomEntity,
  listCustomEntities,
  listCustomEntityTypes,
  listCustomFields,
  updateCustomEntity,
  type CustomEntity,
} from "./lib/api/customdata";
import { SectionTitle } from "./CustomDataShared";

export function CustomEntitiesAdmin() {
  const queryClient = useQueryClient();
  const { messages } = useI18n();
  const copy = messages.admin.customEntities;
  const types = useQuery({
    queryKey: ["custom-entity-types"],
    queryFn: ({ signal }) => listCustomEntityTypes(signal),
  });
  const [typeId, setTypeId] = useState("");
  const [ownerId, setOwnerId] = useState("");
  const [ownerQuery, setOwnerQuery] = useState("");
  const [editing, setEditing] = useState<CustomEntity | null>(null);
  const [draft, setDraft] = useState<Record<string, CustomDraftValue>>({});
  const selectedType = types.data?.types.find((value) => value.id === typeId);
  const fields = useQuery({
    queryKey: ["custom-fields", "CUSTOM_ENTITY_TYPE", typeId],
    queryFn: ({ signal }) => listCustomFields("CUSTOM_ENTITY_TYPE", typeId, signal),
    enabled: Boolean(typeId),
  });
  const entities = useQuery({
    queryKey: ["custom-entities", typeId, ownerId || "all"],
    queryFn: ({ signal }) => listCustomEntities(typeId, ownerId || undefined, signal),
    enabled: Boolean(typeId),
  });
  const save = useMutation({
    mutationFn: () => {
      const values = customInputsFromDraft(
        (fields.data?.fields ?? []).filter((value) => value.active),
        draft,
      );
      return editing
        ? updateCustomEntity(editing.id, editing.version, values)
        : createCustomEntity({
            entity_type_id: typeId,
            ...(ownerId ? { owner_profile_id: ownerId } : {}),
            values,
          });
    },
    onSuccess: async () => {
      setEditing(null);
      setDraft({});
      await queryClient.invalidateQueries({ queryKey: ["custom-entities", typeId] });
    },
  });
  const remove = useMutation({
    mutationFn: (value: CustomEntity) =>
      deleteCustomEntity(value.id, value.version, messages.common.actions.confirm),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: ["custom-entities", typeId] }),
  });
  useEffect(() => {
    setEditing(null);
    setDraft({});
  }, [typeId, ownerId]);
  return (
    <Flex vertical gap="1rem">
      <SectionTitle
        title={copy.title}
        description={copy.description}
      />
      {save.isError || remove.isError || entities.isError || fields.isError ? (
        <Alert
          message={copy.errorTitle}
          type="error"
          description={
            <>{customDataError(save.error ?? remove.error ?? entities.error ?? fields.error)}</>
          }
        />
      ) : null}
      <Card className="custom-admin-form">
        <div className="custom-admin-grid">
          <label>
            {copy.typeLabel}
            <Select
              onChange={(value) => setTypeId(value)}
              options={[
                { value: "", label: copy.typeSelectPlaceholder },
                ...(types.data?.types
                  .filter((value) => value.active)
                  .map((value) => ({
                    value: value.id,
                    label: value.label,
                  })) ?? []),
              ]}
              value={typeId}
            />
          </label>
          <label>
            {copy.ownerProfileLabel}
            <SearchField
              label={copy.ownerProfileLabel}
              lookup
              mode="suggest"
              placeholder={copy.ownerProfilePlaceholder}
              value={ownerQuery}
              onChange={setOwnerQuery}
              onPick={(id, name) => {
                setOwnerId(id);
                setOwnerQuery(name);
              }}
            />
          </label>
        </div>
        {selectedType ? (
          <p>
            {selectedType.profile_cardinality === "ONE_PER_PROFILE"
              ? copy.onePerProfileNotice
              : copy.manyPerProfileNotice}
          </p>
        ) : null}
        {typeId && fields.data ? (
          <CustomFieldInputGrid
            disabled={save.isPending}
            draft={draft}
            fields={fields.data.fields.filter((value) => value.active)}
            onChange={setDraft}
          />
        ) : null}
        {typeId ? (
          <Flex gap="0.5rem">
            <Button
              disabled={
                save.isPending ||
                (selectedType?.profile_cardinality === "ONE_PER_PROFILE" && !ownerId)
              }
              onClick={() => save.mutate()}
              type="primary"
            >
              {editing ? copy.saveEntity : copy.createEntity}
            </Button>
            {editing ? (
              <Button
                onClick={() => {
                  setEditing(null);
                  setDraft({});
                }}
              >
                {copy.cancel}
              </Button>
            ) : null}
          </Flex>
        ) : null}
      </Card>
      <div className="custom-admin-list">
        {entities.data?.entities.map((value, index) => (
          <Card key={value.id} className="custom-admin-card">
            <Flex vertical gap="0.75rem">
              <strong>
                {selectedType?.label ?? copy.defaultEntityName} #{index + 1}
              </strong>
              <span>{value.owner_profile_id ? copy.profileLinked : copy.profileUnlinked}</span>
              <Flex gap="0.5rem">
                <Button
                  onClick={() => {
                    setEditing(value);
                    setDraft(draftFromStoredValues(value.values));
                  }}
                >
                  {copy.edit}
                </Button>
                <Button danger disabled={remove.isPending} onClick={() => remove.mutate(value)}>
                  {copy.delete}
                </Button>
              </Flex>
            </Flex>
          </Card>
        ))}
      </div>
    </Flex>
  );
}
