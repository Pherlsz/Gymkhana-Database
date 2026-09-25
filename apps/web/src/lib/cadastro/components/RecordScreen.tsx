import { Button } from "antd";
import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import { ICON, ICON_STROKE } from "../../../components/icons";
import "../../../cadastro.css";

export function RecordScreen({
  ariaLabel,
  eyebrow,
  title,
  closeLabel,
  onClose,
  children,
  footer,
  ownerLabel,
  ownerName,
  openOwnerLabel,
  onOpenOwner,
}: {
  ariaLabel: string;
  eyebrow: string;
  title: string;
  closeLabel: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  ownerLabel?: string | undefined;
  ownerName?: string | undefined;
  openOwnerLabel?: string | undefined;
  onOpenOwner?: (() => void) | undefined;
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
        <div className="cadastro-single">{children}</div>
      </div>
      {footer ? <div className="profile-panel__footer">{footer}</div> : null}
    </aside>
  );
}
