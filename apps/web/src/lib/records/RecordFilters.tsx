import { Card } from "antd";
import type { ReactNode } from "react";
import { useI18n } from "../../i18n";
import type { BillType, DocumentType, ProfileListSearch } from "../api/client";

export function RecordFiltersCard(props: {
  children?: ReactNode;
  types: Array<{ id: string; label: string }>;
  selectedType: string;
  onTypeChange: (type: string) => void;
  status: string;
  onStatusChange: (status: "AVAILABLE" | "IN_USE" | "") => void;
  medium: string;
  onMediumChange: (medium: "PHYSICAL" | "DIGITAL" | "") => void;
  sortValue: string;
  onSortChange: (sort: string, order: "asc" | "desc") => void;
  sortOptions: Array<{ value: string; label: string }>;
}) {
  const { messages } = useI18n();

  const mediaOptions = [
    { value: "", label: messages.common.labels.all },
    { value: "PHYSICAL", label: messages.common.labels.physical },
    { value: "DIGITAL", label: messages.common.labels.digital },
  ];

  return (
    <Card className="record-filters">
      {props.children}
      <label>
        {messages.common.labels.type}
        <select value={props.selectedType} onChange={(e) => props.onTypeChange(e.target.value)}>
          <option value="">{messages.common.labels.all}</option>
          {props.types.map((type) => (
            <option key={type.id} value={type.id}>
              {type.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        {messages.common.labels.status}
        <select
          value={props.status}
          onChange={(e) => props.onStatusChange(e.target.value as "AVAILABLE" | "IN_USE" | "")}
        >
          <option value="">{messages.common.labels.all}</option>
          <option value="AVAILABLE">{messages.common.status.available}</option>
          <option value="IN_USE">{messages.common.status.inUse}</option>
        </select>
      </label>
      <label>
        {messages.common.labels.medium}
        <select
          value={props.medium}
          onChange={(e) => props.onMediumChange(e.target.value as "PHYSICAL" | "DIGITAL" | "")}
        >
          {mediaOptions.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </label>
      <label>
        {messages.common.labels.order}
        <select
          value={props.sortValue}
          onChange={(e) => {
            const [sort, order] = e.target.value.split(":") as [string, "asc" | "desc"];
            props.onSortChange(sort, order);
          }}
        >
          {props.sortOptions.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </label>
    </Card>
  );
}

export function RecordFilters({
  kind,
  search,
  types,
  onSearch,
}: {
  kind: "document" | "bill";
  search: ProfileListSearch;
  types: Array<{ id: string; label: string }>;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
}) {
  const { messages } = useI18n();
  const isDoc = kind === "document";

  const sortOptions = isDoc
    ? [
        { value: "identifier_value:asc", label: messages.records.filters.sortDocIdAsc },
        { value: "updated_at:desc", label: messages.records.filters.sortUpdatedDesc },
        { value: "document_date:desc", label: messages.records.filters.sortDateDesc },
        { value: "type_label:asc", label: messages.records.filters.sortTypeAsc },
      ]
    : [
        { value: "reference_value:asc", label: messages.records.filters.sortBillRefAsc },
        { value: "updated_at:desc", label: messages.records.filters.sortUpdatedDesc },
        { value: "competence:desc", label: messages.records.filters.sortCompetenceDesc },
        { value: "amount:desc", label: messages.records.filters.sortAmountDesc },
        { value: "type_label:asc", label: messages.records.filters.sortTypeAsc },
      ];

  return (
    <RecordFiltersCard
      medium={isDoc ? search.document_medium : search.bill_medium}
      onMediumChange={(medium) =>
        onSearch(
          isDoc
            ? { document_medium: medium, document_page: 1 }
            : { bill_medium: medium, bill_page: 1 },
        )
      }
      onSortChange={(sort, order) =>
        onSearch(
          isDoc
            ? { document_sort: sort as ProfileListSearch["document_sort"], document_order: order }
            : { bill_sort: sort as ProfileListSearch["bill_sort"], bill_order: order },
        )
      }
      onStatusChange={(status) =>
        onSearch(
          isDoc
            ? { document_status: status, document_page: 1 }
            : { bill_status: status, bill_page: 1 },
        )
      }
      onTypeChange={(type) =>
        onSearch(
          isDoc ? { document_type: type, document_page: 1 } : { bill_type: type, bill_page: 1 },
        )
      }
      selectedType={isDoc ? search.document_type : search.bill_type}
      sortOptions={sortOptions}
      sortValue={
        isDoc
          ? `${search.document_sort}:${search.document_order}`
          : `${search.bill_sort}:${search.bill_order}`
      }
      status={isDoc ? search.document_status : search.bill_status}
      types={types}
    >
      {isDoc ? (
        <label>
          {messages.common.labels.identifier}
          <input
            onChange={(e) => onSearch({ document_identifier: e.target.value, document_page: 1 })}
            value={search.document_identifier}
          />
        </label>
      ) : (
        <>
          <label>
            {messages.common.labels.reference}
            <input
              onChange={(e) => onSearch({ bill_reference: e.target.value, bill_page: 1 })}
              value={search.bill_reference}
            />
          </label>
          <label>
            {messages.common.labels.competence}
            <input
              onChange={(e) => onSearch({ bill_competence: e.target.value, bill_page: 1 })}
              placeholder={messages.common.placeholders.competence}
              value={search.bill_competence}
            />
          </label>
        </>
      )}
    </RecordFiltersCard>
  );
}

export function DocumentFilters(props: {
  search: ProfileListSearch;
  types: DocumentType[];
  onSearch: (patch: Partial<ProfileListSearch>) => void;
}) {
  return <RecordFilters kind="document" {...props} />;
}

export function BillFilters(props: {
  search: ProfileListSearch;
  types: BillType[];
  onSearch: (patch: Partial<ProfileListSearch>) => void;
}) {
  return <RecordFilters kind="bill" {...props} />;
}
