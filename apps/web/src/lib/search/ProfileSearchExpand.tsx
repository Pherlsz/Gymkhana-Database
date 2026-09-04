import { Alert, Button } from "antd";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { DocumentPresenceSection } from "../tables/DocumentPresenceSection";
import { tableLinkProps } from "../tables/tableRoutes";
import { getProfile } from "../api/client";
import { useI18n } from "../../i18n";
import { normalizeProfileSearch, ProfileReadout, ProfileReadoutSkeleton } from "../../ProfilesPage";
import { groupAsResult, profileSearchForResult, shouldFetchPreview } from "./groupResults";
import type { ProfileCard } from "./types";
import { searchErrorMessage } from "./errors";

export function ProfileSearchExpand({
  card,
  preview,
  moduleLabels,
  onCollapse,
}: {
  card: ProfileCard;
  preview: string;
  moduleLabels: Map<string, string>;
  onCollapse: () => void;
}) {
  const { messages } = useI18n();
  const searchMessages = messages.search;
  const inspector = messages.tables.inspector;
  const enabled = shouldFetchPreview(preview, card.profileId);
  const profileQuery = useQuery({
    queryKey: ["profile", card.profileId],
    queryFn: ({ signal }) => getProfile(card.profileId, signal),
    enabled,
  });
  const tableSearch = normalizeProfileSearch({ selected: card.profileId, mode: "view" });
  const expandId = `search-expand-${card.profileId}`;

  return (
    <div aria-busy={profileQuery.isFetching} className="search-card__expand" id={expandId}>
      <div className="search-card__expand-body">
        {profileQuery.isError ? (
          <Alert
            message={searchMessages.previewErrorTitle}
            type="error"
            showIcon
            description={<>{searchErrorMessage(profileQuery.error)}</>}
          />
        ) : null}
        {profileQuery.isFetching && !profileQuery.data ? (
          <>
            <p className="visually-hidden" role="status">
              {searchMessages.loadingPreview}
            </p>
            <ProfileReadoutSkeleton />
          </>
        ) : null}
        {profileQuery.data ? (
          <ProfileReadout
            boolean={messages.tables.boolean}
            columns={messages.tables.columns}
            empty={inspector.empty}
            fields={inspector.fields}
            profile={profileQuery.data}
            sections={inspector.sections}
            showLessLabel={inspector.showLess}
            showMoreLabel={inspector.showMore}
            documentPresence={
              <DocumentPresenceSection
                copy={inspector.presence}
                editable={false}
                marks={messages.tables.badges}
                profile={profileQuery.data}
              />
            }
          />
        ) : null}

        {card.relatedGroups.length > 0 ? (
          <section className="search-card__related" aria-label={searchMessages.showRelated}>
            <h3 className="search-card__related-title">{searchMessages.relatedHitsTitle}</h3>
            <ul className="search-card__related-list">
              {card.relatedGroups.map((group) => {
                const nav = profileSearchForResult(groupAsResult(group));
                return (
                  <li key={group.key} className="search-card__related-row">
                    <span className="search-card__related-kind">
                      {moduleLabels.get(group.module) ?? group.module}
                    </span>
                    <span className="search-card__related-label">{group.entityLabel}</span>
                    <span className="search-card__related-preview">
                      {group.matches[0]?.preview}
                    </span>
                    {nav ? (
                      <Link className="search-card__related-open" {...tableLinkProps(nav)}>
                        {searchMessages.openRecord}
                      </Link>
                    ) : null}
                  </li>
                );
              })}
            </ul>
          </section>
        ) : null}
      </div>

      <div className="search-card__actions">
        <Link className="search-card__action-link" {...tableLinkProps(tableSearch)}>
          {searchMessages.openInTable}
        </Link>
        <Button type="text" className="search-card__collapse" onClick={onCollapse}>
          {searchMessages.collapseFicha}
        </Button>
      </div>
    </div>
  );
}
