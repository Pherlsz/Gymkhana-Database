#!/usr/bin/env python3
from pathlib import Path


def replace_once(text: str, old: str, new: str) -> str:
    if old not in text:
        raise RuntimeError(f"anchor not found: {old[:120]!r}")
    return text.replace(old, new, 1)


profiles_path = Path("apps/web/src/ProfilesPage.tsx")
text = profiles_path.read_text(encoding="utf-8")
text = replace_once(
    text,
    'import { useEffect, useMemo, useState } from "react";\n',
    'import { useEffect, useMemo, useState } from "react";\nimport { ProfileRecordsPanel } from "./ProfileRecordsPanel";\n',
)
text = replace_once(
    text,
    '  type ProfileValuesRequest,\n} from "./lib/api/client";',
    '  type ProfileValuesRequest,\n  type UserRole,\n} from "./lib/api/client";',
)
old_search = '''    mode:
      search.mode === "create" || search.mode === "edit" || search.mode === "view"
        ? search.mode
        : undefined,
  };
}'''
new_search = '''    mode:
      search.mode === "create" || search.mode === "edit" || search.mode === "view"
        ? search.mode
        : undefined,
    section:
      search.section === "documents" || search.section === "bills" ? search.section : "profile",
    document_page: positiveInteger(search.document_page, 1),
    document_limit: Math.min(1000, Math.max(100, positiveInteger(search.document_limit, 100))),
    document_sort: ["identifier_value", "type_label", "document_date", "created_at", "updated_at"].includes(String(search.document_sort))
      ? (search.document_sort as ProfileListSearch["document_sort"])
      : "identifier_value",
    document_order: search.document_order === "desc" ? "desc" : "asc",
    document_identifier: typeof search.document_identifier === "string" ? search.document_identifier : "",
    document_status: search.document_status === "AVAILABLE" || search.document_status === "IN_USE" ? search.document_status : "",
    document_state: ["CURRENT", "REPLACED", "EXPIRED", "ARCHIVED"].includes(String(search.document_state))
      ? (search.document_state as ProfileListSearch["document_state"])
      : "",
    document_type: typeof search.document_type === "string" ? search.document_type : "",
    document_selected: typeof search.document_selected === "string" ? search.document_selected : undefined,
    document_mode: ["create", "view", "edit", "types"].includes(String(search.document_mode))
      ? (search.document_mode as ProfileListSearch["document_mode"])
      : undefined,
    bill_page: positiveInteger(search.bill_page, 1),
    bill_limit: Math.min(1000, Math.max(100, positiveInteger(search.bill_limit, 100))),
    bill_sort: ["reference_value", "type_label", "competence", "amount", "created_at", "updated_at"].includes(String(search.bill_sort))
      ? (search.bill_sort as ProfileListSearch["bill_sort"])
      : "reference_value",
    bill_order: search.bill_order === "desc" ? "desc" : "asc",
    bill_reference: typeof search.bill_reference === "string" ? search.bill_reference : "",
    bill_competence: typeof search.bill_competence === "string" ? search.bill_competence : "",
    bill_status: search.bill_status === "AVAILABLE" || search.bill_status === "IN_USE" ? search.bill_status : "",
    bill_state: ["CURRENT", "REPLACED", "EXPIRED", "ARCHIVED"].includes(String(search.bill_state))
      ? (search.bill_state as ProfileListSearch["bill_state"])
      : "",
    bill_type: typeof search.bill_type === "string" ? search.bill_type : "",
    bill_selected: typeof search.bill_selected === "string" ? search.bill_selected : undefined,
    bill_mode: ["create", "view", "edit", "types"].includes(String(search.bill_mode))
      ? (search.bill_mode as ProfileListSearch["bill_mode"])
      : undefined,
  };
}'''
text = replace_once(text, old_search, new_search)
text = text.replace("<Page.Eyebrow>M3 · Profiles</Page.Eyebrow>", "<Page.Eyebrow>M4 · Profiles e registros</Page.Eyebrow>")
text = text.replace(
    "Cadastre, filtre, edite e duplique pessoas físicas. Filtros, ordenação e paginação\n          permanecem na URL.",
    "Cadastre pessoas e gerencie seus documentos, contas e comprovantes. Todo o estado de\n          navegação permanece na URL.",
)
text = replace_once(
    text,
    '''          onClose={() => updateSearch({ selected: undefined, mode: undefined })}
          onEdit={() => updateSearch({ mode: "edit" })}''',
    '''          section={search.section}
          search={search}
          role={session.user.role}
          onSearch={updateSearch}
          onNotice={(message) => setNotice(message)}
          onClose={() =>
            updateSearch({
              selected: undefined,
              mode: undefined,
              document_selected: undefined,
              document_mode: undefined,
              bill_selected: undefined,
              bill_mode: undefined,
            })
          }
          onEdit={() => updateSearch({ mode: "edit", section: "profile" })}''',
)
start = text.index("function ProfilePanel(props: {")
end = text.index("function profileValues(value: Profile)")
new_panel = r'''function ProfilePanel(props: {
  mode: "create" | "view" | "edit";
  profile: Profile | undefined;
  canDelete: boolean;
  pending: boolean;
  section: ProfileListSearch["section"];
  search: ProfileListSearch;
  role: UserRole;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
  onClose: () => void;
  onEdit: () => void;
  onSaved: (value: Profile, message: string) => Promise<void>;
  onDuplicate: (value: Profile) => void;
  onDelete: (value: Profile, confirmation: string) => void;
}) {
  const [values, setValues] = useState<ProfileValuesRequest>(
    props.profile ? profileValues(props.profile) : emptyValues,
  );
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const editable = props.mode === "create" || props.mode === "edit";
  const set = (field: keyof ProfileValuesRequest, value: string) =>
    setValues((current) => ({ ...current, [field]: value }));
  const setAddress = (field: keyof ProfileValuesRequest["address"], value: string) =>
    setValues((current) => ({ ...current, address: { ...current.address, [field]: value } }));
  const submit = async () => {
    if (!values.full_name.trim()) {
      setError("Nome completo é obrigatório.");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const saved = props.profile
        ? await updateProfile(props.profile.id, { ...values, version: props.profile.version })
        : await createProfile(values);
      await props.onSaved(saved, props.profile ? "Pessoa atualizada." : "Pessoa criada.");
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setSaving(false);
    }
  };
  if (props.mode !== "create" && !props.profile)
    return (
      <aside className="profile-panel">
        <Alert title="Pessoa não encontrada" tone="danger">
          Atualize a lista e tente novamente.
        </Alert>
        <Button onClick={props.onClose}>Fechar</Button>
      </aside>
    );
  return (
    <aside aria-label="Detalhes da pessoa" className="profile-panel">
      <div className="profile-panel__header">
        <div>
          <span className="profile-panel__eyebrow">
            {props.mode === "create" ? "Nova pessoa" : "Perfil"}
          </span>
          <h2>{props.profile?.full_name || "Cadastrar pessoa"}</h2>
        </div>
        <Button onClick={props.onClose}>Fechar</Button>
      </div>
      {props.profile && props.mode !== "create" ? (
        <nav aria-label="Seções da pessoa" className="profile-sections">
          <button
            className={props.section === "profile" ? "profile-sections__active" : undefined}
            onClick={() => props.onSearch({ section: "profile" })}
          >
            Perfil
          </button>
          <button
            className={props.section === "documents" ? "profile-sections__active" : undefined}
            onClick={() => props.onSearch({ section: "documents" })}
          >
            Documentos
          </button>
          <button
            className={props.section === "bills" ? "profile-sections__active" : undefined}
            onClick={() => props.onSearch({ section: "bills" })}
          >
            Contas e comprovantes
          </button>
        </nav>
      ) : null}
      {props.profile && props.section !== "profile" && props.mode !== "create" ? (
        <ProfileRecordsPanel
          profile={props.profile}
          role={props.role}
          search={props.search}
          section={props.section}
          onNotice={props.onNotice}
          onSearch={props.onSearch}
        />
      ) : (
        <>
          {error ? (
            <Alert title="Não foi possível salvar" tone="danger">
              {error}
            </Alert>
          ) : null}
          <div className="profile-form">
            <label>
              Nome completo
              <input
                disabled={!editable}
                required
                value={values.full_name}
                onChange={(event) => set("full_name", event.target.value)}
              />
            </label>
            <label>
              Nome social
              <input
                disabled={!editable}
                value={values.social_name}
                onChange={(event) => set("social_name", event.target.value)}
              />
            </label>
            <label>
              CPF
              <input
                disabled={!editable}
                inputMode="numeric"
                value={values.cpf}
                onChange={(event) => set("cpf", event.target.value)}
              />
            </label>
            <label>
              E-mail
              <input
                disabled={!editable}
                type="email"
                value={values.email}
                onChange={(event) => set("email", event.target.value)}
              />
            </label>
            <label>
              Celular
              <input
                disabled={!editable}
                value={values.mobile_phone}
                onChange={(event) => set("mobile_phone", event.target.value)}
              />
            </label>
            <label>
              Telefone fixo/outro
              <input
                disabled={!editable}
                value={values.landline_phone}
                onChange={(event) => set("landline_phone", event.target.value)}
              />
            </label>
            <label>
              Logradouro
              <input
                disabled={!editable}
                value={values.address.street}
                onChange={(event) => setAddress("street", event.target.value)}
              />
            </label>
            <label>
              Número
              <input
                disabled={!editable}
                value={values.address.number}
                onChange={(event) => setAddress("number", event.target.value)}
              />
            </label>
            <label>
              Complemento
              <input
                disabled={!editable}
                value={values.address.complement}
                onChange={(event) => setAddress("complement", event.target.value)}
              />
            </label>
            <label>
              Bairro
              <input
                disabled={!editable}
                value={values.address.neighborhood}
                onChange={(event) => setAddress("neighborhood", event.target.value)}
              />
            </label>
            <label>
              Cidade
              <input
                disabled={!editable}
                value={values.address.city}
                onChange={(event) => setAddress("city", event.target.value)}
              />
            </label>
            <label>
              UF
              <input
                disabled={!editable}
                maxLength={2}
                value={values.address.state}
                onChange={(event) => setAddress("state", event.target.value.toUpperCase())}
              />
            </label>
            <label>
              CEP
              <input
                disabled={!editable}
                inputMode="numeric"
                value={values.address.postal_code}
                onChange={(event) => setAddress("postal_code", event.target.value)}
              />
            </label>
            <label className="profile-form__wide">
              Observações
              <textarea
                disabled={!editable}
                rows={4}
                value={values.notes}
                onChange={(event) => set("notes", event.target.value)}
              />
            </label>
          </div>
          <Inline className="profile-panel__actions">
            {editable ? (
              <Button disabled={saving} onClick={() => void submit()}>
                {saving ? "Salvando" : "Salvar"}
              </Button>
            ) : (
              <Button onClick={props.onEdit}>Editar</Button>
            )}
            {props.profile ? (
              <Button disabled={props.pending} onClick={() => props.onDuplicate(props.profile!)}>
                Duplicar
              </Button>
            ) : null}
          </Inline>
          {props.profile && props.canDelete ? (
            <Surface className="profile-delete" tone="raised">
              <Stack gap="3">
                <strong>Exclusão permanente</strong>
                <span>Digite Confirmar para excluir esta pessoa.</span>
                <input
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                />
                <Button
                  disabled={confirmation !== "Confirmar" || props.pending}
                  onClick={() => props.onDelete(props.profile!, confirmation)}
                >
                  Excluir permanentemente
                </Button>
              </Stack>
            </Surface>
          ) : null}
        </>
      )}
    </aside>
  );
}
'''
text = text[:start] + new_panel + text[end:]
profiles_path.write_text(text, encoding="utf-8")

