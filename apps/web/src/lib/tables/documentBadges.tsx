import { File, ScanLine, UserRound, type LucideIcon } from "lucide-react";
import { memo } from "react";
import { ICON, ICON_BADGE_STROKE } from "../../components/icons";
import type { ProfileDocumentBadge } from "../api/client";
import { EMPTY_CELL, type TableRow } from "./tableRows";

const TYPE_ACRONYMS: Record<string, string> = {
  cpf: "CPF",
  rg: "RG",
  cnh: "CNH",
  ctps: "CTPS",
  pis: "PIS",
  oab: "OAB",
  crea: "CREA",
  crm: "CRM",
  cro: "CRO",
  coren: "COREN",
  voter_id: "TE",
  sus_card: "SUS",
  citizen_card: "CC",
  student_id: "CE",
  passport: "PP",
  birth_certificate: "CN",
  marriage_certificate: "CAS",
};

export function documentTypeAcronym(technicalKey: string, label: string) {
  const known = TYPE_ACRONYMS[technicalKey];
  if (known) return known;
  const compact = label.replace(/\s+/g, "");
  if (compact.length > 1 && compact.length <= 6 && /^[\p{Lu}0-9./-]+$/u.test(compact)) {
    return compact;
  }
  const fromKey = technicalKey.replace(/_/g, "").slice(0, 5).toUpperCase();
  return fromKey || compact.slice(0, 6) || technicalKey.toUpperCase();
}

export function badgesFromRow(row: TableRow): ProfileDocumentBadge[] {
  const value = row.cells.document_badges;
  if (!Array.isArray(value)) return [];
  return value.filter(isDocumentBadge);
}

function isDocumentBadge(value: unknown): value is ProfileDocumentBadge {
  return Boolean(
    value &&
    typeof value === "object" &&
    "badge" in value &&
    "technical_key" in value &&
    typeof (value as ProfileDocumentBadge).badge === "string" &&
    typeof (value as ProfileDocumentBadge).technical_key === "string",
  );
}

export type BadgeMark = { glyph: string; label: string };

export type BadgeMarksCopy = {
  number: BadgeMark;
  physical: { label: string };
  digital: { label: string };
};

export const DocumentBadges = memo(function DocumentBadges({
  badges,
  empty = EMPTY_CELL,
  withOwnerLabel,
  marks,
}: {
  badges: ProfileDocumentBadge[];
  empty?: string;
  withOwnerLabel: string;
  marks: BadgeMarksCopy;
}) {
  if (badges.length === 0) return empty;
  return (
    <span className="document-badges">
      {badges.map((badge) => (
        <DocumentBadgeChip
          key={badge.document_type_id}
          badge={badge}
          marks={marks}
          withOwnerLabel={withOwnerLabel}
        />
      ))}
    </span>
  );
});

function PresenceIcon({
  className,
  icon: Icon,
  label,
}: {
  className: string;
  icon: LucideIcon;
  label: string;
}) {
  return (
    <span aria-label={label} className={className} title={label}>
      <Icon aria-hidden size={ICON.badge} strokeWidth={ICON_BADGE_STROKE} />
    </span>
  );
}

export function DocumentBadgeChip({
  badge,
  withOwnerLabel,
  marks,
}: {
  badge: ProfileDocumentBadge;
  withOwnerLabel: string;
  marks: BadgeMarksCopy;
}) {
  const acronym = documentTypeAcronym(badge.technical_key, badge.label);
  const kind = badge.badge;
  const dashed = kind === "indication";
  const showNumber = kind === "informed_number";
  const showPhysical =
    kind === "physical" ||
    kind === "physical_with_owner" ||
    kind === "physical_digital" ||
    kind === "physical_with_owner_digital";
  const showDigital =
    kind === "digital" || kind === "physical_digital" || kind === "physical_with_owner_digital";
  const showOwner = kind === "physical_with_owner" || kind === "physical_with_owner_digital";
  return (
    <span className={dashed ? "document-badge document-badge--indication" : "document-badge"}>
      <span className="document-badge__acronym">{acronym}</span>
      {showNumber ? (
        <span
          aria-label={marks.number.label}
          className="document-badge__mark document-badge__mark--number"
          title={marks.number.label}
        >
          {marks.number.glyph}
        </span>
      ) : null}
      {showPhysical ? (
        <PresenceIcon
          className="document-badge__mark document-badge__mark--physical"
          icon={File}
          label={marks.physical.label}
        />
      ) : null}
      {showDigital ? (
        <PresenceIcon
          className="document-badge__mark document-badge__mark--digital"
          icon={ScanLine}
          label={marks.digital.label}
        />
      ) : null}
      {showOwner ? (
        <PresenceIcon className="document-badge__owner" icon={UserRound} label={withOwnerLabel} />
      ) : null}
    </span>
  );
}
