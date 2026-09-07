import { Button } from "antd";
import { CadastroStateCard, StateCard } from "../../components/StateCard";
import { ChevronRight } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { ICON, ICON_STROKE } from "../../components/icons";
import { useI18n } from "../../i18n";
import { getOCRCapability } from "../api/ocr";
import {
  listBills,
  listDocuments,
  type BillListSearch,
  type DocumentListSearch,
} from "../api/client";
import { OcrReviewPanel } from "./OcrReviewPanel";
import type { TableKind } from "./cadastroSearch";
import { CadastroOwnerPicker } from "./CadastroOwnerPicker";
import { useAttachmentsEnabled } from "./useAttachmentsEnabled";

const DOCUMENT_LIST_SEARCH: DocumentListSearch = {
  q: "",
  document_page: 1,
  document_limit: 12,
  document_sort: "identifier_value",
  document_order: "asc",
  document_identifier: "",
  document_status: "",
  document_medium: "",
  document_type: "",
};

const BILL_LIST_SEARCH: BillListSearch = {
  q: "",
  bill_page: 1,
  bill_limit: 12,
  bill_sort: "reference_value",
  bill_order: "asc",
  bill_reference: "",
  bill_competence: "",
  bill_status: "",
  bill_medium: "",
  bill_type: "",
};

export function CadastroOcrWorkspace({
  table,
  ownerId,
  recordId,
  typeId,
  onOwner,
  onRecord,
  onCreateInstead,
}: {
  table: TableKind;
  ownerId?: string | undefined;
  recordId?: string | undefined;
  typeId?: string | undefined;
  onOwner: (ownerProfileId: string) => void;
  onRecord: (next: { table: "documents" | "bills"; id: string }) => void;
  onCreateInstead: () => void;
}) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const attachmentsEnabled = useAttachmentsEnabled();
  const capability = useQuery({
    queryKey: ["ocr-capability"],
    queryFn: ({ signal }) => getOCRCapability(signal),
  });
  const checking = !attachmentsEnabled.isFetched || capability.isLoading;
  const blocked =
    attachmentsEnabled.isFetched &&
    (attachmentsEnabled.data === false || capability.data?.enabled === false);

  if (blocked) {
    return (
      <CadastroStateCard
        description={
          attachmentsEnabled.data === false ? copy.ocrUnavailableR2 : copy.ocrUnavailableProvider
        }
        kind="warning"
        title={copy.ocrUnavailable}
      />
    );
  }

  const recordOwnerKind = table === "bills" ? "BILL" : "DOCUMENT";
  return (
    <div className="cadastro-ocr">
      {checking ? (
        <StateCard
          compact
          kind="loading"
          title={copy.ocrChecking}
          description={copy.ocrCheckingHint}
        />
      ) : null}
      <CadastroOwnerPicker ownerId={ownerId} onClear={() => onOwner("")} onSelect={onOwner} />
      {ownerId && !recordId ? (
        <CadastroRecordPicker
          ownerId={ownerId}
          table={table}
          typeId={typeId}
          onCreateInstead={onCreateInstead}
          onSelect={onRecord}
        />
      ) : null}
      {ownerId && recordId && table !== "people" ? (
        <>
          <div className="cadastro-owner__chip">
            <Button
              onClick={() => onRecord({ table: table === "bills" ? "bills" : "documents", id: "" })}
            >
              {copy.ocrChangeRecord}
            </Button>
          </div>
          <OcrReviewPanel owner={{ owner_kind: recordOwnerKind, owner_id: recordId }} />
        </>
      ) : null}
    </div>
  );
}

function CadastroRecordPicker({
  table,
  ownerId,
  typeId,
  onSelect,
  onCreateInstead,
}: {
  table: TableKind;
  ownerId: string;
  typeId?: string | undefined;
  onSelect: (next: { table: "documents" | "bills"; id: string }) => void;
  onCreateInstead: () => void;
}) {
  const { messages, t } = useI18n();
  const copy = messages.tables.cadastro;
  const showDocuments = table !== "bills";
  const showBills = table !== "documents";
  const documents = useQuery({
    queryKey: ["cadastro-ocr-documents", ownerId, typeId],
    queryFn: ({ signal }) =>
      listDocuments(ownerId, { ...DOCUMENT_LIST_SEARCH, document_type: typeId ?? "" }, signal),
    enabled: showDocuments,
  });
  const bills = useQuery({
    queryKey: ["cadastro-ocr-bills", ownerId, typeId],
    queryFn: ({ signal }) =>
      listBills(ownerId, { ...BILL_LIST_SEARCH, bill_type: typeId ?? "" }, signal),
    enabled: showBills,
  });

  const documentRows = documents.data?.documents ?? [];
  const billRows = bills.data?.bills ?? [];
  const loading = (showDocuments && documents.isLoading) || (showBills && bills.isLoading);
  const empty = !loading && documentRows.length === 0 && billRows.length === 0;
  const total = documentRows.length + billRows.length;

  return (
    <div className="cadastro-ocr__records">
      <p aria-atomic="true" className="cadastro-owner__status" role="status">
        {loading
          ? copy.ocrCheckingHint
          : empty
            ? copy.ocrNoRecords
            : t(copy.ocrRecordCount, { n: total })}
      </p>
      {showDocuments && documentRows.length ? (
        <section className="cadastro-ocr__group">
          {table === "people" ? (
            <h3 className="cadastro-ocr__heading">{copy.ocrDocumentsHeading}</h3>
          ) : null}
          <div className="home-catalog">
            <section className="home-catalog__group">
              <div className="home-catalog__rows">
                {documentRows.map((row) => (
                  <Button
                    className="home-catalog__row"
                    key={row.id}
                    type="text"
                    onClick={() => onSelect({ table: "documents", id: row.id })}
                  >
                    <span className="home-catalog__row-name">
                      {row.type.label} · {row.identifier_value}
                    </span>
                    <ChevronRight
                      aria-hidden
                      className="home-catalog__row-open"
                      size={ICON.md}
                      strokeWidth={ICON_STROKE}
                    />
                  </Button>
                ))}
              </div>
            </section>
          </div>
        </section>
      ) : null}
      {showBills && billRows.length ? (
        <section className="cadastro-ocr__group">
          {table === "people" ? (
            <h3 className="cadastro-ocr__heading">{copy.ocrBillsHeading}</h3>
          ) : null}
          <div className="home-catalog">
            <section className="home-catalog__group">
              <div className="home-catalog__rows">
                {billRows.map((row) => (
                  <Button
                    className="home-catalog__row"
                    key={row.id}
                    type="text"
                    onClick={() => onSelect({ table: "bills", id: row.id })}
                  >
                    <span className="home-catalog__row-name">
                      {row.type.label} · {row.reference_value}
                    </span>
                    <ChevronRight
                      aria-hidden
                      className="home-catalog__row-open"
                      size={ICON.md}
                      strokeWidth={ICON_STROKE}
                    />
                  </Button>
                ))}
              </div>
            </section>
          </div>
        </section>
      ) : null}
      {empty ? <Button onClick={onCreateInstead}>{copy.ocrCreateInstead}</Button> : null}
    </div>
  );
}
