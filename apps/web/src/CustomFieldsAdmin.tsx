import { Alert, Button, Card, Flex, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  emptyField,
  fieldKinds,
  fieldKindLabel,
  fieldValues,
  numberOrUndefined,
  SectionTitle,
} from "./CustomDataShared";
import { CustomOptionsAdmin } from "./CustomOptionsAdmin";
import { customDataError } from "./CustomValuesPanel";
import { listBillTypes, listDocumentTypes } from "./lib/api/client";
import {
  createCustomField,
  deleteCustomField,
  listCustomEntityTypes,
  listCustomFields,
  updateCustomField,
  type CustomField,
  type CustomFieldKind,
  type CustomFieldValues,
  type CustomTargetKind,
} from "./lib/api/customdata";

function scopedFieldValues(
  values: CustomFieldValues,
  targetKind: CustomTargetKind,
  targetId: string,
): CustomFieldValues {
  const { target_id: _targetId, ...rest } = values;
  return {
    ...rest,
    target_kind: targetKind,
    ...(targetKind !== "PROFILE" && targetId ? { target_id: targetId } : {}),
  };
}

function withOptionalNumber(
  values: CustomFieldValues,
  key: "minimum_length" | "maximum_length",
  rawValue: string,
): CustomFieldValues {
  const next = { ...values };
  delete next[key];
  const parsed = numberOrUndefined(rawValue);
  if (parsed !== undefined) next[key] = parsed;
  return next;
}

function withOptionalString(
  values: CustomFieldValues,
  key: "validation_regex",
  rawValue: string,
): CustomFieldValues {
  const next = { ...values };
  delete next[key];
  if (rawValue !== "") next[key] = rawValue;
  return next;
}