styles_path = Path("apps/web/src/styles.css")
styles = styles_path.read_text(encoding="utf-8")
styles += r'''

.profile-sections {
  display: flex;
  gap: var(--gym-space-2);
  overflow: auto;
  margin-block-end: var(--gym-space-5);
  padding-block-end: var(--gym-space-2);
  border-block-end: 1px solid var(--gym-color-border);
}
.profile-sections button,
.type-list-item {
  min-block-size: 2.5rem;
  padding: var(--gym-space-2) var(--gym-space-3);
  border: 1px solid transparent;
  border-radius: var(--gym-radius-md);
  background: transparent;
  color: var(--gym-color-text-muted);
  cursor: pointer;
  white-space: nowrap;
}
.profile-sections button:hover,
.profile-sections__active {
  border-color: var(--gym-color-border);
  background: var(--gym-color-surface-raised);
  color: var(--gym-color-text);
}
.records-section-header,
.record-editor__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--gym-space-4);
}
.records-section-header h3,
.record-editor__header h3 {
  margin: 0;
}
.records-section-header p {
  margin-block-start: var(--gym-space-1);
  color: var(--gym-color-text-muted);
}
.record-filters,
.record-form,
.type-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--gym-space-3);
  padding: var(--gym-space-4);
}
.record-filters label,
.record-form label,
.type-form label,
.current-use label {
  display: grid;
  gap: var(--gym-space-2);
  color: var(--gym-color-text-muted);
  font-size: var(--gym-font-size-sm);
}
.record-filters input,
.record-filters select,
.record-form input,
.record-form select,
.record-form textarea,
.current-use select,
.record-delete input,
.type-form input,
.type-form select,
.type-delete input {
  min-block-size: 2.5rem;
  padding: var(--gym-space-2) var(--gym-space-3);
  border: 1px solid var(--gym-color-border);
  border-radius: var(--gym-radius-md);
  background: var(--gym-color-surface);
  color: var(--gym-color-text);
}
.record-form__wide {
  grid-column: 1 / -1;
}
.records-list {
  overflow: hidden;
}
.records-table-wrap {
  overflow: auto;
  max-block-size: 48vh;
}
.records-table {
  inline-size: 100%;
  min-inline-size: 48rem;
  border-collapse: collapse;
}
.records-table th,
.records-table td {
  padding: var(--gym-space-3);
  border-block-end: 1px solid var(--gym-color-border);
  text-align: start;
  vertical-align: middle;
}
.records-table th {
  position: sticky;
  inset-block-start: 0;
  z-index: 1;
  background: var(--gym-color-surface);
  color: var(--gym-color-text-muted);
  font-size: var(--gym-font-size-sm);
}
.records-empty {
  padding: var(--gym-space-6);
  color: var(--gym-color-text-muted);
}
.records-cards {
  display: none;
}
.records-pagination {
  justify-content: space-between;
}
.record-card,
.record-editor,
.current-use,
.record-delete {
  padding: var(--gym-space-4);
}
.record-editor {
  border: 1px solid var(--gym-color-border);
}
.record-editor__actions {
  margin-block: var(--gym-space-4);
}
.current-use,
.record-delete {
  margin-block-start: var(--gym-space-4);
}
.record-delete,
.type-delete {
  border: 1px solid var(--gym-color-danger, var(--gym-color-border));
}
.types-admin {
  display: grid;
  grid-template-columns: minmax(12rem, 1fr) minmax(18rem, 2fr);
  gap: var(--gym-space-4);
}
.type-list {
  display: grid;
  align-content: start;
  padding: var(--gym-space-2);
}
.type-list-item {
  display: grid;
  gap: var(--gym-space-1);
  text-align: start;
}
.type-list-item span {
  font-size: var(--gym-font-size-sm);
}
.record-checkbox {
  display: flex !important;
  align-items: center;
  grid-template-columns: auto 1fr;
}
.record-checkbox input {
  min-block-size: auto;
}
.type-delete {
  display: grid;
  gap: var(--gym-space-2);
  grid-column: 1 / -1;
  padding: var(--gym-space-3);
}
@media (max-width: 48rem) {
  .record-filters,
  .record-form,
  .type-form,
  .types-admin {
    grid-template-columns: 1fr;
  }
  .record-form__wide,
  .type-delete {
    grid-column: auto;
  }
  .records-table-wrap {
    display: none;
  }
  .records-cards {
    display: grid;
    gap: var(--gym-space-3);
    padding: var(--gym-space-3);
  }
  .records-pagination,
  .records-section-header,
  .record-editor__header {
    align-items: stretch;
    flex-direction: column;
  }
}
'''
styles_path.write_text(styles, encoding="utf-8")

