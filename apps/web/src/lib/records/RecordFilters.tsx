import { Card } from "antd";
import type { BillType, DocumentType, ProfileListSearch } from "../api/client";

const media = ["PHYSICAL", "DIGITAL"] as const;

function mediumLabel(value: string) {
  return value === "DIGITAL" ? "Digital" : "Físico";
}

export function DocumentFilters({
  search,
  types,
  onSearch,
}: {
  search: ProfileListSearch;
  types: DocumentType[];
  onSearch: (patch: Partial<ProfileListSearch>) => void;
}) {
  return (
    <Card className="record-filters">
      <label>
        Identificador
        <input
          value={search.document_identifier}
          onChange={(event) =>
            onSearch({ document_identifier: event.target.value, document_page: 1 })
          }
        />
      </label>
      <label>
        Tipo
        <select
          value={search.document_type}
          onChange={(event) => onSearch({ document_type: event.target.value, document_page: 1 })}
        >
          <option value="">Todos</option>
          {types.map((value) => (
            <option key={value.id} value={value.id}>
              {value.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        Status
        <select
          value={search.document_status}
          onChange={(event) =>
            onSearch({
              document_status: event.target.value as ProfileListSearch["document_status"],
              document_page: 1,
            })
          }
        >
          <option value="">Todos</option>
          <option value="AVAILABLE">Disponível</option>
          <option value="IN_USE">Em uso</option>
        </select>
      </label>
      <label>
        Meio
        <select
          value={search.document_medium}
          onChange={(event) =>
            onSearch({
              document_medium: event.target.value as ProfileListSearch["document_medium"],
              document_page: 1,
            })
          }
        >
          <option value="">Todos</option>
          {media.map((value) => (
            <option key={value} value={value}>
              {mediumLabel(value)}
            </option>
          ))}
        </select>
      </label>
      <label>
        Ordenar
        <select
          value={`${search.document_sort}:${search.document_order}`}
          onChange={(event) => {
            const [sort, order] = event.target.value.split(":") as [
              ProfileListSearch["document_sort"],
              ProfileListSearch["document_order"],
            ];
            onSearch({ document_sort: sort, document_order: order });
          }}
        >
          <option value="identifier_value:asc">Identificador A–Z</option>
          <option value="updated_at:desc">Atualizados recentemente</option>
          <option value="document_date:desc">Data mais recente</option>
          <option value="type_label:asc">Tipo A–Z</option>
        </select>
      </label>
    </Card>
  );
}

export function BillFilters({
  search,
  types,
  onSearch,
}: {
  search: ProfileListSearch;
  types: BillType[];
  onSearch: (patch: Partial<ProfileListSearch>) => void;
}) {
  return (
    <Card className="record-filters">
      <label>
        Referência
        <input
          value={search.bill_reference}
          onChange={(event) => onSearch({ bill_reference: event.target.value, bill_page: 1 })}
        />
      </label>
      <label>
        Competência
        <input
          placeholder="AAAA-MM"
          value={search.bill_competence}
          onChange={(event) => onSearch({ bill_competence: event.target.value, bill_page: 1 })}
        />
      </label>
      <label>
        Tipo
        <select
          value={search.bill_type}
          onChange={(event) => onSearch({ bill_type: event.target.value, bill_page: 1 })}
        >
          <option value="">Todos</option>
          {types.map((value) => (
            <option key={value.id} value={value.id}>
              {value.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        Status
        <select
          value={search.bill_status}
          onChange={(event) =>
            onSearch({
              bill_status: event.target.value as ProfileListSearch["bill_status"],
              bill_page: 1,
            })
          }
        >
          <option value="">Todos</option>
          <option value="AVAILABLE">Disponível</option>
          <option value="IN_USE">Em uso</option>
        </select>
      </label>
      <label>
        Meio
        <select
          value={search.bill_medium}
          onChange={(event) =>
            onSearch({
              bill_medium: event.target.value as ProfileListSearch["bill_medium"],
              bill_page: 1,
            })
          }
        >
          <option value="">Todos</option>
          {media.map((value) => (
            <option key={value} value={value}>
              {mediumLabel(value)}
            </option>
          ))}
        </select>
      </label>
      <label>
        Ordenar
        <select
          value={`${search.bill_sort}:${search.bill_order}`}
          onChange={(event) => {
            const [sort, order] = event.target.value.split(":") as [
              ProfileListSearch["bill_sort"],
              ProfileListSearch["bill_order"],
            ];
            onSearch({ bill_sort: sort, bill_order: order });
          }}
        >
          <option value="reference_value:asc">Referência A–Z</option>
          <option value="updated_at:desc">Atualizados recentemente</option>
          <option value="competence:desc">Competência recente</option>
          <option value="amount:desc">Maior valor</option>
          <option value="type_label:asc">Tipo A–Z</option>
        </select>
      </label>
    </Card>
  );
}
