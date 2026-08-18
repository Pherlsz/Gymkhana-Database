import { Alert, Button, Card, Flex, Form, Input, Skeleton } from "antd";
import { ChevronDown, ChevronRight, FileText, Receipt } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import * as v from "valibot";
import { ProfileRecordsPanel } from "./ProfileRecordsPanel";
import { useI18n } from "./i18n";
import {
  createProfile,
  updateProfile,
  type Profile,
  type ProfileListSearch,
  type ProfileValuesRequest,
  type UserRole,
} from "./lib/api/client";
import { DocumentPresenceSection } from "./lib/tables/DocumentPresenceSection";

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

const profileFormSchema = v.object({
  full_name: v.pipe(v.string(), v.trim(), v.minLength(1, "Nome completo é obrigatório.")),
  social_name: v.string(),
  cpf: v.string(),
  email: v.union([v.literal(""), v.pipe(v.string(), v.email("E-mail inválido."))]),
  mobile_phone: v.string(),
  landline_phone: v.string(),
  address: v.object({
    street: v.string(),
    number: v.string(),
    complement: v.string(),
    neighborhood: v.string(),
    city: v.string(),
    state: v.string(),
    postal_code: v.string(),
  }),
  notes: v.string(),
});

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
    limit: Math.min(1000, Math.max(50, positiveInteger(search.limit, 100))),
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
    document_limit: Math.min(1000, Math.max(50, positiveInteger(search.document_limit, 100))),
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
    document_medium:
      search.document_medium === "PHYSICAL" || search.document_medium === "DIGITAL"
        ? search.document_medium
        : "",
    document_type: typeof search.document_type === "string" ? search.document_type : "",
    document_selected:
      typeof search.document_selected === "string" ? search.document_selected : undefined,
    document_mode: ["create", "view", "edit", "types"].includes(String(search.document_mode))
      ? (search.document_mode as ProfileListSearch["document_mode"])
      : undefined,
    bill_page: positiveInteger(search.bill_page, 1),
    bill_limit: Math.min(1000, Math.max(50, positiveInteger(search.bill_limit, 100))),
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
    bill_medium:
      search.bill_medium === "PHYSICAL" || search.bill_medium === "DIGITAL"
        ? search.bill_medium
        : "",
    bill_type: typeof search.bill_type === "string" ? search.bill_type : "",
    bill_selected: typeof search.bill_selected === "string" ? search.bill_selected : undefined,
    bill_mode: ["create", "view", "edit", "types"].includes(String(search.bill_mode))
      ? (search.bill_mode as ProfileListSearch["bill_mode"])
      : undefined,
    records_owner:
      typeof search.records_owner === "string" && search.records_owner
        ? search.records_owner
        : undefined,
    cols: typeof search.cols === "string" ? search.cols : "",
  };
}

