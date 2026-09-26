import { useQuery } from "@tanstack/react-query";
import { ClipboardList, FileSpreadsheet, IdCard, User, Zap } from "lucide-react";
import { useNavigate } from "@tanstack/react-router";
import { listGoogleFormsSources } from "../api/googleForms";
import type { OperationImport } from "../api/operations";
import { listOperationImports } from "../api/operations";
import { APIRequestError } from "../api/client";
import { normalizeCadastroPageSearch, type TableKind } from "./cadastroSearch";
import { queryKeys } from "../api/queryKeys";
import { AppCard } from "../../components/AppCard";
import { useI18n } from "../../i18n";
import { useApplicationSession } from "../../session";
import { canManageUsers } from "../roles";

export interface CadastroEntryScreenProps {
  onSelectTable?: (table: TableKind) => void;
  onOpenForms?: () => void;
  onOpenBulk?: () => void;
  canUseForms?: boolean;
}

export function CadastroEntryScreen({
  canUseForms: propCanUseForms,
  onSelectTable,
  onOpenForms,
  onOpenBulk,
}: CadastroEntryScreenProps = {}) {
  const { messages, t } = useI18n();
  const copy = messages.tables.cadastro;
  const session = useApplicationSession();
  const canUseForms = propCanUseForms ?? canManageUsers(session.user.role);
  const navigate = useNavigate();

  const handleSelectTable =
    onSelectTable ??
    ((table: TableKind) => {
      void navigate({
        to: "/cadastro",
        search: (current) =>
          normalizeCadastroPageSearch({
            ...current,
            table,
            mode: "manual",
            type: undefined,
            owner: undefined,
            record: undefined,
            import: undefined,
          }),
      });
    });

  const handleOpenForms =
    onOpenForms ??
    (() => {
      void navigate({
        to: "/cadastro",
        search: (current) =>
          normalizeCadastroPageSearch({
            ...current,
            table: "people",
            mode: "forms",
            tab: "sources",
            type: undefined,
            owner: undefined,
            record: undefined,
            import: undefined,
          }),
      });
    });

  const handleOpenBulk =
    onOpenBulk ??
    (() => {
      void navigate({
        to: "/cadastro",
        search: (current) =>
          normalizeCadastroPageSearch({
            ...current,
            table: "people",
            mode: "xlsx",
            type: undefined,
            owner: undefined,
            record: undefined,
            import: undefined,
          }),
      });
    });
  const formsSources = useQuery({
    queryKey: queryKeys.cadastro.entryFormsSources,
    queryFn: ({ signal }) => listGoogleFormsSources(signal),
  });
  const imports = useQuery({
    queryKey: queryKeys.cadastro.entryImports,
    queryFn: ({ signal }) => listOperationImports(signal),
    retry: (count, error) =>
      !(error instanceof APIRequestError && error.status === 503) && count < 2,
  });

  const activeSources = (formsSources.data?.sources ?? []).filter(
    (source) => source.state === "ACTIVE",
  );
  const latestImport: OperationImport | undefined = (imports.data?.imports ?? []).toSorted((a, b) =>
    b.created_at.localeCompare(a.created_at),
  )[0];
  const latestRowCount = (latestImport?.inserted_count ?? 0) + (latestImport?.updated_count ?? 0);

  return (
    <section aria-label={copy.targetLabel} className="cadastro-entry">
      <div className="cadastro-entry__section">
        <h3 className="cadastro-entry__section-title">{copy.entryDirectSection}</h3>
        <div className="cadastro-entry__cards cadastro-entry__cards--direct">
          <AppCard
            badge={copy.cardDirectPersonBadge}
            icon={<User size={22} strokeWidth={1.75} />}
            onClick={() => handleSelectTable("people")}
            title={copy.entryPeopleTitle}
            variant="people"
          >
            <p className="app-card__desc">{copy.entryPeopleBody}</p>
          </AppCard>
          <AppCard
            badge={copy.cardDirectDocumentBadge}
            icon={<IdCard size={22} strokeWidth={1.75} />}
            onClick={() => handleSelectTable("documents")}
            title={copy.entryDocsTitle}
            variant="documents"
          >
            <p className="app-card__desc">{copy.entryDocsBody}</p>
          </AppCard>
          <AppCard
            badge={copy.cardDirectBillBadge}
            icon={<Zap size={22} strokeWidth={1.75} />}
            onClick={() => handleSelectTable("bills")}
            title={copy.entryBillsTitle}
            variant="bills"
          >
            <p className="app-card__desc">{copy.entryBillsBody}</p>
          </AppCard>
        </div>
      </div>

      <div className="cadastro-entry__section">
        <h3 className="cadastro-entry__section-title">{copy.entryAutomationSection}</h3>
        <div className="cadastro-entry__cards cadastro-entry__cards--automation">
          <AppCard
            badge={copy.cardAutoFormsBadge}
            icon={<ClipboardList size={22} strokeWidth={1.75} />}
            onClick={handleOpenForms}
            title={copy.entryFormsTitle}
            variant="forms"
          >
            <p className="app-card__desc">{copy.entryFormsBody}</p>
            {canUseForms ? null : (
              <span className="cadastro-entry__disabled">{copy.entryFormsDisabled}</span>
            )}
            <div className="app-card__foot">
              <span aria-hidden="true" className="cadastro-entry__dot" />
              {activeSources.length > 0
                ? t(copy.entryFormsFoot, { n: activeSources.length })
                : copy.entryFormsFootNone}
            </div>
          </AppCard>
          <AppCard
            badge={copy.cardAutoMassBadge}
            icon={<FileSpreadsheet size={22} strokeWidth={1.75} />}
            onClick={handleOpenBulk}
            title={copy.entryBulkTitle}
            variant="bulk"
          >
            <p className="app-card__desc">{copy.entryBulkBody}</p>
            <div className="app-card__foot">
              <span aria-hidden="true" className="cadastro-entry__dot cadastro-entry__dot--idle" />
              {imports.isError
                ? copy.entryBulkFootUnavailable
                : latestImport
                  ? t(copy.entryBulkFoot, {
                      n: latestRowCount,
                      time: timeAgo(latestImport.created_at),
                    })
                  : copy.entryBulkFootNone}
            </div>
          </AppCard>
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
