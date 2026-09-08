// Minimum-requirement indicator for the manual cadastro flow.
// A person needs at least one official document (CPF, RG, CNH, birth
// certificate). Rendered as a thin text line under the step trail — never an
// Alert/banner (that treatment was rejected in the 011 design review).
// No official document: soft amber note naming the official list.
// At least one: quiet "ok" line with the type label.
import { useQuery } from "@tanstack/react-query";
import { useI18n } from "../../i18n";
import { listDocuments, type DocumentListSearch } from "../api/client";

export const OFFICIAL_DOCUMENT_TYPE_KEYS = ["cpf", "rg", "cnh", "birth_certificate"];
export const MINIMUM_REQUIREMENT_QUERY = ["cadastro-minimum-requirement"];

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
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const query = useQuery({
    queryKey: [...MINIMUM_REQUIREMENT_QUERY, ownerId],
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
        {copy.minimumOk.replace("{label}", first.type.label)}
      </p>
    );
  }
  return <p className="cadastro-minreq cadastro-minreq--warn">{copy.minimumRequired}</p>;
}
