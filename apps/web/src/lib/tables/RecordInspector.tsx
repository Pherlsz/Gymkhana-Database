import { Button } from "antd";
import { ChevronRight } from "lucide-react";
import { ICON, ICON_STROKE } from "../../components/icons";
import { StateCard } from "../../components/StateCard";
import { EMPTY_CELL, type TableRow } from "./tableRows";

export type RecordInspectorField = {
  key: string;
  label: string;
  value: string;
};

/**
 * The row card for documents and bills. Clicking either table used to open the
 * owner's person card, so the record you clicked was never the thing you saw.
 *
 * It reads like the grid does: principal data first, empty fields omitted
 * rather than drawn as an em dash, and the owner offered as a link instead of
 * being substituted for the record.
 */
export function RecordInspector({
  eyebrow,
  ariaLabel,
  title,
  fields,
  ownerLabel,
  ownerName,
  openOwnerLabel,
  closeLabel,
  onClose,
  onOpenOwner,
}: {
  eyebrow: string;
  ariaLabel: string;
  title: string;
  fields: RecordInspectorField[];
  ownerLabel: string;
  ownerName: string;
  openOwnerLabel: string;
  closeLabel: string;
  onClose: () => void;
  onOpenOwner: (() => void) | undefined;
}) {
  return (
    <aside aria-label={ariaLabel} className="profile-panel record-panel">
      <div className="profile-panel__header">
        <div>
          <span className="profile-panel__eyebrow">{eyebrow}</span>
          <h2>{title}</h2>
        </div>
        <Button onClick={onClose}>{closeLabel}</Button>
      </div>
      <div className="profile-panel__body">
        {fields.length > 0 ? (
          <dl className="profile-view__grid record-panel__grid">
            {fields.map((field) => (
              <div className="profile-view__item" key={field.key}>
                <dt>{field.label}</dt>
                <dd>{field.value}</dd>
              </div>
            ))}
          </dl>
        ) : null}
        {onOpenOwner ? (
          <nav aria-label={ownerLabel} className="profile-panel__links">
            <Button className="profile-panel__record-link" onClick={onOpenOwner}>
              <span className="record-panel__owner">
                <span className="record-panel__owner-label">{ownerLabel}</span>
                <strong>{ownerName}</strong>
              </span>
              <ChevronRight
                aria-hidden
                className="profile-panel__link-arrow"
                size={ICON.md}
                strokeWidth={ICON_STROKE}
              />
              <span className="visually-hidden">{openOwnerLabel}</span>
            </Button>
          </nav>
        ) : null}
      </div>
    </aside>
  );
}

/** Loading and not-found share the card's chrome so the panel never jumps. */
export function RecordInspectorState({
  eyebrow,
  ariaLabel,
  closeLabel,
  onClose,
  kind,
  title,
  description,
}: {
  eyebrow: string;
  ariaLabel: string;
  closeLabel: string;
  onClose: () => void;
  kind: "loading" | "error";
  title: string;
  description?: string | undefined;
}) {
  return (
    <aside
      aria-busy={kind === "loading"}
      aria-label={ariaLabel}
      className="profile-panel record-panel"
    >
      <div className="profile-panel__header">
        <div>
          <span className="profile-panel__eyebrow">{eyebrow}</span>
          <h2>{title}</h2>
        </div>
        <Button onClick={onClose}>{closeLabel}</Button>
      </div>
      <div className="profile-panel__body">
        <StateCard compact description={description} kind={kind} title={title} />
      </div>
    </aside>
  );
}

/**
 * Turns an already-built grid row into card fields, keeping the grid's own
 * labels and formatting so the two never disagree, and dropping the keys that
 * are plumbing or that the card header already shows.
 */
export function recordInspectorFields(
  row: TableRow,
  order: { key: string; label: string }[],
  omit: ReadonlySet<string>,
): RecordInspectorField[] {
  const fields: RecordInspectorField[] = [];
  for (const { key, label } of order) {
    if (omit.has(key)) continue;
    const raw = row.cells[key];
    const value = typeof raw === "string" ? raw.trim() : raw == null ? "" : String(raw);
    // The grid renders a missing value as an em dash; the card omits the field
    // instead, so what is left on screen is what the record actually has.
    if (!value || value === EMPTY_CELL) continue;
    fields.push({ key, label, value });
  }
  return fields;
}
