import { memo } from "react";
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

export const DocumentBadges = memo(function DocumentBadges({
  badges,
  empty = EMPTY_CELL,
  withOwnerLabel,
}: {
  badges: ProfileDocumentBadge[];
  empty?: string;
  withOwnerLabel: string;
}) {
  if (badges.length === 0) return empty;
  return (
    <span className="document-badges">
      {badges.map((badge) => (
        <DocumentBadgeChip
          key={badge.document_type_id}
          badge={badge}
          withOwnerLabel={withOwnerLabel}
        />
      ))}
    </span>
  );
});

export function DocumentBadgeChip({
  badge,
  withOwnerLabel,
}: {
  badge: ProfileDocumentBadge;
  withOwnerLabel: string;
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
        <span className="document-badge__mark document-badge__mark--number">nº</span>
      ) : null}
      {showPhysical ? (
        <span className="document-badge__mark document-badge__mark--physical">F</span>
      ) : null}
      {showDigital ? (
        <span className="document-badge__mark document-badge__mark--digital">D</span>
      ) : null}
      {showOwner ? (
        <span aria-label={withOwnerLabel} className="document-badge__owner" title={withOwnerLabel}>
          (i)
        </span>
      ) : null}
    </span>
  );
}
