import { useQuery } from "@tanstack/react-query";
import {
  listDocumentTypes,
  type DocumentType,
  type Profile,
  type ProfileDocumentPresence,
} from "../api/client";
import { queryKeys } from "../api/queryKeys";
import { useI18n } from "../../i18n";
import { DocumentBadges, documentTypeAcronym, type BadgeMarksCopy } from "./documentBadges";

type PresenceCopy = {
  title: string;
  unspecified: string;
  absence: string;
  indication: string;
  informedNumber: string;
  number: string;
  hasExemplar: string;
  withOwner: string;
};

const UNSPECIFIED = "";

export function DocumentPresenceSection({
  profile,
  hideTitle,
  copy: propCopy,
  marks: propMarks,
}: {
  profile: Profile;
  hideTitle?: boolean | undefined;
  copy?: PresenceCopy;
  marks?: BadgeMarksCopy;
}) {
  const { messages } = useI18n();
  const copy = propCopy ?? messages.tables.inspector.presence;
  const marks = propMarks ?? (messages.tables.badges as BadgeMarksCopy);
  const types = useQuery({
    queryKey: queryKeys.types.documents,
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const isRepeatedIdentity = (technicalKey: string) =>
    technicalKey === "cpf" || technicalKey === "rg";
  const activeTypes = (types.data?.types ?? []).filter(
    (type) =>
      (type.active || presenceFor(profile, type.id)) && !isRepeatedIdentity(type.technical_key),
  );
  const visibleBadges = profile.document_badges ?? [];
  if (
    activeTypes.every((type) => !presenceFor(profile, type.id)) &&
    visibleBadges.length === 0
  ) {
    return null;
  }
  return (
    <section className="document-presence">
      {hideTitle ? null : <h3 className="document-presence__title">{copy.title}</h3>}
      {visibleBadges.length ? (
        <DocumentBadges
          badges={visibleBadges}
          empty=""
          marks={marks}
          withOwnerLabel={copy.withOwner}
        />
      ) : null}
      <ul className="document-presence__list">
        {activeTypes.map((type) => (
          <PresenceRow
            key={type.id}
            copy={copy}
            presence={presenceFor(profile, type.id)}
            type={type}
          />
        ))}
      </ul>
    </section>
  );
}

function PresenceRow({
  type,
  presence,
  copy,
}: {
  type: DocumentType;
  presence: ProfileDocumentPresence | undefined;
  copy: PresenceCopy;
}) {
  if (!presence) return null;
  const acronym = documentTypeAcronym(type.technical_key, type.label);
  const locked = Boolean(presence.has_physical || presence.has_digital);
  const claim = presence.claim ?? UNSPECIFIED;
  const status = locked
    ? copy.hasExemplar
    : claim === "absence"
      ? copy.absence
      : claim === "indication"
        ? copy.indication
        : claim === "informed_number"
          ? copy.informedNumber
          : copy.unspecified;
  return (
    <li className="document-presence__row">
      <span className="document-presence__type" title={type.label}>
        {acronym}
      </span>
      <span>{status}</span>
      {claim === "informed_number" && presence.identifier_value ? (
        <span className="document-presence__number">{presence.identifier_value}</span>
      ) : null}
    </li>
  );
}

function presenceFor(profile: Profile, typeId: string) {
  return (profile.document_presences ?? []).find((value) => value.document_type_id === typeId);
}
