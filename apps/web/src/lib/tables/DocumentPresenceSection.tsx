import { Input, Select } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  listDocumentTypes,
  upsertDocumentPresence,
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
  saveError: string;
  withOwner: string;
};

const UNSPECIFIED = "";

export function DocumentPresenceSection({
  profile,
  editable,
  copy: propCopy,
  marks: propMarks,
}: {
  profile: Profile;
  editable: boolean;
  copy?: PresenceCopy;
  marks?: BadgeMarksCopy;
}) {
  const { messages } = useI18n();
  const copy = propCopy ?? messages.tables.inspector.presence;
  const marks = propMarks ?? (messages.tables.badges as BadgeMarksCopy);
  const queryClient = useQueryClient();
  const types = useQuery({
    queryKey: queryKeys.types.documents,
    queryFn: ({ signal }) => listDocumentTypes(signal),
  });
  const [error, setError] = useState<string | null>(null);
  const save = useMutation({
    mutationFn: (request: {
      document_type_id: string;
      claim: "absence" | "indication" | "informed_number";
      identifier_value?: string;
    }) =>
      upsertDocumentPresence({
        profile_id: profile.id,
        document_type_id: request.document_type_id,
        claim: request.claim,
        ...(request.identifier_value !== undefined
          ? { identifier_value: request.identifier_value }
          : {}),
      }),
    onSuccess: async () => {
      setError(null);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.tables.profiles() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.profiles.detail(profile.id) }),
      ]);
    },
    onError: (caught) => {
      setError(caught instanceof Error ? caught.message : copy.saveError);
    },
  });
  const isRepeatedIdentity = (technicalKey: string) =>
    !editable && (technicalKey === "cpf" || technicalKey === "rg");
  const activeTypes = (types.data?.types ?? []).filter(
    (type) =>
      (type.active || presenceFor(profile, type.id)) && !isRepeatedIdentity(type.technical_key),
  );
  // Badges are no longer filtered against the readout: this section is now the
  // only place they render, so hiding CPF and RG here would drop them entirely.
  const visibleBadges = profile.document_badges ?? [];
  if (
    !editable &&
    activeTypes.every((type) => !presenceFor(profile, type.id)) &&
    visibleBadges.length === 0
  ) {
    return null;
  }
  return (
    <section
      className={editable ? "document-presence document-presence--editable" : "document-presence"}
    >
      <h3 className="document-presence__title">{copy.title}</h3>
      {visibleBadges.length ? (
        <DocumentBadges
          badges={visibleBadges}
          empty=""
          marks={marks}
          withOwnerLabel={copy.withOwner}
        />
      ) : null}
      {error ? <p className="document-presence__error">{error}</p> : null}
      <ul className="document-presence__list">
        {activeTypes.map((type) => (
          <PresenceRow
            key={type.id}
            copy={copy}
            editable={editable}
            pending={save.isPending}
            presence={presenceFor(profile, type.id)}
            type={type}
            onSave={(claim, identifier) => {
              if (claim === UNSPECIFIED) return;
              save.mutate({
                document_type_id: type.id,
                claim,
                ...(claim === "informed_number" ? { identifier_value: identifier } : {}),
              });
            }}
          />
        ))}
      </ul>
    </section>
  );
}

function PresenceRow({
  type,
  presence,
  editable,
  pending,
  copy,
  onSave,
}: {
  type: DocumentType;
  presence: ProfileDocumentPresence | undefined;
  editable: boolean;
  pending: boolean;
  copy: PresenceCopy;
  onSave: (claim: "" | "absence" | "indication" | "informed_number", identifier: string) => void;
}) {
  const acronym = documentTypeAcronym(type.technical_key, type.label);
  const locked = Boolean(presence?.has_physical || presence?.has_digital);
  const claim = presence?.claim ?? UNSPECIFIED;
  const [identifier, setIdentifier] = useState(presence?.identifier_value ?? "");
  useEffect(() => {
    setIdentifier(presence?.identifier_value ?? "");
  }, [presence?.identifier_value]);
  const status = locked
    ? copy.hasExemplar
    : claim === "absence"
      ? copy.absence
      : claim === "indication"
        ? copy.indication
        : claim === "informed_number"
          ? copy.informedNumber
          : copy.unspecified;
  if (!editable) {
    if (!presence) return null;
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
  return (
    <li className="document-presence__row">
      <span className="document-presence__type" title={type.label}>
        {acronym}
      </span>
      <Select<"" | "absence" | "indication" | "informed_number">
        aria-label={type.label}
        disabled={locked || pending}
        onChange={(next) => {
          if (next === "informed_number" && !identifier.trim()) return;
          onSave(next, identifier.trim());
        }}
        options={[
          ...(claim === UNSPECIFIED ? [{ value: UNSPECIFIED, label: copy.unspecified }] : []),
          { value: "absence", label: copy.absence },
          { value: "indication", label: copy.indication },
          { value: "informed_number", label: copy.informedNumber },
        ]}
        value={claim}
      />
      {locked ? <span className="document-presence__hint">{copy.hasExemplar}</span> : null}
      <Input
        aria-label={`${copy.number} · ${type.label}`}
        disabled={locked || pending}
        onBlur={() => {
          if (identifier.trim()) onSave("informed_number", identifier.trim());
        }}
        onChange={(event) => setIdentifier(event.target.value)}
        placeholder={copy.number}
        value={identifier}
      />
    </li>
  );
}

function presenceFor(profile: Profile, typeId: string) {
  return (profile.document_presences ?? []).find((value) => value.document_type_id === typeId);
}