test_path = Path("apps/web/src/ProfileRecordsPanel.test.tsx")
test_path.write_text(r'''import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ProfileRecordsPanel } from "./ProfileRecordsPanel";
import { normalizeProfileSearch } from "./ProfilesPage";

const profile = {
  id: "019bf789-4400-7f12-9abc-123456789abc",
  full_name: "Ana da Silva",
  social_name: "Ana",
  cpf: "52998224725",
  email: "ana@example.com",
  mobile_phone: "+5551999998888",
  landline_phone: "",
  address: { street: "Rua A", number: "1", complement: "", neighborhood: "Centro", city: "Porto Alegre", state: "RS", postal_code: "90000000" },
  notes: "",
  version: 1,
  created_at: "2026-07-15T12:00:00Z",
  updated_at: "2026-07-15T12:00:00Z",
};

function jsonResponse(payload: unknown): Response {
  return new Response(JSON.stringify(payload), { status: 200, headers: { "content-type": "application/json" } });
}

function renderPanel(role: "MEMBER" | "ADMIN" = "MEMBER") {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const onSearch = vi.fn();
  const onNotice = vi.fn();
  render(
    <QueryClientProvider client={queryClient}>
      <ProfileRecordsPanel
        profile={profile}
        role={role}
        search={normalizeProfileSearch({ section: "documents" })}
        section="documents"
        onSearch={onSearch}
        onNotice={onNotice}
      />
    </QueryClientProvider>,
  );
  return { onSearch, onNotice };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("ProfileRecordsPanel", () => {
  it("lists Profile documents, keeps filters in navigation state, and saves inline fields", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/api/v1/document-types"))
        return Promise.resolve(jsonResponse({ types: [{ id: "type-1", technical_key: "rg", label: "RG", active: true, uniqueness_policy: "PER_PROFILE", validation_regex: "", date_required: false, version: 1, created_at: "2026-07-15T12:00:00Z", updated_at: "2026-07-15T12:00:00Z" }], page: { total: 1, limit: 1000, offset: 0, sort_field: "label", sort_order: "asc" } }));
      if (url.includes("/api/v1/documents?") && (!init?.method || init.method === "GET"))
        return Promise.resolve(jsonResponse({ documents: [{ id: "doc-1", owner_profile_id: profile.id, document_type_id: "type-1", identifier_value: "00123", document_date: "2026-07-01", notes: "", record_state: "CURRENT", status: "AVAILABLE", type: { id: "type-1", technical_key: "rg", label: "RG", active: true, uniqueness_policy: "PER_PROFILE", validation_regex: "", date_required: false, version: 1, created_at: "2026-07-15T12:00:00Z", updated_at: "2026-07-15T12:00:00Z" }, version: 1, created_at: "2026-07-15T12:00:00Z", updated_at: "2026-07-15T12:00:00Z" }], page: { total: 1, limit: 100, offset: 0, sort_field: "identifier_value", sort_order: "asc" } }));
      if (url.includes("/api/v1/documents/doc-1") && init?.method === "PUT")
        return Promise.resolve(jsonResponse({}));
      return Promise.resolve(jsonResponse({ profiles: [profile], page: { total: 1, limit: 1000, offset: 0, sort_field: "full_name", sort_order: "asc" } }));
    });
    vi.stubGlobal("fetch", fetchMock);
    const { onSearch } = renderPanel();
    expect(await screen.findAllByText("RG")).not.toHaveLength(0);
    expect(screen.getAllByText("00123")).not.toHaveLength(0);
    fireEvent.change(screen.getByLabelText("Identificador"), { target: { value: "001" } });
    expect(onSearch).toHaveBeenCalledWith({ document_identifier: "001", document_page: 1 });
    const inline = screen.getByLabelText("Identificador de RG");
    fireEvent.change(inline, { target: { value: "00099" } });
    fireEvent.blur(inline);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining("/api/v1/documents/doc-1"), expect.objectContaining({ method: "PUT" })));
  });

  it("shows type administration only to administrators", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ types: [], documents: [], page: { total: 0, limit: 100, offset: 0, sort_field: "identifier_value", sort_order: "asc" } })));
    renderPanel("MEMBER");
    await screen.findByText("Nenhum documento cadastrado para esta pessoa.");
    expect(screen.queryByRole("button", { name: "Administrar tipos" })).not.toBeInTheDocument();
    cleanup();
    renderPanel("ADMIN");
    expect(await screen.findByRole("button", { name: "Administrar tipos" })).toBeInTheDocument();
  });
});
''', encoding="utf-8")

changelog_path = Path("CHANGELOG.md")
changelog = changelog_path.read_text(encoding="utf-8")
anchor = "### Added\n\n"
addition = "- Profile-integrated document and bill management with URL-backed sections, filters, sorting, pagination, selection, and forms.\n- Responsive record cards, supported desktop inline editing, type administration, current-use workflows, duplication, permanent deletion, and frontend acceptance coverage.\n"
changelog = replace_once(changelog, anchor, anchor + addition)
changelog_path.write_text(changelog, encoding="utf-8")
