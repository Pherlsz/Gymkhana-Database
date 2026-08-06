import { Alert, Stack, Surface } from "./ui";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { SectionTitle } from "./CustomDataShared";
import { CustomValuesPanel, customDataError } from "./CustomValuesPanel";
import { listBills, listDocuments, listProfilesForSelection } from "./lib/api/client";

type Target = "profile" | "document" | "bill";

export function CustomRecordValuesAdmin() {
  const [target, setTarget] = useState<Target>("profile");
  const [profileId, setProfileId] = useState("");
  const [recordId, setRecordId] = useState("");
  const profiles = useQuery({
    queryKey: ["profiles", "selection"],
    queryFn: ({ signal }) => listProfilesForSelection(signal),
  });
  const documents = useQuery({
    queryKey: ["documents", "custom-values", profileId],
    queryFn: ({ signal }) =>
      listDocuments(
        profileId,
        {
          document_page: 1,
          document_limit: 1000,
          document_sort: "type_label",
          document_order: "asc",
          document_identifier: "",
          document_status: "",
          document_state: "",
          document_type: "",
        },
        signal,
      ),
    enabled: target === "document" && Boolean(profileId),
  });
  const bills = useQuery({
    queryKey: ["bills", "custom-values", profileId],
    queryFn: ({ signal }) =>
      listBills(
        profileId,
        {
          bill_page: 1,
          bill_limit: 1000,
          bill_sort: "type_label",
          bill_order: "asc",
          bill_reference: "",
          bill_competence: "",
          bill_status: "",
          bill_state: "",
          bill_type: "",
        },
        signal,
      ),
    enabled: target === "bill" && Boolean(profileId),
  });
  useEffect(() => setRecordId(""), [target, profileId]);
  const selectedDocument = useMemo(
    () => documents.data?.documents.find((value) => value.id === recordId),
    [documents.data, recordId],
  );
  const selectedBill = useMemo(
    () => bills.data?.bills.find((value) => value.id === recordId),
    [bills.data, recordId],
  );
  const error = profiles.error ?? documents.error ?? bills.error;
  return (
    <Stack gap="4">
      <SectionTitle
        title="Valores por registro"
        description="Edite os campos tipados de pessoas, documentos e contas sem sair da área de dados personalizados."
      />
      {error ? (
        <Alert title="Não foi possível carregar os registros" tone="danger">
          {customDataError(error)}
        </Alert>
      ) : null}
      <Surface className="custom-admin-form" tone="raised">
        <div className="custom-admin-grid">
          <label>
            Registro
            <select value={target} onChange={(event) => setTarget(event.target.value as Target)}>
              <option value="profile">Pessoa</option>
              <option value="document">Documento</option>
              <option value="bill">Conta/comprovante</option>
            </select>
          </label>
          <label>
            Pessoa
            <select value={profileId} onChange={(event) => setProfileId(event.target.value)}>
              <option value="">Selecione</option>
              {profiles.data?.profiles.map((profile) => (
                <option key={profile.id} value={profile.id}>
                  {profile.full_name}
                </option>
              ))}
            </select>
          </label>
          {target === "document" ? (
            <label>
              Documento
              <select value={recordId} onChange={(event) => setRecordId(event.target.value)}>
                <option value="">Selecione</option>
                {documents.data?.documents.map((record) => (
                  <option key={record.id} value={record.id}>
                    {record.type.label} · {record.identifier_value}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
          {target === "bill" ? (
            <label>
              Conta/comprovante
              <select value={recordId} onChange={(event) => setRecordId(event.target.value)}>
                <option value="">Selecione</option>
                {bills.data?.bills.map((record) => (
                  <option key={record.id} value={record.id}>
                    {record.type.label} · {record.reference_value}
                  </option>
                ))}
              </select>
            </label>
          ) : null}
        </div>
      </Surface>
      {target === "profile" && profileId ? (
        <CustomValuesPanel
          definitionTargetKind="PROFILE"
          valueTargetKind="profile"
          valueTargetId={profileId}
        />
      ) : null}
      {target === "document" && selectedDocument ? (
        <CustomValuesPanel
          definitionTargetKind="DOCUMENT_TYPE"
          definitionTargetId={selectedDocument.document_type_id}
          valueTargetKind="document"
          valueTargetId={selectedDocument.id}
        />
      ) : null}
      {target === "bill" && selectedBill ? (
        <CustomValuesPanel
          definitionTargetKind="BILL_TYPE"
          definitionTargetId={selectedBill.bill_type_id}
          valueTargetKind="bill"
          valueTargetId={selectedBill.id}
        />
      ) : null}
    </Stack>
  );
}
