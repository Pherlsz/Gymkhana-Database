import { Alert, Button, Card, Checkbox, Flex, Input, Select, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useI18n } from "./i18n";
import { customDataError } from "./CustomValuesPanel";
import { emptyType, SectionTitle } from "./CustomDataShared";
import {
  createCustomEntityType,
  deleteCustomEntityType,
  listCustomEntityTypes,
  updateCustomEntityType,
  type CustomEntityType,
} from "./lib/api/customdata";

export function CustomEntityTypesAdmin() {
  const client = useQueryClient();
  const { messages } = useI18n();
  const copy = messages.admin.customEntityTypes;
  const [draft, setDraft] = useState(emptyType);
  const [editing, setEditing] = useState<CustomEntityType>();
  const query = useQuery({
    queryKey: ["custom-entity-types"],
    queryFn: ({ signal }) => listCustomEntityTypes(signal),
  });
  const refresh = () => client.invalidateQueries({ queryKey: ["custom-entity-types"] });
  const save = useMutation({
    mutationFn: () =>
      editing
        ? updateCustomEntityType(editing.id, editing.version, draft)
        : createCustomEntityType(draft),
    onSuccess: async () => {
      setEditing(undefined);
      setDraft(emptyType);
      await refresh();
    },
  });
  const remove = useMutation({
    mutationFn: (value: CustomEntityType) =>
      deleteCustomEntityType(value.id, value.version, messages.common.actions.confirm),
    onSuccess: refresh,
  });
  const error = save.error ?? remove.error ?? query.error;
  return (
    <Flex vertical gap="1rem">
      <SectionTitle
        title={copy.title}
        description={copy.description}
      />
      {error ? (
        <Alert
          message={copy.errorTitle}
          type="error"
          description={<>{customDataError(error)}</>}
        />
      ) : null}
      <Card className="custom-admin-form">
        <div className="custom-admin-grid">
          <label>
            {copy.technicalKey}
            <Input
              disabled={Boolean(editing)}
              value={draft.technical_key}
              onChange={(event) => setDraft({ ...draft, technical_key: event.target.value })}
            />
          </label>
          <label>
            {copy.name}
            <Input
              value={draft.label}
              onChange={(event) => setDraft({ ...draft, label: event.target.value })}
            />
          </label>
          <label>
            {copy.cardinality}
            <Select
              value={draft.profile_cardinality}
              onChange={(value) =>
                setDraft({
                  ...draft,
                  profile_cardinality:
                    value === "ONE_PER_PROFILE"
                      ? "ONE_PER_PROFILE"
                      : "MANY_PER_PROFILE",
                })
              }
              options={[
                { value: "ONE_PER_PROFILE", label: copy.onePerProfile },
                { value: "MANY_PER_PROFILE", label: copy.manyPerProfile },
              ]}
            />
          </label>
          <Checkbox
            checked={draft.active}
            className="custom-admin-check"
            onChange={(event) => setDraft({ ...draft, active: event.target.checked })}
          >
            {copy.active}
          </Checkbox>
        </div>
        <Flex gap="0.5rem">
          <Button
            disabled={!draft.technical_key || !draft.label || save.isPending}
            onClick={() => save.mutate()}
            type="primary"
          >
            {editing ? copy.saveType : copy.createType}
          </Button>
          {editing ? (
            <Button
              onClick={() => {
                setEditing(undefined);
                setDraft(emptyType);
              }}
            >
              {copy.cancel}
            </Button>
          ) : null}
        </Flex>
      </Card>
      <div className="custom-admin-list">
        {query.data?.types.map((value) => (
          <Card className="custom-admin-card" key={value.id}>
            <Flex vertical gap="0.75rem">
              <Flex className="custom-admin-card__header">
                <div>
                  <strong>{value.label}</strong>
                  <p>{value.technical_key}</p>
                </div>
                <Tag color={value.active ? "success" : "neutral"}>
                  {value.active ? copy.active : copy.inactive}
                </Tag>
              </Flex>
              <span>
                {value.profile_cardinality === "ONE_PER_PROFILE"
                  ? copy.onePerProfile
                  : copy.manyPerProfile}
              </span>
              <Flex gap="0.5rem">
                <Button
                  onClick={() => {
                    setEditing(value);
                    setDraft({
                      technical_key: value.technical_key,
                      label: value.label,
                      active: value.active,
                      profile_cardinality: value.profile_cardinality,
                    });
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