export function ProfilePanel(props: {
  mode: "create" | "view" | "edit";
  profile: Profile | undefined;
  canDelete: boolean;
  pending: boolean;
  section: ProfileListSearch["section"];
  search: ProfileListSearch;
  role: UserRole;
  hideSections?: boolean;
  recordLinks?: boolean;
  loading?: boolean;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
  onClose: () => void;
  onEdit: () => void;
  onCancelEdit?: () => void;
  onSaved: (value: Profile, message: string) => Promise<void>;
  onDelete: (value: Profile, confirmation: string) => void;
  onOpenDocuments?: (value: Profile) => void;
  onOpenBills?: (value: Profile) => void;
}) {
  const { messages } = useI18n();
  const copy = messages.tables.inspector;
  const fields = copy.fields;
  const [form] = Form.useForm<ProfileValuesRequest>();
  const [confirmation, setConfirmation] = useState("");
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const editable = props.mode === "create" || props.mode === "edit";
  const profileId = props.profile?.id;
  const profileVersion = props.profile?.version;
  useEffect(() => {
    if (editable) {
      form.setFieldsValue(props.profile ? profileValues(props.profile) : emptyValues);
    }
    setConfirmation("");
    setConfirmingDelete(false);
    setError(null);
  }, [editable, form, profileId, profileVersion, props.mode, props.profile]);
  const submit = async (values: ProfileValuesRequest) => {
    const parsed = v.safeParse(profileFormSchema, values);
    if (!parsed.success) {
      setError(parsed.issues.map((issue) => issue.message).join(" "));
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const saved = props.profile
        ? await updateProfile(props.profile.id, {
            ...profileValues(props.profile),
            ...parsed.output,
            version: props.profile.version,
          })
        : await createProfile(parsed.output);
      await props.onSaved(saved, props.profile ? "Pessoa atualizada." : "Pessoa criada.");
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setSaving(false);
    }
  };
  if (props.mode !== "create" && !props.profile) {
    if (props.loading) {
      return (
        <aside aria-busy="true" aria-label="Detalhes da pessoa" className="profile-panel">
          <div className="profile-panel__header">
            <div>
              <span className="profile-panel__eyebrow">{copy.eyebrow}</span>
              <h2>
                <span className="visually-hidden">{copy.loading}</span>
                <Skeleton.Input active className="profile-panel__title-skeleton" size="small" />
              </h2>
            </div>
            <Button onClick={props.onClose}>{copy.close}</Button>
          </div>
          <div className="profile-panel__body">
            <p className="visually-hidden" role="status">
              {copy.loading}
            </p>
            <ProfileReadoutSkeleton />
          </div>
        </aside>
      );
    }
    return (
      <aside className="profile-panel">
        <div className="profile-panel__header">
          <div>
            <span className="profile-panel__eyebrow">{copy.eyebrow}</span>
            <h2>{copy.notFound}</h2>
          </div>
          <Button onClick={props.onClose}>{copy.close}</Button>
        </div>
        <div className="profile-panel__body">
          <Alert message={copy.notFound} type="error" description={copy.notFoundHint} />
        </div>
      </aside>
    );
  }
  const showRecords =
    Boolean(props.profile) &&
    props.section !== "profile" &&
    props.mode !== "create" &&
    !props.hideSections;
  return (
    <aside aria-label="Detalhes da pessoa" className="profile-panel">
      <div className="profile-panel__header">
        <div>
          <span className="profile-panel__eyebrow">
            {props.mode === "create" ? copy.newPerson : copy.eyebrow}
          </span>
          <h2>{props.profile?.full_name || copy.register}</h2>
        </div>
        <Button onClick={props.onClose}>{copy.close}</Button>
      </div>
      <div className="profile-panel__body">
        {props.profile && props.mode !== "create" && !props.hideSections ? (
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
        {props.profile && props.recordLinks && props.mode !== "create" ? (
          <nav aria-label="Registros da pessoa" className="profile-panel__links">
            {props.section === "documents" || !props.onOpenDocuments ? null : (
              <Button
                className="profile-panel__record-link"
                icon={<FileText aria-hidden size={16} strokeWidth={1.75} />}
                onClick={() => props.onOpenDocuments?.(props.profile!)}
              >
                {copy.documentsLink}
                <ChevronRight
                  aria-hidden
                  className="profile-panel__link-arrow"
                  size={15}
                  strokeWidth={1.75}
                />
              </Button>
            )}
            {props.section === "bills" || !props.onOpenBills ? null : (
              <Button
                className="profile-panel__record-link"
                icon={<Receipt aria-hidden size={16} strokeWidth={1.75} />}
                onClick={() => props.onOpenBills?.(props.profile!)}
              >
                {copy.billsLink}
                <ChevronRight
                  aria-hidden
                  className="profile-panel__link-arrow"
                  size={15}
                  strokeWidth={1.75}
                />
              </Button>
            )}
          </nav>
        ) : null}
        {showRecords && (props.section === "documents" || props.section === "bills") ? (
          <ProfileRecordsPanel
            profile={props.profile!}
            role={props.role}
            search={props.search}
            section={props.section}
            onNotice={props.onNotice}
            onSearch={props.onSearch}
          />
        ) : (
          <>
            {error ? (
              <Alert message={copy.saveError} type="error" description={<>{error}</>} />
            ) : null}
            {props.mode === "view" && props.profile ? (
              <ProfileReadout
                boolean={messages.tables.boolean}
                columns={messages.tables.columns}
                empty={copy.empty}
                fields={fields}
                profile={props.profile}
                sections={copy.sections}
                showLessLabel={copy.showLess}
                showMoreLabel={copy.showMore}
                documentPresence={
                  <DocumentPresenceSection
                    copy={copy.presence}
                    editable={false}
                    marks={messages.tables.badges}
                    profile={props.profile}
                  />
                }
              />
            ) : (
              <Form
                className="profile-form"
                form={form}
                initialValues={props.profile ? profileValues(props.profile) : emptyValues}
                layout="vertical"
                onFinish={(values) => void submit(values)}
              >
                <Form.Item label={fields.fullName} name="full_name">
                  <Input />
                </Form.Item>
                <Form.Item label={fields.socialName} name="social_name">
                  <Input />
                </Form.Item>
                <Form.Item label={fields.cpf} name="cpf">
                  <Input inputMode="numeric" />
                </Form.Item>
                <Form.Item label={fields.email} name="email">
                  <Input type="email" />
                </Form.Item>
                <Form.Item label={fields.mobile} name="mobile_phone">
                  <Input />
                </Form.Item>
                <Form.Item label={fields.landline} name="landline_phone">
                  <Input />
                </Form.Item>
                <Form.Item label={fields.street} name={["address", "street"]}>
                  <Input />
                </Form.Item>
                <div className="profile-form__pair">
                  <Form.Item label={fields.number} name={["address", "number"]}>
                    <Input />
                  </Form.Item>
                  <Form.Item
                    label={fields.state}
                    name={["address", "state"]}
                    normalize={(value) => String(value).toUpperCase()}
                  >
                    <Input maxLength={2} />
                  </Form.Item>
                </div>
                <Form.Item label={fields.complement} name={["address", "complement"]}>
                  <Input />
                </Form.Item>
                <Form.Item label={fields.neighborhood} name={["address", "neighborhood"]}>
                  <Input />
                </Form.Item>
                <Form.Item label={fields.city} name={["address", "city"]}>
                  <Input />
                </Form.Item>
                <Form.Item label={fields.postalCode} name={["address", "postal_code"]}>
                  <Input inputMode="numeric" />
                </Form.Item>
                <Form.Item label={fields.notes} name="notes">
                  <Input.TextArea rows={4} />
                </Form.Item>
              </Form>
            )}
            {props.profile && props.mode === "edit" ? (
              <DocumentPresenceSection
                copy={copy.presence}
                editable
                marks={messages.tables.badges}
                profile={props.profile}
              />
            ) : null}
          </>
        )}
      </div>
      {showRecords ? null : (
        <div className="profile-panel__footer">
          {props.profile && props.canDelete && confirmingDelete ? (
            <Card className="profile-delete" size="small">
              <Flex vertical gap="0.75rem">
                <strong>{copy.deleteTitle}</strong>
                <span>{copy.deleteHint}</span>
                <Input
                  autoComplete="off"
                  placeholder={copy.confirmPlaceholder}
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                />
                <Flex gap="0.6rem" wrap>
                  <Button
                    danger
                    disabled={confirmation !== "Confirmar" || props.pending}
                    onClick={() => props.onDelete(props.profile!, confirmation)}
                  >
                    {copy.deleteConfirm}
                  </Button>
                  <Button
                    disabled={props.pending}
                    onClick={() => {
                      setConfirmingDelete(false);
                      setConfirmation("");
                    }}
                  >
                    {copy.cancel}
                  </Button>
                </Flex>
              </Flex>
            </Card>
          ) : (
            <Flex className="profile-panel__actions">
              {editable ? (
                <Button type="primary" disabled={saving} onClick={() => form.submit()}>
                  {saving ? copy.saving : copy.save}
                </Button>
              ) : (
                <Button type="primary" onClick={props.onEdit}>
                  {copy.edit}
                </Button>
              )}
              {props.mode === "edit" && props.onCancelEdit ? (
                <Button onClick={props.onCancelEdit}>{copy.cancel}</Button>
              ) : null}
              {props.profile && props.canDelete && props.mode === "view" ? (
                <Button danger disabled={props.pending} onClick={() => setConfirmingDelete(true)}>
                  {copy.delete}
                </Button>
              ) : null}
            </Flex>
          )}
        </div>
      )}
    </aside>
  );
}

function ProfileReadoutSkeleton() {
  const bar = (width: string, height: string) => (
    <Skeleton.Input active size="small" style={{ width, height, minWidth: 0 }} />
  );
  const item = (labelWidth: string, valueWidth: string) => (
    <div className="profile-view__item">
      {bar(labelWidth, "0.7rem")}
      {bar(valueWidth, "1rem")}
    </div>
  );
  return (
    <div aria-hidden="true" className="profile-view profile-view--loading">
      <div className="profile-view__grid">
        {item("7.5rem", "85%")}
        {item("6rem", "45%")}
        {item("2.5rem", "55%")}
        {item("4rem", "70%")}
        {item("4.5rem", "60%")}
        {item("8rem", "50%")}
        {item("6.5rem", "90%")}
        {item("4rem", "70%")}
        {item("2rem", "50%")}
        {item("4rem", "40%")}
        {item("3.5rem", "55%")}
        {item("3.5rem", "65%")}
      </div>
    </div>
  );
}

function ProfileReadout({
  profile,
  empty,
  fields,
  columns,
  sections,
  boolean,
  documentPresence,
  showMoreLabel,
  showLessLabel,
}: {
  profile: Profile;
  empty: string;
  fields: ReturnType<typeof useI18n>["messages"]["tables"]["inspector"]["fields"];
  columns: ReturnType<typeof useI18n>["messages"]["tables"]["columns"];
  sections: ReturnType<typeof useI18n>["messages"]["tables"]["inspector"]["sections"];
  boolean: ReturnType<typeof useI18n>["messages"]["tables"]["boolean"];
  documentPresence: ReactNode;
  showMoreLabel: string;
  showLessLabel: string;
}) {
  const [expanded, setExpanded] = useState(false);
  useEffect(() => setExpanded(false), [profile.id]);
  /**
   * Empty fields are omitted rather than drawn as an em dash. The grid already
   * works this way, and "Ver mais dados" used to dump ~40 labels of which most
   * were placeholders, which buried the ones that carried data.
   */
  const item = (
    label: string,
    value: unknown,
    options?: { date?: boolean; key?: string; wide?: boolean },
  ) => {
    const display = displayProfileValue(value, empty, boolean, options?.date);
    if (display === empty || display === "") return null;
    return (
      <div
        className={
          options?.wide ? "profile-view__item profile-view__item--wide" : "profile-view__item"
        }
        key={options?.key}
      >
        <dt>{label}</dt>
        <dd>{display}</dd>
      </div>
    );
  };
  const section = (title: string, content: ReactNode) => (
    <section className="profile-view__section">
      <h3>{title}</h3>
      <dl className="profile-view__grid">{content}</dl>
    </section>
  );
  const customValues = Object.entries(profile.custom_values ?? {}).sort(([left], [right]) =>
    left.localeCompare(right, "pt-BR"),
  );
  const detailsId = `profile-details-${profile.id}`;

  return (
    <div className="profile-view">
      {section(
        sections.identification,
        <>
          {item(fields.fullName, profile.full_name, { wide: true })}
          {item(fields.socialName, profile.social_name)}
          {item(fields.cpf, formatCPF(profile.cpf))}
          {item(columns.birthDate, profile.birth_date, { date: true })}
          {item(columns.gender, profile.gender)}
          {item(columns.bloodType, profile.blood_type)}
          {item(columns.maritalStatus, profile.marital_status)}
        </>,
      )}
      {/* The badges used to be drawn here and again at the top of the presence
          section immediately below, which showed the same row twice. */}
      {documentPresence}
      <Button
        aria-controls={detailsId}
        aria-expanded={expanded}
        className="profile-view__more"
        onClick={() => setExpanded((current) => !current)}
      >
        {expanded ? showLessLabel : showMoreLabel}
        <ChevronDown
          aria-hidden
          className={expanded ? "is-open" : undefined}
          size={16}
          strokeWidth={1.75}
        />
      </Button>
      {expanded ? (
        <div className="profile-view__details" id={detailsId}>
          {section(
            sections.contact,
            <>
              {item(fields.email, profile.email, { wide: true })}
              {item(fields.mobile, profile.mobile_phone)}
              {item(fields.landline, profile.landline_phone)}
              {item(fields.street, profile.address.street, { wide: true })}
              {item(fields.number, profile.address.number)}
              {item(fields.complement, profile.address.complement)}
              {item(fields.neighborhood, profile.address.neighborhood)}
              {item(fields.city, profile.address.city)}
              {item(fields.state, profile.address.state)}
              {item(fields.postalCode, profile.address.postal_code)}
            </>,
          )}
          {section(
            sections.originFamily,
            <>
              {item(columns.nationality, profile.nationality)}
              {item(columns.placeOfOrigin, profile.place_of_origin)}
              {item(columns.birthCity, profile.birth_city)}
              {item(columns.birthCountry, profile.birth_country)}
              {item(columns.weddingDate, profile.wedding_date, { date: true })}
              {item(columns.parentsWeddingDate, profile.parents_wedding_date, { date: true })}
              {item(columns.fatherName, profile.father_name)}
              {item(columns.fatherBirthDate, profile.father_birth_date, { date: true })}
              {item(columns.motherName, profile.mother_name)}
              {item(columns.motherBirthDate, profile.mother_birth_date, { date: true })}
            </>,
          )}
          {section(
            sections.healthCommunity,
            <>
              {item(columns.healthPlan, profile.health_plan)}
              {item(columns.bloodDonor, profile.blood_donor)}
              {item(columns.organDonor, profile.organ_donor)}
              {item(columns.team, profile.team)}
              {item(columns.sector, profile.sector)}
              {item(columns.clubMembership, profile.club_membership)}
              {item(columns.membershipType, profile.membership_type)}
            </>,
          )}
          {section(
            sections.interests,
            <>
              {item(columns.collections, profile.collections)}
              {item(columns.supermarketClub, profile.supermarket_club)}
              {item(columns.pet, profile.pet)}
              {item(columns.travelCountries, profile.travel_countries, { wide: true })}
            </>,
          )}
          {section(
            sections.vehicleFinancial,
            <>
              {item(columns.vehicleModel, profile.vehicle_model)}
              {item(columns.vehicleColor, profile.vehicle_color)}
              {item(columns.vehiclePlate, profile.vehicle_plate)}
              {item(columns.vehicleYear, profile.vehicle_year)}
              {item(columns.cardBrand, profile.card_brand)}
              {item(columns.cardBank, profile.card_bank)}
            </>,
          )}
          {customValues.length > 0
            ? section(
                sections.custom,
                <>{customValues.map(([key, value]) => item(humanizeKey(key), value, { key }))}</>,
              )
            : null}
          {section(
            sections.notes,
            <>
              {item(fields.notes, profile.notes, { wide: true })}
              {item(sections.createdAt, profile.created_at, { wide: true })}
              {item(sections.updatedAt, profile.updated_at, { wide: true })}
            </>,
          )}
        </div>
      ) : null}
    </div>
  );
}

function displayProfileValue(
  value: unknown,
  empty: string,
  boolean: { yes: string; no: string },
  date = false,
) {
  if (typeof value === "boolean") return value ? boolean.yes : boolean.no;
  if (typeof value === "number")
    return Number.isFinite(value) ? value.toLocaleString("pt-BR") : empty;
  const trimmed = String(value ?? "").trim();
  if (!trimmed) return empty;
  if (date && /^\d{4}-\d{2}-\d{2}/.test(trimmed)) {
    const parsed = new Date(`${trimmed.slice(0, 10)}T00:00:00`);
    if (!Number.isNaN(parsed.getTime())) return parsed.toLocaleDateString("pt-BR");
  }
  if (/^\d{4}-\d{2}-\d{2}T/.test(trimmed)) {
    const parsed = new Date(trimmed);
    if (!Number.isNaN(parsed.getTime())) return parsed.toLocaleString("pt-BR");
  }
  return trimmed;
}

function humanizeKey(value: string) {
  const label = value.replaceAll("_", " ").replaceAll("-", " ").trim();
  return label ? `${label[0]?.toLocaleUpperCase("pt-BR") ?? ""}${label.slice(1)}` : value;
}
function profileValues(value: Profile): ProfileValuesRequest {
  const request: ProfileValuesRequest = {
    full_name: value.full_name,
    social_name: value.social_name,
    cpf: value.cpf,
    email: value.email,
    mobile_phone: value.mobile_phone,
    landline_phone: value.landline_phone,
    address: { ...value.address },
    notes: value.notes,
  };
  const extras: Array<[keyof ProfileValuesRequest, unknown]> = [
    ["birth_date", value.birth_date],
    ["gender", value.gender],
    ["blood_type", value.blood_type],
    ["nationality", value.nationality],
    ["birth_city", value.birth_city],
    ["marital_status", value.marital_status],
    ["wedding_date", value.wedding_date],
    ["father_name", value.father_name],
    ["father_birth_date", value.father_birth_date],
    ["mother_name", value.mother_name],
    ["mother_birth_date", value.mother_birth_date],
    ["health_plan", value.health_plan],
    ["blood_donor", value.blood_donor],
    ["organ_donor", value.organ_donor],
    ["team", value.team],
    ["sector", value.sector],
    ["collections", value.collections],
    ["vehicle_model", value.vehicle_model],
    ["vehicle_color", value.vehicle_color],
    ["vehicle_plate", value.vehicle_plate],
    ["vehicle_year", value.vehicle_year],
    ["club_membership", value.club_membership],
    ["membership_type", value.membership_type],
    ["place_of_origin", value.place_of_origin],
    ["birth_country", value.birth_country],
    ["parents_wedding_date", value.parents_wedding_date],
    ["supermarket_club", value.supermarket_club],
    ["pet", value.pet],
    ["travel_countries", value.travel_countries],
    ["card_brand", value.card_brand],
    ["card_bank", value.card_bank],
  ];
  for (const [key, extra] of extras) {
    if (extra !== undefined) {
      (request as Record<string, unknown>)[key] = extra;
    }
  }
  return request;
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
