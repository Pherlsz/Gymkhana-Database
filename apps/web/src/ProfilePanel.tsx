import { Button, Flex, Segmented, Skeleton } from "antd";
import { ChevronDown } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { ConfirmDelete } from "./components/ConfirmDelete";
import { StateCard } from "./components/StateCard";
import { StatusBanner } from "./components/StatusBanner";
import { ProfileRecordsPanel } from "./ProfileRecordsPanel";
import { useI18n } from "./i18n";
import { PersonOwnedCollections } from "./lib/cadastro/components/PersonOwnedCollections";
import { PersonRecordFields } from "./lib/cadastro/components/PersonRecordFields";
import {
  buildProfilePayload,
  demographicsHasMoreDetails,
  mapProfileToState,
} from "./lib/cadastro/cadastroPayloads";
import { INITIAL_COMPLEMENTARY, INITIAL_DEMOGRAPHICS, INITIAL_FAMILY } from "./lib/cadastro/types";
import {
  createProfile,
  updateProfile,
  type Profile,
  type ProfileListSearch,
  type ProfileValuesRequest,
  type UserRole,
} from "./lib/api/client";
import { errorMessage, formatCPF } from "./lib/formatters";
import {
  emptyValues,
  normalizeProfileSearch,
  validateProfileForm,
} from "./lib/profile/profileSearch";
import "./cadastro.css";

export { emptyValues, normalizeProfileSearch, validateProfileForm };

