import { Alert, Button, Inline, Page, Stack, Surface } from "@pherlsz/gymkhana-ui";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { useEffect, useMemo, useState } from "react";
import { ProfileRecordsPanel } from "./ProfileRecordsPanel";
import { profilesRoute, useApplicationSession } from "./App";
import {
  APIRequestError,
  createProfile,
  deleteProfile,
  duplicateProfile,
  listProfiles,
  updateProfile,
  type Profile,
  type ProfileListSearch,
  type ProfileValuesRequest,
  type UserRole,
} from "./lib/api/client";

const emptyValues: ProfileValuesRequest = {
  full_name: "",
  social_name: "",
  cpf: "",
  email: "",
  mobile_phone: "",
  landline_phone: "",
  address: {
    street: "",
    number: "",
    complement: "",
    neighborhood: "",
    city: "",
    state: "",
    postal_code: "",
  },
  notes: "",
};

function positiveInteger(value: unknown, fallback: number) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : fallback;
}

export function normalizeProfileSearch(search: Record<string, unknown>): ProfileListSearch {
  const sortValues = [
    "full_name",
    "cpf",
    "email",
    "address_city",
    "created_at",
    "updated_at",
  ] as const;
  const sort = sortValues.includes(search.sort as (typeof sortValues)[number])
    ? (search.sort as ProfileListSearch["sort"])
    : "full_name";
  return {
    page: positiveInteger(search.page, 1),
    limit: Math.min(1000, Math.max(100, positiveInteger(search.limit, 100))),
    sort,
    order: search.order === "desc" ? "desc" : "asc",
    full_name: typeof search.full_name === "string" ? search.full_name : "",
    cpf: typeof search.cpf === "string" ? search.cpf : "",
    email: typeof search.email === "string" ? search.email : "",
    city: typeof search.city === "string" ? search.city : "",
    state: typeof search.state === "string" ? search.state : "",
    selected: typeof search.selected === "string" ? search.selected : undefined,
    mode:
      search.mode === "create" || search.mode === "edit" || search.mode === "view"
        ? search.mode
        : undefined,
    section:
      search.section === "documents" || search.section === "bills" ? search.section : "profile",
    document_page: positiveInteger(search.document_page, 1),
    document_limit: Math.min(1000, Math.max(100, positiveInteger(search.document_limit, 100))),
    document_sort: [
      "identifier_value",
      "type_label",
      "document_date",
      "created_at",
      "updated_at",
    ].includes(String(search.document_sort))
      ? (search.document_sort as ProfileListSearch["document_sort"])
      : "identifier_value",
    document_order: search.document_order === "desc" ? "desc" : "asc",
    document_identifier:
      typeof search.document_identifier === "string" ? search.document_identifier : "",
    document_status:
      search.document_status === "AVAILABLE" || search.document_status === "IN_USE"
        ? search.document_status
        : "",
    document_state: ["CURRENT", "REPLACED", "EXPIRED", "ARCHIVED"].includes(
      String(search.document_state),
    )
      ? (search.document_state as ProfileListSearch["document_state"])
      : "",
    document_type: typeof search.document_type === "string" ? search.document_type : "",
    document_selected:
      typeof search.document_selected === "string" ? search.document_selected : undefined,
    document_mode: ["create", "view", "edit", "types"].includes(String(search.document_mode))
      ? (search.document_mode as ProfileListSearch["document_mode"])
      : undefined,
    bill_page: positiveInteger(search.bill_page, 1),
    bill_limit: Math.min(1000, Math.max(100, positiveInteger(search.bill_limit, 100))),
    bill_sort: [
      "reference_value",
      "type_label",
      "competence",
      "amount",
      "created_at",
      "updated_at",
    ].includes(String(search.bill_sort))
      ? (search.bill_sort as ProfileListSearch["bill_sort"])
      : "reference_value",
    bill_order: search.bill_order === "desc" ? "desc" : "asc",
    bill_reference: typeof search.bill_reference === "string" ? search.bill_reference : "",
    bill_competence: typeof search.bill_competence === "string" ? search.bill_competence : "",
    bill_status:
      search.bill_status === "AVAILABLE" || search.bill_status === "IN_USE"
        ? search.bill_status
        : "",
    bill_state: ["CURRENT", "REPLACED", "EXPIRED", "ARCHIVED"].includes(String(search.bill_state))
      ? (search.bill_state as ProfileListSearch["bill_state"])
      : "",
    bill_type: typeof search.bill_type === "string" ? search.bill_type : "",
    bill_selected: typeof search.bill_selected === "string" ? search.bill_selected : undefined,
    bill_mode: ["create", "view", "edit", "types"].includes(String(search.bill_mode))
      ? (search.bill_mode as ProfileListSearch["bill_mode"])
      : undefined,
  };
}

