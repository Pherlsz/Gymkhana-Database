import { getRouteApi, useNavigate } from "@tanstack/react-router";
import { useCallback, useState } from "react";
import { PageShell } from "./components/PageShell";
import { StatusBanner } from "./components/StatusBanner";
import { useI18n } from "./i18n";
import { CadastroEntryScreen } from "./lib/cadastro/CadastroEntryScreen";
import { CadastroPanel } from "./lib/cadastro/CadastroPanel";
import { CadastroSingleScreen } from "./lib/cadastro/CadastroSingleScreen";
import { CadastroWorkShell } from "./lib/cadastro/CadastroWork";
import { GoogleFormsPage } from "./lib/cadastro/forms/GoogleFormsPage";
import "./cadastro.css";
import {
  type CadastroPageSearch,
  cadastroWorkMode,
  moduleFromTable,
  normalizeCadastroPageSearch,
  tableFromModule,
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

  const goToXlsxList = useCallback(() => {
    patchSearch({ import: undefined, mode: "xlsx" });
  }, [patchSearch]);

  const goBackXlsx = useCallback(() => {
    if (search.import) {
      goToXlsxList();
      return;
    }
    goToEntry();
  }, [goToEntry, goToXlsxList, search.import]);

  const entryActive =
    search.table === "people" &&
    !search.mode &&
    !search.type &&
    !search.owner &&
    !search.record &&
    !search.import;

  return (
    <PageShell
      className="cadastro-page"
      description={entryActive ? copy.description : undefined}
      measure
      title={entryActive ? copy.title : undefined}
    >
      {entryActive ? (
        <>
          {notice ? (
            <StatusBanner closable title={notice} tone="success" onClose={() => setNotice(null)} />
          ) : null}
          <CadastroEntryScreen />
        </>
      ) : null}

      {!entryActive && work === "manual" ? (
        <CadastroSingleScreen
          targetTable={search.table}
          typeId={search.type}
          onCancel={goToEntry}
          onSuccess={() => {
            goToEntry();
          }}
        />
      ) : null}

      {!entryActive && work === "forms" ? (
        <CadastroWorkShell backLabel={copy.crumbHome} current={copy.modeForms} onBack={goToEntry}>
          <GoogleFormsPage defaultModule={moduleFromTable(search.table)} />
        </CadastroWorkShell>
      ) : null}

      {!entryActive && work === "xlsx" ? (
        <CadastroWorkShell
          backLabel={search.import ? copy.modeXlsx : copy.crumbHome}
          current={search.import ? undefined : copy.modeXlsx}
          onBack={goBackXlsx}
        >
          <CadastroPanel
            importId={search.import}
            table={search.table}
            onImportChange={(importId) => patchSearch({ import: importId })}
            onModuleChange={(next) => patchSearch({ table: tableFromModule(next) })}
            onNotice={setNotice}
          />
        </CadastroWorkShell>
      ) : null}
    </PageShell>
  );
}
