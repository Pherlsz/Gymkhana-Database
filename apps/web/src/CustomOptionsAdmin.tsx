import { Alert, Button, Card, Flex } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { customDataError } from "./CustomValuesPanel";
import { emptyOption, optionValues } from "./CustomDataShared";
import {
  createCustomOption,
  deleteCustomOption,
  listCustomOptions,
  updateCustomOption,
  type CustomField,
  type CustomOption,
} from "./lib/api/customdata";

export function CustomOptionsAdmin({
  field,
  onClose,
}: {
  field: CustomField;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(emptyOption);
  const [editing, setEditing] = useState<CustomOption | null>(null);
  const query = useQuery({
    queryKey: ["custom-field-options", field.id],
    queryFn: ({ signal }) => listCustomOptions(field.id, signal),
  });
  const save = useMutation({
    mutationFn: () =>
      editing
        ? updateCustomOption(field.id, editing.id, editing.version, draft)
        : createCustomOption(field.id, draft),
    onSuccess: async () => {
      setDraft(emptyOption);
      setEditing(null);
      await queryClient.invalidateQueries({ queryKey: ["custom-field-options", field.id] });
    },
  });
  const remove = useMutation({
    mutationFn: (value: CustomOption) =>
      deleteCustomOption(field.id, value.id, value.version, "Confirmar"),
    onSuccess: async () =>
      queryClient.invalidateQueries({ queryKey: ["custom-field-options", field.id] }),
  });
  return (
    <Card className="custom-admin-form">
      <Flex vertical gap="1rem">
        <Flex align="center" className="custom-admin-card__header">
          <div>
            <strong>Opções de {field.label}</strong>
            <p>Opções inativas continuam legíveis em valores históricos.</p>
          </div>
          <Button onClick={onClose}>Fechar</Button>
        </Flex>
        {save.isError || remove.isError || query.isError ? (
          <Alert
            message="Não foi possível alterar opções"
            type="error"
            description={<>{customDataError(save.error ?? remove.error ?? query.error)}</>}
          />
        ) : null}
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
            Ordem
            <input
              inputMode="numeric"
              value={draft.sort_order}
              onChange={(event) =>
                setDraft({ ...draft, sort_order: Number(event.target.value) || 0 })
              }
            />
          </label>
          <label className="custom-admin-check">
            <input
              checked={draft.active}
              type="checkbox"
              onChange={(event) => setDraft({ ...draft, active: event.target.checked })}
            />
            Ativa
          </label>
        </div>
        <Flex>
          <Button
            disabled={!draft.technical_key || !draft.label || save.isPending}
            onClick={() => save.mutate()}
          >
            {editing ? "Salvar opção" : "Criar opção"}
          </Button>
          {editing ? (
            <Button
              onClick={() => {
                setEditing(null);
                setDraft(emptyOption);
              }}
            >
              Cancelar
            </Button>
          ) : null}
        </Flex>
        <div className="custom-option-list">
          {query.data?.options.map((value) => (
            <div key={value.id} className="custom-option-row">
              <span>
                {value.label} · {value.technical_key}
              </span>
              <Flex>
                <Button
                  onClick={() => {
                    setEditing(value);
                    setDraft(optionValues(value));
                  }}
                >
                  Editar
                </Button>
                <Button onClick={() => remove.mutate(value)}>Excluir</Button>
              </Flex>
            </div>
          ))}
        </div>
      </Flex>
    </Card>
  );
}