export function ProfilesPage() {
  const session = useApplicationSession();
  const search = profilesRoute.useSearch();
  const navigate = profilesRoute.useNavigate();
  const queryClient = useQueryClient();
  const [notice, setNotice] = useState<string | null>(null);
  const query = useQuery({
    queryKey: ["profiles", search],
    queryFn: ({ signal }) => listProfiles(search, signal),
  });
  const selected = query.data?.profiles.find((value) => value.id === search.selected);
  const canDelete = session.user.role === "ADMIN" || session.user.role === "SUPERADMIN";
  const updateSearch = (patch: Partial<ProfileListSearch>) => {
    void navigate({ search: (current) => ({ ...current, ...patch }) });
  };
  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: ["profiles"] });
  };

  const duplicateMutation = useMutation({
    mutationFn: duplicateProfile,
    onSuccess: async (value) => {
      await refresh();
      setNotice("Cópia criada. Revise os dados antes de continuar.");
      updateSearch({ selected: value.id, mode: "edit" });
    },
  });
  const deleteMutation = useMutation({
    mutationFn: ({ value, confirmation }: { value: Profile; confirmation: string }) =>
      deleteProfile(value.id, value.version, confirmation),
    onSuccess: async () => {
      await refresh();
      setNotice("Pessoa excluída permanentemente.");
      updateSearch({ selected: undefined, mode: undefined });
    },
  });
  const columns = useMemo(
    () =>
      createProfileColumns(
        async (value, field, nextValue) => {
          try {
            await updateProfile(value.id, toUpdateRequest(value, field, nextValue));
            setNotice("Alteração salva.");
            await refresh();
          } catch (error) {
            if (error instanceof APIRequestError && error.status === 409) {
              setNotice("Outro usuário alterou esta pessoa. A linha foi recarregada.");
              await refresh();
              return;
            }
            throw error;
          }
        },
        (value) => updateSearch({ selected: value.id, mode: "view" }),
      ),
    [],
  );
  const table = useReactTable({
    data: query.data?.profiles ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  });
  const totalPages = Math.max(1, Math.ceil((query.data?.page.total ?? 0) / search.limit));

  return (
    <Page.Root maxWidth="lg">
      <Page.Header>
        <Page.Eyebrow>M4 · Profiles e registros</Page.Eyebrow>
        <Page.Title>Pessoas</Page.Title>
        <Page.Description>
          Cadastre pessoas e gerencie seus documentos, contas e comprovantes. Todo o estado de
          navegação permanece na URL.
        </Page.Description>
        <Page.Actions>
          <Button onClick={() => updateSearch({ selected: undefined, mode: "create" })}>
            Nova pessoa
          </Button>
        </Page.Actions>
      </Page.Header>
      <Page.Content>
        <Stack gap="5">
          {notice ? (
            <Alert title="Atualização" tone="success">
              {notice}
            </Alert>
          ) : null}
          {query.isError ? (
            <Alert title="Não foi possível carregar pessoas" tone="danger">
              {errorMessage(query.error)}
            </Alert>
          ) : null}
          <ProfileFilters
            search={search}
            onChange={(patch) => updateSearch({ ...patch, page: 1 })}
          />
          <Surface className="profiles-grid" tone="raised">
            {query.isLoading ? <p className="profiles-empty">Carregando pessoas...</p> : null}
            {!query.isLoading && (query.data?.profiles.length ?? 0) === 0 ? (
              <p className="profiles-empty">Nenhuma pessoa encontrada.</p>
            ) : null}
            {(query.data?.profiles.length ?? 0) > 0 ? (
              <>
                <div className="profiles-table-wrap">
                  <table className="profiles-table">
                    <thead>
                      {table.getHeaderGroups().map((group) => (
                        <tr key={group.id}>
                          {group.headers.map((header) => (
                            <th key={header.id}>
                              {header.isPlaceholder
                                ? null
                                : flexRender(header.column.columnDef.header, header.getContext())}
                            </th>
                          ))}
                        </tr>
                      ))}
                    </thead>
                    <tbody>
                      {table.getRowModel().rows.map((row) => (
                        <tr key={row.id}>
                          {row.getVisibleCells().map((cell) => (
                            <td key={cell.id}>
                              {flexRender(cell.column.columnDef.cell, cell.getContext())}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="profiles-cards">
                  {query.data?.profiles.map((value) => (
                    <ProfileCard
                      key={value.id}
                      value={value}
                      onOpen={() => updateSearch({ selected: value.id, mode: "view" })}
                    />
                  ))}
                </div>
              </>
            ) : null}
          </Surface>
          <Inline align="center" className="profiles-pagination">
            <Button
              disabled={search.page <= 1}
              onClick={() => updateSearch({ page: search.page - 1 })}
            >
              Anterior
            </Button>
            <span>
              Página {search.page} de {totalPages} · {query.data?.page.total ?? 0} pessoas
            </span>
            <Button
              disabled={search.page >= totalPages}
              onClick={() => updateSearch({ page: search.page + 1 })}
            >
              Próxima
            </Button>
          </Inline>
        </Stack>
      </Page.Content>
      {search.mode ? (
        <ProfilePanel
          key={`${search.mode}:${selected?.id ?? "new"}:${selected?.version ?? 0}`}
          canDelete={canDelete}
          mode={search.mode}
          profile={selected}
          section={search.section}
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
          onEdit={() => updateSearch({ mode: "edit", section: "profile" })}
          onSaved={async (value, message) => {
            await refresh();
            setNotice(message);
            updateSearch({ selected: value.id, mode: "view" });
          }}
          onDuplicate={(value) => duplicateMutation.mutate(value.id)}
          onDelete={(value, confirmation) => deleteMutation.mutate({ value, confirmation })}
          pending={duplicateMutation.isPending || deleteMutation.isPending}
        />
      ) : null}
    </Page.Root>
  );
}

function ProfileFilters({
  search,
  onChange,
}: {
  search: ProfileListSearch;
  onChange: (patch: Partial<ProfileListSearch>) => void;
}) {
  return (
    <Surface className="profile-filters" tone="raised">
      <label>
        Nome
        <input
          value={search.full_name}
          onChange={(event) => onChange({ full_name: event.target.value })}
        />
      </label>
      <label>
        CPF
        <input
          inputMode="numeric"
          value={search.cpf}
          onChange={(event) => onChange({ cpf: event.target.value })}
        />
      </label>
      <label>
        E-mail
        <input
          type="email"
          value={search.email}
          onChange={(event) => onChange({ email: event.target.value })}
        />
      </label>
      <label>
        Cidade
        <input value={search.city} onChange={(event) => onChange({ city: event.target.value })} />
      </label>
      <label>
        UF
        <input
          maxLength={2}
          value={search.state}
          onChange={(event) => onChange({ state: event.target.value.toUpperCase() })}
        />
      </label>
      <label>
        Ordenar
        <select
          value={`${search.sort}:${search.order}`}
          onChange={(event) => {
            const [sort, order] = event.target.value.split(":") as [
              ProfileListSearch["sort"],
              ProfileListSearch["order"],
            ];
            onChange({ sort, order });
          }}
        >
          <option value="full_name:asc">Nome A–Z</option>
          <option value="full_name:desc">Nome Z–A</option>
          <option value="updated_at:desc">Atualizados recentemente</option>
          <option value="created_at:desc">Criados recentemente</option>
          <option value="cpf:asc">CPF</option>
          <option value="email:asc">E-mail</option>
          <option value="address_city:asc">Cidade</option>
        </select>
      </label>
      <label>
        Por página
        <select
          value={search.limit}
          onChange={(event) => onChange({ limit: Number(event.target.value) })}
        >
          <option value={100}>100</option>
          <option value={250}>250</option>
          <option value={500}>500</option>
          <option value={1000}>1000</option>
        </select>
      </label>
    </Surface>
  );
}

const columnHelper = createColumnHelper<Profile>();
function createProfileColumns(
  onSave: (value: Profile, field: InlineField, next: string) => Promise<void>,
  onOpen: (value: Profile) => void,
) {
  return [
    columnHelper.accessor("full_name", {
      header: "Nome",
      cell: ({ row }) => <InlineEditor value={row.original} field="full_name" onSave={onSave} />,
    }),
    columnHelper.accessor("cpf", {
      header: "CPF",
      cell: ({ row }) => <span>{formatCPF(row.original.cpf)}</span>,
    }),
    columnHelper.accessor("email", {
      header: "E-mail",
      cell: ({ row }) => <InlineEditor value={row.original} field="email" onSave={onSave} />,
    }),
    columnHelper.accessor((value) => value.address.city, {
      id: "city",
      header: "Cidade",
      cell: ({ row }) => <InlineEditor value={row.original} field="city" onSave={onSave} />,
    }),
    columnHelper.accessor("mobile_phone", {
      header: "Celular",
      cell: ({ row }) => <InlineEditor value={row.original} field="mobile_phone" onSave={onSave} />,
    }),
    columnHelper.display({
      id: "actions",
      header: "",
      cell: ({ row }) => <Button onClick={() => onOpen(row.original)}>Abrir</Button>,
    }),
  ];
}
type InlineField = "full_name" | "email" | "city" | "mobile_phone";
function InlineEditor({
  value,
  field,
  onSave,
}: {
  value: Profile;
  field: InlineField;
  onSave: (value: Profile, field: InlineField, next: string) => Promise<void>;
}) {
  const initial = field === "city" ? value.address.city : value[field];
  const [current, setCurrent] = useState(initial);
  useEffect(() => setCurrent(initial), [initial]);
  return (
    <input
      aria-label={`${field} de ${value.full_name}`}
      className="inline-editor"
      value={current}
      onChange={(event) => setCurrent(event.target.value)}
      onBlur={() => {
        if (current !== initial) void onSave(value, field, current);
      }}
    />
  );
}
function ProfileCard({ value, onOpen }: { value: Profile; onOpen: () => void }) {
  return (
    <Surface className="profile-card" tone="raised">
      <Stack gap="2">
        <strong>{value.full_name}</strong>
        <span>{formatCPF(value.cpf) || "CPF não informado"}</span>
        <span>{value.email || "E-mail não informado"}</span>
        <span>{value.address.city || "Cidade não informada"}</span>
        <Button onClick={onOpen}>Abrir</Button>
      </Stack>
    </Surface>
  );
}

function ProfilePanel(props: {
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
function profileValues(value: Profile): ProfileValuesRequest {
  return {
    full_name: value.full_name,
    social_name: value.social_name,
    cpf: value.cpf,
    email: value.email,
    mobile_phone: value.mobile_phone,
    landline_phone: value.landline_phone,
    address: { ...value.address },
    notes: value.notes,
  };
}
function toUpdateRequest(value: Profile, field: InlineField, next: string) {
  const values = profileValues(value);
  if (field === "city") values.address.city = next;
  else values[field] = next;
  return { ...values, version: value.version };
}
function formatCPF(value: string) {
  const digits = value.replace(/\D/g, "");
  return digits.length === 11
    ? digits.replace(/(\d{3})(\d{3})(\d{3})(\d{2})/, "$1.$2.$3-$4")
    : value;
}
function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "Erro inesperado.";
}
