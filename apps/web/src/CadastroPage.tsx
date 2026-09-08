import { Alert, Button, Spin } from "antd";
import { ArrowLeft } from "lucide-react";
import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { lazy, Suspense, useCallback, useState } from "react";
import { PageHeader } from "./components/PageHeader";
import { useI18n } from "./i18n";
import { CadastroEntryScreen } from "./lib/cadastro/CadastroEntryScreen";
import { CadastroSingleScreen } from "./lib/cadastro/CadastroSingleScreen";
import "./cadastro.css";

const GoogleFormsPage = lazy(() =>
  import("./GoogleFormsPage").then((m) => ({ default: m.GoogleFormsPage })),
);
const CadastroPanel = lazy(() =>
  import("./lib/cadastro/CadastroPanel").then((m) => ({ default: m.CadastroPanel })),
);
import {
  type CadastroPageSearch,
  cadastroWorkMode,
  moduleFromTable,
  normalizeCadastroPageSearch,
} from "./lib/cadastro/cadastroSearch";

const cadastroRoute = getRouteApi("/cadastro");

export function CadastroPage() {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const search = cadastroRoute.useSearch();
  const navigate = useNavigate();
  const [notice, setNotice] = useState<string | null>(null);
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

  return (
    <div className="cadastro-page page-measure">
      {entryActive ? (
        <>
          <PageHeader description={copy.description} title={copy.title} />
          {notice ? (
            <Alert closable message={notice} type="success" onClose={() => setNotice(null)} />
          ) : null}
          <CadastroEntryScreen />
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
            <Button
              className="cadastro-crumb__btn"
              type="text"
              size="small"
              icon={<ArrowLeft size={13} />}
              onClick={goToEntry}
            >
              {copy.crumbHome}
            </Button>
            <span className="cadastro-crumb__sep">/</span>
            <span className="cadastro-crumb__current">{copy.modeForms}</span>
          </nav>
          <Suspense fallback={<Spin size="large" style={{ display: "block", margin: "3rem auto" }} />}>
            <GoogleFormsPage defaultModule={moduleFromTable(search.table)} />
          </Suspense>
        </div>
      ) : null}

      {!entryActive && (work === "xlsx" || work === "ocr") ? (
        <div className="cadastro-work">
          <nav aria-label={copy.crumbHome} className="cadastro-crumb">
            <Button
              className="cadastro-crumb__btn"
              type="text"
              size="small"
              icon={<ArrowLeft size={13} />}
              onClick={goToEntry}
            >
              {copy.crumbHome}
            </Button>
            <span className="cadastro-crumb__sep">/</span>
            <span className="cadastro-crumb__current">
              {work === "xlsx" ? copy.modeXlsx : copy.modeOcr}
            </span>
          </nav>
          <Suspense fallback={<Spin size="large" style={{ display: "block", margin: "3rem auto" }} />}>
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
          </Suspense>
        </div>
      ) : null}
    </div>
  );
}
