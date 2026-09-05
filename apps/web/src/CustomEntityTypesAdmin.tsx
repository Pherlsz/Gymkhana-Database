import { Alert, Button, Card, Checkbox, Flex, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
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
      deleteCustomEntityType(value.id, value.version, "Confirmar"),
    onSuccess: refresh,
  });
  const error = save.error ?? remove.error ?? query.error;
  return (
    <Flex vertical gap="1rem">
      <SectionTitle
        title="Tipos de entidade"
        description="Controle a cardinalidade por pessoa e mantenha chaves técnicas estáveis."
      />
      {error ? (
        <Alert
          message="Não foi possível concluir a operação"
          type="error"
          description={<>{customDataError(error)}</>}
        />
      ) : null}
      <Card className="custom-admin-form">
        <div className="custom-admin-grid">
          <label>
            Chave técnica
            <input
              disabled={Boolean(editing)}
              value={draft.technical_key}
              onChange={(event) => setDraft({ ...draft, technical_key: event.target.value })}
            />
          </label>
          <label>
            Nome
            <input
              value={draft.label}
              onChange={(event) => setDraft({ ...draft, label: event.target.value })}
            />
          </label>
          <label>
            Cardinalidade
            <select
              value={draft.profile_cardinality}
              onChange={(event) =>
                setDraft({
                  ...draft,
                  profile_cardinality:
                    event.target.value === "ONE_PER_PROFILE"
                      ? "ONE_PER_PROFILE"
                      : "MANY_PER_PROFILE",
                })
              }
            >
              <option value="ONE_PER_PROFILE">Uma por pessoa</option>
              <option value="MANY_PER_PROFILE">Várias por pessoa</option>
            </select>
          </label>
          <Checkbox
            checked={draft.active}
            className="custom-admin-check"
            onChange={(event) => setDraft({ ...draft, active: event.target.checked })}
          >
            Ativo
          </Checkbox>
        </div>
        <Flex>
          <Button
            disabled={!draft.technical_key || !draft.label || save.isPending}
            onClick={() => save.mutate()}
          >
            {editing ? "Salvar tipo" : "Criar tipo"}
          </Button>
          {editing ? (
            <Button
              onClick={() => {
                setEditing(undefined);
                setDraft(emptyType);
              }}
            >
              Cancelar
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
                  {value.active ? "Ativo" : "Inativo"}
                </Tag>
              </Flex>
              <span>
                {value.profile_cardinality === "ONE_PER_PROFILE"
                  ? "Uma por pessoa"
                  : "Várias por pessoa"}
              </span>
              <Flex>
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
