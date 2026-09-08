import { ChevronDown } from "lucide-react";
import { evidenceRowsForCard } from "./groupResults";
import { ProfileSearchExpand } from "./ProfileSearchExpand";
import type { ProfileCard } from "./types";
import { useI18n } from "../../i18n";
import { ICON, ICON_STROKE } from "../../components/icons";

export function ProfileSearchCard({
  card,
  open,
  showUpdatedAt,
  moduleLabels,
  onToggle,
}: {
  card: ProfileCard;
  open: boolean;
  showUpdatedAt: boolean;
  moduleLabels: Map<string, string>;
  onToggle: (profileId: string) => void;
}) {
  const { messages } = useI18n();
  const searchMessages = messages.search;
  const evidence = evidenceRowsForCard(card);
  const expandId = `search-expand-${card.profileId}`;
  const relatedPreview = card.relatedGroups.slice(0, 4);
  const relatedExtra = Math.max(0, card.relatedGroups.length - relatedPreview.length);

  return (
    <article className={open ? "search-card is-open" : "search-card"}>
      <button
        aria-controls={expandId}
        aria-expanded={open}
        className="search-card__header"
        type="button"
        onClick={() => onToggle(card.profileId)}
      >
        <span className="search-card__title-block">
          <span className="search-card__name">{card.profileLabel}</span>
          {showUpdatedAt ? (
            <time className="search-card__updated" dateTime={card.updatedAt}>
              {new Intl.DateTimeFormat("pt-BR", { dateStyle: "short" }).format(
                new Date(card.updatedAt),
              )}
            </time>
          ) : null}
        </span>
        <span className="search-card__header-action">
          <span className="search-card__ficha-label">
            {open ? searchMessages.collapseFicha : searchMessages.openFicha}
          </span>
          <ChevronDown
            aria-hidden
            className="search-card__caret"
            size={ICON.sm}
            strokeWidth={ICON_STROKE}
          />
        </span>
      </button>

      {evidence.length > 0 ? (
        <div className="search-card__evidence">
          {evidence.map((row) => (
            <div key={row.fieldKey} className="search-card__evidence-cell">
              <span className="search-card__origin">
                {row.module && row.module !== "profiles"
                  ? `${moduleLabels.get(row.module) ?? row.module} · ${row.fieldLabel}`
                  : row.fieldLabel}
              </span>
              <strong className="search-card__value">{row.preview}</strong>
            </div>
          ))}
        </div>
      ) : null}

      {!open && relatedPreview.length > 0 ? (
        <div className="search-card__meta">
          {relatedPreview.map((group) => (
            <span key={group.key} className="search-card__chip">
              <span className="search-card__chip-kind">
                {moduleLabels.get(group.module) ?? group.module}
              </span>
              <span className="search-card__chip-value">
                {group.matches[0]?.preview || group.entityLabel}
              </span>
            </span>
          ))}
          {relatedExtra > 0 ? (
            <span className="search-card__related-hint">
              {searchMessages.relatedCount({ count: relatedExtra })}
            </span>
          ) : null}
        </div>
      ) : null}

      {open ? (
        <ProfileSearchExpand
          card={card}
          moduleLabels={moduleLabels}
          preview={card.profileId}
          onCollapse={() => onToggle(card.profileId)}
        />
      ) : null}
    </article>
  );
}
