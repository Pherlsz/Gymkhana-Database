import { useQuery } from "@tanstack/react-query";
import { useI18n } from "../../i18n";
import { listDocuments, type DocumentListSearch } from "../api/client";
import { queryKeys } from "../api/queryKeys";

export const OFFICIAL_DOCUMENT_TYPE_KEYS = ["cpf", "rg", "cnh", "birth_certificate"];
export const MINIMUM_REQUIREMENT_QUERY = queryKeys.cadastro.minimumRequirement();

const SEARCH: DocumentListSearch = {
  q: "",
  document_page: 1,
  document_limit: 50,
  document_sort: "identifier_value",
  document_order: "asc",
  document_identifier: "",
  document_status: "",
  document_medium: "",
  document_type: "",
};

export function CadastroMinimumRequirement({ ownerId }: { ownerId: string }) {
  const { messages, t } = useI18n();
  const copy = messages.tables.cadastro;
  const query = useQuery({
    queryKey: queryKeys.cadastro.minimumRequirement(ownerId),
    queryFn: ({ signal }) => listDocuments(ownerId, SEARCH, signal),
    staleTime: 5_000,
  });
  if (query.isLoading) return null;
  const official = (query.data?.documents ?? []).filter((document) =>
    OFFICIAL_DOCUMENT_TYPE_KEYS.includes(document.type.technical_key),
  );
  if (official.length > 0) {
    const first = official[0]!;
    return (
      <p className="cadastro-minreq cadastro-minreq--ok">
        {t(copy.minimumOk, { label: first.type.label })}
      </p>
    );
  }
  return <p className="cadastro-minreq cadastro-minreq--warn">{copy.minimumRequired}</p>;
}
