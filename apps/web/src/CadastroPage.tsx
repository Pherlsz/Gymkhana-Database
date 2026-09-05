import { Alert } from "antd";
import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { useCallback, useState } from "react";
import { PageHeader } from "./components/PageHeader";
import { GoogleFormsPage } from "./GoogleFormsPage";
import { useI18n } from "./i18n";
import { CadastroEntryScreen } from "./lib/cadastro/CadastroEntryScreen";
import { CadastroPanel } from "./lib/cadastro/CadastroPanel";
import { CadastroSingleScreen } from "./lib/cadastro/CadastroSingleScreen";
import {
  type CadastroPageSearch,
  type TableKind,
  cadastroWorkMode,
  moduleFromTable,
  normalizeCadastroPageSearch,
} from "./lib/cadastro/cadastroSearch";
import { canManageUsers } from "./lib/roles";
import { useApplicationSession } from "./session";

const cadastroRoute = getRouteApi("/cadastro");

export function CadastroPage() {
  const session = useApplicationSession();
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const search = cadastroRoute.useSearch();
  const navigate = useNavigate();
  const [notice, setNotice] = useState<string | null>(null);
  const canUseForms = canManageUsers(session.user.role);
  const work = cadastroWorkMode(search);

  const patchSearch = useCallback(
    (patch: Partial<CadastroPageSearch>) => {
      void navigate({
        to: "/cadastro",
        search: (current) => normalizeCadastroPageSearch({ ...current, ...patch }),
      });
    },
    [navigate],
  );

  const goToEntry = useCallback(() => {
    patchSearch({
      table: "people",
      mode: undefined,
      type: undefined,
      owner: undefined,
      record: undefined,
      import: undefined,
      tab: undefined,
      source: undefined,
      google_forms: undefined,
    });
  }, [patchSearch]);

  const entryActive =
    search.table === "people" &&
    !search.mode &&
    !search.type &&
    !search.owner &&
    !search.record &&
    !search.import;

  const entrySelectTable = useCallback(
    (table: TableKind) => {
      patchSearch({
        table,
        mode: "manual",
        type: undefined,
        owner: undefined,
        record: undefined,
        import: undefined,
      });
    },
    [patchSearch],
  );

  const entryOpenForms = useCallback(() => {
    patchSearch({
      table: "people",
      mode: "forms",
      tab: "sources",
      type: undefined,
      owner: undefined,
      record: undefined,
      import: undefined,
    });
  }, [patchSearch]);

  const entryOpenBulk = useCallback(() => {
    patchSearch({
      table: "people",
      mode: "xlsx",
      type: undefined,
      owner: undefined,
      record: undefined,
      import: undefined,
    });
  }, [patchSearch]);

  return (
    <div className="cadastro-page page-measure">
      {entryActive ? (
        <>
          <PageHeader description={copy.description} title={copy.title} />
          {notice ? (
            <Alert closable message={notice} type="success" onClose={() => setNotice(null)} />
          ) : null}
          <CadastroEntryScreen
            canUseForms={canUseForms}
            copy={{
              bills: copy.bills,
              documents: copy.documents,
              entryAutomationSection: copy.entryAutomationSection,
              entryBillsBody: copy.entryBillsBody,
              entryBillsTitle: copy.entryBillsTitle,
              entryBulkBody: copy.entryBulkBody,
              entryBulkButton: copy.entryBulkButton,
              entryBulkFoot: copy.entryBulkFoot,
              entryBulkFootNone: copy.entryBulkFootNone,
              entryBulkTitle: copy.entryBulkTitle,
              entryDirectSection: copy.entryDirectSection,
              entryDocsBody: copy.entryDocsBody,
              entryDocsTitle: copy.entryDocsTitle,
              entryFormsBadge: copy.entryFormsBadge,
              entryFormsBadgeFallback: copy.entryFormsBadgeFallback,
              entryFormsBody: copy.entryFormsBody,
              entryFormsDisabled: copy.entryFormsDisabled,
              entryFormsFoot: copy.entryFormsFoot,
              entryFormsFootNone: copy.entryFormsFootNone,
              entryFormsSync: copy.entryFormsSync,
              entryFormsTitle: copy.entryFormsTitle,
              entryPeopleBody: copy.entryPeopleBody,
              entryPeopleTitle: copy.entryPeopleTitle,
              people: copy.people,
              targetLabel: copy.targetLabel,
              cardDirectPersonBadge: copy.cardDirectPersonBadge,
              cardDirectDocumentBadge: copy.cardDirectDocumentBadge,
              cardDirectBillBadge: copy.cardDirectBillBadge,
              cardAutoFormsBadge: copy.cardAutoFormsBadge,
              cardAutoMassBadge: copy.cardAutoMassBadge,
            }}
            onOpenBulk={entryOpenBulk}
            onOpenForms={entryOpenForms}
            onSelectTable={entrySelectTable}
          />
        </>
      ) : null}

      {!entryActive && (work === "manual" || work === "catalog") ? (
        <CadastroSingleScreen
          targetTable={search.table}
          onCancel={goToEntry}
          onSuccess={() => {
            goToEntry();
          }}
        />
      ) : null}

      {!entryActive && work === "forms" ? (
        <div className="cadastro-work">
          <nav aria-label={copy.crumbHome} className="cadastro-crumb">
            <button className="cadastro-crumb__btn" type="button" onClick={goToEntry}>
              ← {copy.crumbHome}
            </button>
            <span className="cadastro-crumb__sep">/</span>
            <span className="cadastro-crumb__current">{copy.modeForms}</span>
          </nav>
          <GoogleFormsPage defaultModule={moduleFromTable(search.table)} />
        </div>
      ) : null}

      {!entryActive && (work === "xlsx" || work === "ocr") ? (
        <div className="cadastro-work">
          <nav aria-label={copy.crumbHome} className="cadastro-crumb">
            <button className="cadastro-crumb__btn" type="button" onClick={goToEntry}>
              ← {copy.crumbHome}
            </button>
            <span className="cadastro-crumb__sep">/</span>
            <span className="cadastro-crumb__current">
              {work === "xlsx" ? copy.modeXlsx : copy.modeOcr}
            </span>
          </nav>
          <CadastroPanel
            cadastro={work}
            importId={search.import}
            recordId={search.record}
            recordsOwner={search.owner}
            table={search.table}
            typeId={search.type}
            onClearOwner={() => patchSearch({ owner: undefined, record: undefined })}
            onCreateInstead={() =>
              patchSearch({
                mode: "manual",
                record: undefined,
                owner: search.owner,
                table: search.table === "people" ? "documents" : search.table,
              })
            }
            onImportChange={(importId) => patchSearch({ import: importId })}
            onOwner={(ownerProfileId) =>
              patchSearch({
                owner: ownerProfileId || undefined,
                record: undefined,
              })
            }
            onRecord={(next) =>
              patchSearch({
                table: next.id ? next.table : search.table,
                record: next.id || undefined,
                owner: search.owner,
                mode: "ocr",
              })
            }
          />
        </div>
      ) : null}
    </div>
  );
}
