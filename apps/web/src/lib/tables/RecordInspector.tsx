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
    if (!value || value === EMPTY_CELL) continue;
    fields.push({ key, label, value });
  }
  return fields;
}