export function CustomFieldsAdmin() {
  const queryClient = useQueryClient();
  const types = useQuery({
    queryKey: ["custom-entity-types"],
    queryFn: ({ signal }) => listCustomEntityTypes(signal),
  });
  const documentTypes = useQuery({
    queryKey: ["document-types"],
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const billTypes = useQuery({
    queryKey: ["bill-types"],
    queryFn: ({ signal }) => listBillTypes(signal),
  });
  const [targetKind, setTargetKind] = useState<CustomTargetKind>("PROFILE");
  const [targetId, setTargetId] = useState("");
  const [draft, setDraft] = useState<CustomFieldValues>(emptyField);
  const [editing, setEditing] = useState<CustomField | null>(null);
  const [selected, setSelected] = useState<CustomField | null>(null);
  const fields = useQuery({
    queryKey: ["custom-fields", targetKind, targetId || "global"],
    queryFn: ({ signal }) => listCustomFields(targetKind, targetId || undefined, signal),
    enabled: targetKind === "PROFILE" || Boolean(targetId),
  });

  useEffect(() => {
    setDraft((current) => scopedFieldValues(current, targetKind, targetId));
    setEditing(null);
    setSelected(null);
  }, [targetKind, targetId]);

  const save = useMutation({
    mutationFn: () =>
      editing ? updateCustomField(editing.id, editing.version, draft) : createCustomField(draft),
    onSuccess: async () => {
      setEditing(null);
      setDraft(scopedFieldValues(emptyField, targetKind, targetId));
      await queryClient.invalidateQueries({ queryKey: ["custom-fields", targetKind] });
    },
  });
  const remove = useMutation({
    mutationFn: (value: CustomField) => deleteCustomField(value.id, value.version, "Confirmar"),
    onSuccess: async () => {
      setSelected(null);
      await queryClient.invalidateQueries({ queryKey: ["custom-fields", targetKind] });
    },
  });
  const targets =
    targetKind === "DOCUMENT_TYPE"
      ? (documentTypes.data?.types.map((value) => ({ id: value.id, label: value.label })) ?? [])
      : targetKind === "BILL_TYPE"
        ? (billTypes.data?.types.map((value) => ({ id: value.id, label: value.label })) ?? [])
        : targetKind === "CUSTOM_ENTITY_TYPE"
          ? (types.data?.types.map((value) => ({ id: value.id, label: value.label })) ?? [])
          : [];

  return (
    <Flex vertical gap="1rem">
      <SectionTitle
        title="Campos personalizados"
        description="Campos pertencem a um contexto e preservam o tipo depois que recebem valores."
      />
      {save.isError || remove.isError || fields.isError ? (
        <Alert message="Não foi possível concluir a operação" type="error" description={<>{customDataError(save.error ?? remove.error ?? fields.error)}
        </>} />
      ) : null}
      <Card className="custom-admin-form" style={{ padding: "1rem" }}>
        <div className="custom-admin-grid">
          <label>
            Contexto
            <select
              value={targetKind}
              onChange={(event) => {
                setTargetKind(event.target.value as CustomTargetKind);
                setTargetId("");
              }}
            >
              <option value="PROFILE">Pessoa</option>
              <option value="DOCUMENT_TYPE">Tipo de documento</option>
              <option value="BILL_TYPE">Tipo de conta/comprovante</option>
              <option value="CUSTOM_ENTITY_TYPE">Tipo de entidade</option>
            </select>
          </label>
          {targetKind !== "PROFILE" ? (
            <label>
              Alvo
              <select value={targetId} onChange={(event) => setTargetId(event.target.value)}>
                <option value="">Selecione</option>
                {targets.map((value) => (
                  <option key={value.id} value={value.id}>
                    {value.label}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
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
            Tipo do valor
            <select
              disabled={Boolean(editing)}
              value={draft.field_kind}
              onChange={(event) =>
                setDraft({ ...draft, field_kind: event.target.value as CustomFieldKind })
              }
            >
              {fieldKinds.map((value) => (
                <option key={value} value={value}>
                  {fieldKindLabel(value)}
                </option>
              ))}
            </select>
          </label>
          <label>
            Tamanho mínimo
            <input
              inputMode="numeric"
              value={draft.minimum_length ?? ""}
              onChange={(event) =>
                setDraft((current) =>
                  withOptionalNumber(current, "minimum_length", event.target.value),
                )
              }
            />
          </label>
          <label>
            Tamanho máximo
            <input
              inputMode="numeric"
              value={draft.maximum_length ?? ""}
              onChange={(event) =>
                setDraft((current) =>
                  withOptionalNumber(current, "maximum_length", event.target.value),
                )
              }
            />
          </label>
          <label>
            Regex opcional
            <input
              value={draft.validation_regex ?? ""}
              onChange={(event) =>
                setDraft((current) =>
                  withOptionalString(current, "validation_regex", event.target.value),
                )
              }
            />
          </label>
          <label className="custom-admin-check">
            <input
              checked={draft.required}
              type="checkbox"
              onChange={(event) => setDraft({ ...draft, required: event.target.checked })}
            />
            Obrigatório
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
        <Flex>
          <Button
            disabled={
              save.isPending ||
              !draft.technical_key ||
              !draft.label ||
              (targetKind !== "PROFILE" && !targetId)
            }
            onClick={() => save.mutate()}
          >
            {editing ? "Salvar campo" : "Criar campo"}
          </Button>
          {editing ? (
            <Button
              onClick={() => {
                setEditing(null);
                setDraft(scopedFieldValues(emptyField, targetKind, targetId));
              }}
            >
              Cancelar
            </Button>
          ) : null}
        </Flex>
      </Card>
      <div className="custom-admin-list">
        {fields.data?.fields.map((value) => (
          <Card key={value.id} className="custom-admin-card" style={{ padding: "1rem" }}>
            <Flex vertical gap="0.75rem">
              <Flex align="center" className="custom-admin-card__header">
                <div>
                  <strong>{value.label}</strong>
                  <p>
                    {value.technical_key} · {fieldKindLabel(value.field_kind)}
                  </p>
                </div>
                <Tag color={value.active ? "success" : "neutral"}>
                  {value.active ? "Ativo" : "Inativo"}
                </Tag>
              </Flex>
              <Flex>
                <Button
                  onClick={() => {
                    setEditing(value);
                    setDraft(fieldValues(value));
                  }}
                >
                  Editar
                </Button>
                {value.field_kind === "SINGLE_SELECT" || value.field_kind === "MULTI_SELECT" ? (
                  <Button onClick={() => setSelected(value)}>Opções</Button>
                ) : null}
                <Button disabled={remove.isPending} onClick={() => remove.mutate(value)}>
                  Excluir
                </Button>
              </Flex>
            </Flex>
          </Card>
        ))}
      </div>
      {selected ? <CustomOptionsAdmin field={selected} onClose={() => setSelected(null)} /> : null}
    </Flex>
  );
}
