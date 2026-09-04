import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FileSpreadsheet, FileText, User, Zap } from "lucide-react";
import type { ReactNode } from "react";
import { listGoogleFormsSources, refreshGoogleFormsSource } from "../api/googleForms";
import type { OperationImport } from "../api/operations";
import { listOperationImports } from "../api/operations";
import type { TableKind } from "./cadastroSearch";

/**
 * Entry screen of mock 011: inventory counts plus three entry cards
 * ("Por tipo", "Via formulário", "Em massa"). Counts are real totals from the
 * list endpoints; the form badge uses the newest active source and the bulk
 * footer the latest import. Palette and radii come from shell.css tokens.
 */
function EntryCard({
  icon,
  title,
  body,
  badge,
  middle,
  foot,
  onActivate,
  variant,
}: {
  icon: ReactNode;
  title: string;
  body: string;
  badge?: string;
  middle: ReactNode;
  foot: ReactNode;
  onActivate: () => void;
  variant?: "people" | "documents" | "bills" | "forms" | "bulk";
}) {
  return (
    <div
      className={`cadastro-entry__card cadastro-entry__card--${variant ?? "default"}`}
      role="button"
      tabIndex={0}
      onClick={onActivate}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onActivate();
        }
      }}
    >
      {badge ? <span className="cadastro-entry__pill-badge">{badge}</span> : null}
      <span aria-hidden="true" className="cadastro-entry__icon">
        {icon}
      </span>
      <span className="cadastro-entry__title">{title}</span>
      <span className="cadastro-entry__body">{body}</span>
      {middle}
      <span className="cadastro-entry__foot">{foot}</span>
    </div>
  );
}

