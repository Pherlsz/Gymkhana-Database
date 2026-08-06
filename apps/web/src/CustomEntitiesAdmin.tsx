import { Alert, Button, Card, Flex } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  CustomFieldInputGrid,
  customDataError,
  customInputsFromDraft,
  draftFromStoredValues,
  type CustomDraftValue,
} from "./CustomValuesPanel";
import { listProfilesForSelection } from "./lib/api/client";
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
  const types = useQuery({
    queryKey: ["custom-entity-types"],
    queryFn: ({ signal }) => listCustomEntityTypes(signal),
  });
  const profiles = useQuery({
    queryKey: ["profiles", "selection"],
    queryFn: ({ signal }) => listProfilesForSelection(signal),
  });
  const [typeId, setTypeId] = useState("");
  const [ownerId, setOwnerId] = useState("");
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
    mutationFn: (value: CustomEntity) => deleteCustomEntity(value.id, value.version, "Confirmar"),
    onSuccess: async () => queryClient.invalidateQueries({ queryKey: ["custom-entities", typeId] }),
  });
  useEffect(() => {
    setEditing(null);
    setDraft({});
  }, [typeId, ownerId]);
  return (
    <Flex vertical gap="1rem">
      <SectionTitle
        title="Entidades personalizadas"
        description="Cadastre itens como veículos, equipamentos ou qualquer outro conjunto tipado reutilizável."
      />
      {save.isError || remove.isError || entities.isError || fields.isError ? (
        <Alert message="Não foi possível concluir a operação" type="error" description={<>{customDataError(save.error ?? remove.error ?? entities.error ?? fields.error)}
        </>} />
      ) : null}
      <Card className="custom-admin-form" style={{ padding: "1rem" }}>
        <div className="custom-admin-grid">
          <label>
            Tipo
            <select value={typeId} onChange={(event) => setTypeId(event.target.value)}>
              <option value="">Selecione</option>
              {types.data?.types
                .filter((value) => value.active)
                .map((value) => (
                  <option key={value.id} value={value.id}>
                    {value.label}
                  </option>
                ))}
            </select>
          </label>
          <label>
            Pessoa vinculada
            <select value={ownerId} onChange={(event) => setOwnerId(event.target.value)}>
              <option value="">Sem vínculo / todas</option>
              {profiles.data?.profiles.map((value) => (
                <option key={value.id} value={value.id}>
                  {value.full_name}
                </option>
              ))}
            </select>
          </label>
        </div>
        {selectedType ? (
          <p>
            {selectedType.profile_cardinality === "ONE_PER_PROFILE"
              ? "Este tipo aceita uma entidade por pessoa."
              : "Este tipo aceita várias entidades por pessoa."}
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
          <Flex>
            <Button
              disabled={
                save.isPending ||
                (selectedType?.profile_cardinality === "ONE_PER_PROFILE" && !ownerId)
              }
              onClick={() => save.mutate()}
            >
              {editing ? "Salvar entidade" : "Criar entidade"}
            </Button>
            {editing ? (
              <Button
                onClick={() => {
                  setEditing(null);
                  setDraft({});
                }}
              >
                Cancelar
              </Button>
            ) : null}
          </Flex>
        ) : null}
      </Card>
      <div className="custom-admin-list">
        {entities.data?.entities.map((value, index) => (
          <Card key={value.id} className="custom-admin-card" style={{ padding: "1rem" }}>
            <Flex vertical gap="0.75rem">
              <strong>
                {selectedType?.label ?? "Entidade"} #{index + 1}
              </strong>
              <span>
                {value.owner_profile_id
                  ? (profiles.data?.profiles.find(
                      (profile) => profile.id === value.owner_profile_id,
                    )?.full_name ?? "Pessoa vinculada")
                  : "Sem pessoa vinculada"}
              </span>
              <Flex>
                <Button
                  onClick={() => {
                    setEditing(value);
                    setDraft(draftFromStoredValues(value.values));
                  }}
                >
                  Editar
                </Button>
                <Button disabled={remove.isPending} onClick={() => remove.mutate(value)}>
                  Excluir
                </Button>
              </Flex>
            </Flex>
          </Card>
        ))}
      </div>
    </Flex>
  );
}