export function ProfilePanel(props: {
  mode: "create" | "view" | "edit";
  profile: Profile | undefined;
  canDelete: boolean;
  pending: boolean;
  section: ProfileListSearch["section"];
  search: ProfileListSearch;
  role: UserRole;
  hideSections?: boolean;
  embedded?: boolean | undefined;
  loading?: boolean;
  onSearch: (patch: Partial<ProfileListSearch>) => void;
  onNotice: (message: string) => void;
  onClose: () => void;
  onEdit: () => void;
  onCancelEdit?: () => void;
  onSaved: (value: Profile, message: string) => Promise<void>;
  onDelete: (value: Profile, confirmation: string) => void;
  onOpenOwnedDocument?: (id: string) => void;
  onOpenOwnedBill?: (id: string) => void;
}) {
  const { messages } = useI18n();
  const copy = messages.tables.inspector;
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const initialFields = props.profile
    ? mapProfileToState(props.profile)
    : {
        demographics: INITIAL_DEMOGRAPHICS,
        family: INITIAL_FAMILY,
        complementary: INITIAL_COMPLEMENTARY,
      };
  const [demographics, setDemographics] = useState(initialFields.demographics);
  const [family, setFamily] = useState(initialFields.family);
  const [complementary, setComplementary] = useState(initialFields.complementary);
  const editable = props.mode === "create" || props.mode === "edit";
  const profileId = props.profile?.id;
  const profileVersion = props.profile?.version;

  useEffect(() => {
    const mapped = props.profile
      ? mapProfileToState(props.profile)
      : {
          demographics: INITIAL_DEMOGRAPHICS,
          family: INITIAL_FAMILY,
          complementary: INITIAL_COMPLEMENTARY,
        };
    setDemographics(mapped.demographics);
    setFamily(mapped.family);
    setComplementary(mapped.complementary);
    setConfirmingDelete(false);
    setError(null);
  }, [profileId, profileVersion, props.mode, props.profile]);

  const submit = async () => {
    const payload: ProfileValuesRequest = {
      ...buildProfilePayload(demographics, family, complementary, demographics.fullName, ""),
      notes: props.profile?.notes ?? "",
    };
    const parsed = validateProfileForm(payload, messages.common.validation);
    if (!parsed.success) {
      setError(parsed.issues.map((issue) => issue.message).join(" "));
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const saved = props.profile
        ? await updateProfile(props.profile.id, {
            ...parsed.output,
            notes: props.profile.notes,
            version: props.profile.version,
          })
        : await createProfile(parsed.output);
      await props.onSaved(saved, props.profile ? copy.personUpdated : copy.personCreated);
    } catch (caught) {
      setError(errorMessage(caught));
    } finally {
      setSaving(false);
    }
  };

  if (props.mode !== "create" && !props.profile) {
    if (props.loading) {
      return (
        <aside aria-busy="true" aria-label={copy.personDetails} className="profile-panel">
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
          <StateCard compact description={copy.notFoundHint} kind="error" title={copy.notFound} />
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
    <aside
      aria-label={copy.personDetails}
      className={props.embedded ? "profile-panel profile-panel--embedded" : "profile-panel"}
    >
      {props.embedded ? null : (
        <div className="profile-panel__header">
          <div>
            <span className="profile-panel__eyebrow">
              {props.mode === "create" ? copy.newPerson : copy.eyebrow}
            </span>
            <h2>{props.profile?.full_name || copy.register}</h2>
          </div>
          <Button onClick={props.onClose}>{copy.close}</Button>
        </div>
      )}
      <div className="profile-panel__body">
        {props.profile && props.mode !== "create" && !props.hideSections ? (
          <nav aria-label={copy.personSections} className="profile-sections">
            <Segmented
              block
              className="segmented-tabs"
              onChange={(value) =>
                props.onSearch({ section: value as "profile" | "documents" | "bills" })
              }
              options={[
                { label: copy.sectionProfile, value: "profile" },
                { label: copy.sectionDocuments, value: "documents" },
                { label: copy.sectionBills, value: "bills" },
              ]}
              value={props.section || "profile"}
            />
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
              <StatusBanner description={error} title={copy.saveError} tone="error" />
            ) : null}
            <div className={editable ? "cadastro-single" : "cadastro-single cadastro-record--view"}>
              <PersonRecordFields
                key={profileId ?? "new"}
                complementary={complementary}
                defaultShowMoreDetails={demographicsHasMoreDetails(demographics)}
                demographics={demographics}
                disabled={!editable}
                family={family}
                onChangeComplementary={(patch) =>
                  setComplementary((current) => ({ ...current, ...patch }))
                }
                onChangeDemographics={(patch) =>
                  setDemographics((current) => ({ ...current, ...patch }))
                }
                onChangeFamily={(patch) => setFamily((current) => ({ ...current, ...patch }))}
              />
              {props.profile ? (
                <PersonOwnedCollections
                  editable={editable}
                  profile={props.profile}
                  {...(props.onOpenOwnedDocument
                    ? { onOpenDocument: props.onOpenOwnedDocument }
                    : {})}
                  {...(props.onOpenOwnedBill ? { onOpenBill: props.onOpenOwnedBill } : {})}
                />
              ) : null}
            </div>
          </>
        )}
      </div>
      {showRecords ? null : (
        <div className="profile-panel__footer">
          {props.profile && props.canDelete && confirmingDelete ? (
            <ConfirmDelete
              cancelLabel={copy.cancel}
              confirmLabel={copy.deleteConfirm}
              confirmationLabel={copy.deleteHint}
              confirmationWord="Confirmar"
              description="Esta ação exclui a pessoa permanentemente e não pode ser desfeita."
              pending={props.pending}
              title={copy.deleteTitle}
              onCancel={() => {
                setConfirmingDelete(false);
              }}
              onConfirm={(word) => props.onDelete(props.profile!, word)}
            />
          ) : (
            <Flex className="profile-panel__actions">
              {editable ? (
                <Button disabled={saving} onClick={() => void submit()} type="primary">
                  {saving ? copy.saving : copy.save}
                </Button>
              ) : (
                <Button onClick={props.onEdit} type="primary">
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

function renderSkeletonBar(width: string, height: string) {
  return <Skeleton.Input active size="small" style={{ width, height, minWidth: 0 }} />;
}

function renderProfileSection(title: string, content: ReactNode) {
  return (
    <section className="profile-view__section">
      <h3>{title}</h3>
      <dl className="profile-view__grid">{content}</dl>
    </section>
  );
}

export function ProfileReadoutSkeleton() {
  const item = (labelWidth: string, valueWidth: string) => (
    <div className="profile-view__item">
      {renderSkeletonBar(labelWidth, "0.7rem")}
      {renderSkeletonBar(valueWidth, "1rem")}
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

export function ProfileReadout({
  profile,
  empty: propEmpty,
  fields: propFields,
  columns: propColumns,
  sections: propSections,
  boolean: propBoolean,
  documentPresence,
  showMoreLabel: propShowMoreLabel,
  showLessLabel: propShowLessLabel,
}: {
  profile: Profile;
  empty?: string;
  fields?: ReturnType<typeof useI18n>["messages"]["tables"]["inspector"]["fields"];
  columns?: ReturnType<typeof useI18n>["messages"]["tables"]["columns"];
  sections?: ReturnType<typeof useI18n>["messages"]["tables"]["inspector"]["sections"];
  boolean?: ReturnType<typeof useI18n>["messages"]["tables"]["boolean"];
  documentPresence?: ReactNode;
  showMoreLabel?: string;
  showLessLabel?: string;
}) {
  const { messages } = useI18n();
  const empty = propEmpty ?? messages.tables.inspector.empty;
  const fields = propFields ?? messages.tables.inspector.fields;
  const columns = propColumns ?? messages.tables.columns;
  const sections = propSections ?? messages.tables.inspector.sections;
  const boolean = propBoolean ?? messages.tables.boolean;
  const showMoreLabel = propShowMoreLabel ?? messages.tables.inspector.showMore;
  const showLessLabel = propShowLessLabel ?? messages.tables.inspector.showLess;
  const [expanded, setExpanded] = useState(false);
  useEffect(() => setExpanded(false), [profile.id]);

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
  const customValues = Object.entries(profile.custom_values ?? {}).toSorted(([left], [right]) =>
    left.localeCompare(right, "pt-BR"),
  );
  const detailsId = `profile-details-${profile.id}`;

  return (
    <div className="profile-view">
      {renderProfileSection(
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
          {renderProfileSection(
            sections.contact,
            <>
              {item(fields.email, profile.email)}
              {item(fields.mobile, profile.mobile_phone)}
              {item(fields.landline, profile.landline_phone)}
              {item(fields.street, profile.address.street)}
              {item(fields.number, profile.address.number)}
              {item(fields.complement, profile.address.complement)}
              {item(fields.neighborhood, profile.address.neighborhood)}
              {item(fields.city, profile.address.city)}
              {item(fields.state, profile.address.state)}
              {item(fields.postalCode, profile.address.postal_code)}
            </>,
          )}
          {renderProfileSection(
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
          {renderProfileSection(
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
          {renderProfileSection(
            sections.interests,
            <>
              {item(columns.collections, profile.collections)}
              {item(columns.supermarketClub, profile.supermarket_club)}
              {item(columns.pet, profile.pet)}
              {item(columns.travelCountries, profile.travel_countries, { wide: true })}
            </>,
          )}
          {renderProfileSection(
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
            ? renderProfileSection(
                sections.custom,
                <>{customValues.map(([key, value]) => item(humanizeKey(key), value, { key }))}</>,
              )
            : null}
          {String(profile.notes ?? "").trim()
            ? renderProfileSection(
                sections.notes,
                <>{item(fields.notes, profile.notes, { wide: true })}</>,
              )
            : null}
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
