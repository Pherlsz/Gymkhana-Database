import { Alert, Button, Inline, Stack, StatusBadge, Surface } from "./ui";
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
    <Stack gap="4">
      <SectionTitle
        title="Tipos de entidade"
        description="Controle a cardinalidade por pessoa e mantenha chaves técnicas estáveis."
      />
      {error ? (
        <Alert title="Não foi possível concluir a operação" tone="danger">
          {customDataError(error)}
        </Alert>
      ) : null}
      <Surface className="custom-admin-form" tone="raised">
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
          <label className="custom-admin-check">
            <input
              checked={draft.active}
              type="checkbox"
              onChange={(event) => setDraft({ ...draft, active: event.target.checked })}
            />
            Ativo
          </label>
        </div>
        <Inline>
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
        </Inline>
      </Surface>
      <div className="custom-admin-list">
        {query.data?.types.map((value) => (
          <Surface className="custom-admin-card" key={value.id} tone="raised">
            <Stack gap="3">
              <Inline className="custom-admin-card__header">
                <div>
                  <strong>{value.label}</strong>
                  <p>{value.technical_key}</p>
                </div>
                <StatusBadge tone={value.active ? "success" : "neutral"}>
                  {value.active ? "Ativo" : "Inativo"}
                </StatusBadge>
              </Inline>
              <span>
                {value.profile_cardinality === "ONE_PER_PROFILE"
                  ? "Uma por pessoa"
                  : "Várias por pessoa"}
              </span>
              <Inline>
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
              </Inline>
            </Stack>
          </Surface>
        ))}
      </div>
    </Stack>
  );
}