export function CadastroEntryScreen({
  copy,
  canUseForms,
  onSelectTable,
  onOpenForms,
  onOpenBulk,
}: {
  copy: {
    entryPeopleTitle: string;
    entryPeopleBody: string;
    entryDocsTitle: string;
    entryDocsBody: string;
    entryBillsTitle: string;
    entryBillsBody: string;
    entryDirectSection: string;
    entryAutomationSection: string;
    entryFormsTitle: string;
    entryFormsBody: string;
    entryFormsBadge: string;
    entryFormsBadgeFallback: string;
    entryFormsSync: string;
    entryFormsFoot: string;
    entryFormsFootNone: string;
    entryFormsDisabled: string;
    entryBulkTitle: string;
    entryBulkBody: string;
    entryBulkButton: string;
    entryBulkFoot: string;
    entryBulkFootNone: string;
    people: string;
    documents: string;
    bills: string;
    targetLabel: string;
    cardDirectPersonBadge: string;
    cardDirectDocumentBadge: string;
    cardDirectBillBadge: string;
    cardAutoFormsBadge: string;
    cardAutoMassBadge: string;
  };
  canUseForms: boolean;
  onSelectTable: (table: TableKind) => void;
  onOpenForms: () => void;
  onOpenBulk: () => void;
}) {
  const queryClient = useQueryClient();
  const formsSources = useQuery({
    queryKey: ["cadastro-entry-forms-sources"],
    queryFn: ({ signal }) => listGoogleFormsSources(signal),
  });
  const imports = useQuery({
    queryKey: ["cadastro-entry-imports"],
    queryFn: ({ signal }) => listOperationImports(signal),
  });

  const activeSources = (formsSources.data?.sources ?? []).filter(
    (source) => source.state === "ACTIVE",
  );
  const latestSource = activeSources[0];
  const syncMutation = useMutation({
    mutationFn: (source: (typeof activeSources)[number]) => refreshGoogleFormsSource(source),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["google-forms-sources"] });
      void queryClient.invalidateQueries({ queryKey: ["google-forms-syncs"] });
    },
  });
  const latestImport: OperationImport | undefined = (imports.data?.imports ?? []).toSorted((a, b) =>
    b.created_at.localeCompare(a.created_at),
  )[0];
  const latestRowCount = (latestImport?.inserted_count ?? 0) + (latestImport?.updated_count ?? 0);

  return (
    <section aria-label={copy.targetLabel} className="cadastro-entry">
      {/* Seção 1: Ingestão Direta por Entidade */}
      <div className="cadastro-entry__section">
        <h3 className="cadastro-entry__section-title">{copy.entryDirectSection}</h3>
        <div className="cadastro-entry__cards cadastro-entry__cards--direct">
          <EntryCard
            badge={copy.cardDirectPersonBadge}
            body={copy.entryPeopleBody}
            foot={
              <>
                <span aria-hidden="true" className="cadastro-entry__dot" />
                <span>{copy.people}</span>
              </>
            }
            icon={<User size={22} strokeWidth={1.75} />}
            middle={null}
            onActivate={() => onSelectTable("people")}
            title={copy.entryPeopleTitle}
            variant="people"
          />
          <EntryCard
            badge={copy.cardDirectDocumentBadge}
            body={copy.entryDocsBody}
            foot={
              <>
                <span aria-hidden="true" className="cadastro-entry__dot" />
                <span>{copy.documents}</span>
              </>
            }
            icon={<FileText size={22} strokeWidth={1.75} />}
            middle={null}
            onActivate={() => onSelectTable("documents")}
            title={copy.entryDocsTitle}
            variant="documents"
          />
          <EntryCard
            badge={copy.cardDirectBillBadge}
            body={copy.entryBillsBody}
            foot={
              <>
                <span aria-hidden="true" className="cadastro-entry__dot" />
                <span>{copy.bills}</span>
              </>
            }
            icon={<Zap size={22} strokeWidth={1.75} />}
            middle={null}
            onActivate={() => onSelectTable("bills")}
            title={copy.entryBillsTitle}
            variant="bills"
          />
        </div>
      </div>

      {/* Seção 2: Canais de Ingestão e Automação */}
      <div className="cadastro-entry__section">
        <h3 className="cadastro-entry__section-title">{copy.entryAutomationSection}</h3>
        <div className="cadastro-entry__cards cadastro-entry__cards--automation">
          <EntryCard
            badge={copy.cardAutoFormsBadge}
            body={copy.entryFormsBody}
            foot={
              <>
                <span aria-hidden="true" className="cadastro-entry__dot" />
                {activeSources.length > 0
                  ? copy.entryFormsFoot.replace("{n}", String(activeSources.length))
                  : copy.entryFormsFootNone}
              </>
            }
            icon={<FileText size={22} strokeWidth={1.75} />}
            middle={
              canUseForms ? (
                <span className="cadastro-entry__formsrow">
                  <span className="cadastro-entry__badge">
                    {latestSource
                      ? copy.entryFormsBadge
                          .replace("{title}", latestSource.title)
                          .replace("{n}", String(activeSources.length))
                      : copy.entryFormsBadgeFallback}
                  </span>
                  {latestSource ? (
                    <button
                      className="cadastro-entry__linkbtn"
                      disabled={syncMutation.isPending}
                      onClick={(event) => {
                        event.stopPropagation();
                        syncMutation.mutate(latestSource);
                      }}
                      type="button"
                    >
                      {copy.entryFormsSync}
                    </button>
                  ) : null}
                </span>
              ) : (
                <span className="cadastro-entry__disabled">{copy.entryFormsDisabled}</span>
              )
            }
            onActivate={onOpenForms}
            title={copy.entryFormsTitle}
            variant="forms"
          />
          <EntryCard
            badge={copy.cardAutoMassBadge}
            body={copy.entryBulkBody}
            foot={
              <>
                <span
                  aria-hidden="true"
                  className="cadastro-entry__dot cadastro-entry__dot--idle"
                />
                {latestImport
                  ? copy.entryBulkFoot
                      .replace("{n}", String(latestRowCount))
                      .replace("{time}", timeAgo(latestImport.created_at))
                  : copy.entryBulkFootNone}
              </>
            }
            icon={<FileSpreadsheet size={22} strokeWidth={1.75} />}
            middle={
              <span
                className="cadastro-entry__bulkrow"
                onClick={(event) => event.stopPropagation()}
                role="presentation"
              >
                <button className="cadastro-entry__linkbtn" onClick={onOpenBulk} type="button">
                  {copy.entryBulkButton}
                </button>
              </span>
            }
            onActivate={onOpenBulk}
            title={copy.entryBulkTitle}
            variant="bulk"
          />
        </div>
      </div>
    </section>
  );
}

function timeAgo(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const minutes = Math.max(0, Math.round((Date.now() - then) / 60_000));
  if (minutes < 1) return "agora";
  if (minutes < 60) return `há ${minutes} minuto${minutes === 1 ? "" : "s"}`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `há ${hours} hora${hours === 1 ? "" : "s"}`;
  const days = Math.round(hours / 24);
  if (days < 30) return `há ${days} dia${days === 1 ? "" : "s"}`;
  const months = Math.round(days / 30);
  return `há ${months} mês${months === 1 ? "" : "es"}`;
}
