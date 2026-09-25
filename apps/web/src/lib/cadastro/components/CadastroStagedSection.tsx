import { Button } from "antd";
import { Plus, Trash2 } from "lucide-react";
import type { KeyboardEvent, ReactNode } from "react";
import { StatusBanner } from "../../../components/StatusBanner";
import { useI18n } from "../../../i18n";
import { CadastroSection, CadastroSectionBadge } from "./CadastroSection";

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
  extraTitle?: ReactNode | undefined;
  count: number;
  items: StagedItem[];
  emptyText: string;
  allowAdd?: boolean | undefined;
  addLabel?: string | undefined;
  confirmLabel?: string | undefined;
  cancelLabel?: string | undefined;
  removeLabel?: string | undefined;
  adding?: boolean | undefined;
  onStartAdd?: (() => void) | undefined;
  onCancelAdd?: (() => void) | undefined;
  onConfirmAdd?: (() => void) | undefined;
  onRemoveItem?: ((id: string) => void) | undefined;
  onOpenItem?: ((id: string) => void) | undefined;
  extra?: ReactNode | undefined;
  error?: string | undefined;
  defaultOpen?: boolean | undefined;
  open?: boolean | undefined;
  onToggleOpen?: (() => void) | undefined;
  children?: ReactNode | undefined;
}

/**
 * Collection strip used on Cadastro (pending add/remove) and on the person
 * inspector (existing documents/bills, optional add when editing).
 */
export function CadastroStagedSection({
  title,
  hint,
  icon,
  modifier,
  extraTitle,
  count,
  items,
  emptyText,
  allowAdd = true,
  addLabel,
  confirmLabel,
  cancelLabel,
  removeLabel,
  adding = false,
  onStartAdd,
  onCancelAdd,
  onConfirmAdd,
  onRemoveItem,
  onOpenItem,
  extra,
  error,
  defaultOpen = true,
  open,
  onToggleOpen,
  children,
}: CadastroStagedSectionProps) {
  const { messages } = useI18n();
  const resolvedRemove = removeLabel ?? messages.common.actions.remove;
  const showAdd = allowAdd && Boolean(addLabel) && Boolean(onStartAdd);

  const openItem = (id: string) => {
    onOpenItem?.(id);
  };

  const handleRowKey = (event: KeyboardEvent<HTMLDivElement>, id: string) => {
    if (!onOpenItem) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openItem(id);
    }
  };

  return (
    <CadastroSection
      badge={<CadastroSectionBadge modifier={modifier}>{count}</CadastroSectionBadge>}
      defaultOpen={defaultOpen}
      extraTitle={extraTitle}
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
          items.map((item) => {
            const interactive = Boolean(onOpenItem);
            return (
              <div
                className={interactive ? "srow srow--interactive" : "srow"}
                key={item.id}
                {...(interactive
                  ? {
                      "aria-label": [item.title, item.subtitle].filter(Boolean).join(" · "),
                      onClick: () => openItem(item.id),
                      onKeyDown: (event: KeyboardEvent<HTMLDivElement>) =>
                        handleRowKey(event, item.id),
                      role: "button" as const,
                      tabIndex: 0,
                    }
                  : {})}
              >
                <span className="mk">{icon}</span>
                <div className="tx">
                  <b>{item.title}</b>
                  {item.subtitle ? <span>{item.subtitle}</span> : null}
                </div>
                <span className="grow" />
                <span className={`tag ${item.isOcr ? "gold" : ""}`}>{item.tagText}</span>
                {onRemoveItem ? (
                  <div className="acts">
                    <button
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation();
                        onRemoveItem(item.id);
                      }}
                    >
                      <Trash2 size={13} style={{ marginRight: 4 }} />
                      {resolvedRemove}
                    </button>
                  </div>
                ) : null}
              </div>
            );
          })
        )}
      </div>

      {error ? <StatusBanner title={error} tone="error" /> : null}
      {extra}

      {showAdd && adding ? (
        <div className="inline-addform">
          {children}
          <div className="inline-actions">
            <Button onClick={onCancelAdd}>{cancelLabel}</Button>
            <Button type="primary" onClick={onConfirmAdd}>
              {confirmLabel}
            </Button>
          </div>
        </div>
      ) : null}

      {showAdd && !adding ? (
        <button className="addrow" type="button" onClick={onStartAdd}>
          <Plus size={15} strokeWidth={2} />
          <span>{addLabel}</span>
        </button>
      ) : null}
    </CadastroSection>
  );
}
