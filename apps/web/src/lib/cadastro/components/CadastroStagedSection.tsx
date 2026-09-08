import { Button } from "antd";
import { Plus, Trash2 } from "lucide-react";
import type { ReactNode } from "react";
import { CadastroSection } from "./CadastroSection";

export interface StagedItem {
  id: string;
  title: string;
  subtitle?: string | undefined;
  tagText: string;
  isOcr?: boolean | undefined;
}

export interface CadastroStagedSectionProps {
  title: string;
  hint: string;
  icon: ReactNode;
  modifier?: "documents" | "bills" | undefined;
  count: number;
  items: StagedItem[];
  emptyText: string;
  addLabel: string;
  confirmLabel: string;
  cancelLabel: string;
  removeLabel?: string | undefined;
  adding: boolean;
  onStartAdd: () => void;
  onCancelAdd: () => void;
  onConfirmAdd: () => void;
  onRemoveItem: (id: string) => void;
  defaultOpen?: boolean | undefined;
  open?: boolean | undefined;
  onToggleOpen?: (() => void) | undefined;
  children: ReactNode;
}

/**
 * Standardized staged collection section used across person mode for
 * managing pending documents and utility bills before persistence.
 */
export function CadastroStagedSection({
  title,
  hint,
  icon,
  modifier,
  count,
  items,
  emptyText,
  addLabel,
  confirmLabel,
  cancelLabel,
  removeLabel = "Remover",
  adding,
  onStartAdd,
  onCancelAdd,
  onConfirmAdd,
  onRemoveItem,
  defaultOpen = true,
  open,
  onToggleOpen,
  children,
}: CadastroStagedSectionProps) {
  const badgeModifier = modifier ? ` cadastro-group__badge--${modifier}` : "";

  return (
    <CadastroSection
      badge={<span className={`cadastro-group__badge${badgeModifier}`}>{count}</span>}
      defaultOpen={defaultOpen}
      hint={hint}
      icon={icon}
      modifier={modifier}
      onToggleOpen={onToggleOpen}
      open={open}
      title={title}
    >
      <div className="strip">
        {items.length === 0 ? (
          <div className="emptyrow">{emptyText}</div>
        ) : (
          items.map((item) => (
            <div key={item.id} className="srow">
              <span className="mk">{icon}</span>
              <div className="tx">
                <b>{item.title}</b>
                {item.subtitle ? <span>{item.subtitle}</span> : null}
              </div>
              <span className="grow" />
              <span className={`tag ${item.isOcr ? "gold" : ""}`}>{item.tagText}</span>
              <div className="acts">
                <button type="button" onClick={() => onRemoveItem(item.id)}>
                  <Trash2 size={13} style={{ marginRight: 4 }} />
                  {removeLabel}
                </button>
              </div>
            </div>
          ))
        )}
      </div>

      {adding ? (
        <div className="inline-addform">
          {children}
          <div className="inline-actions">
            <Button onClick={onCancelAdd}>{cancelLabel}</Button>
            <Button type="primary" onClick={onConfirmAdd}>
              {confirmLabel}
            </Button>
          </div>
        </div>
      ) : (
        <button className="addrow" type="button" onClick={onStartAdd}>
          <Plus size={15} strokeWidth={2} />
          <span>{addLabel}</span>
        </button>
      )}
    </CadastroSection>
  );
}
