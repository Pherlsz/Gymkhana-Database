import { Link, useNavigate } from "@tanstack/react-router";
import { Button, Skeleton } from "antd";
import { useState } from "react";
import { useI18n } from "../../i18n";
import { normalizeProfileSearch } from "../../ProfilePanel";
import { tableLinkProps } from "../tables/tableRoutes";
import type { HomeInUseTypeChip, HomeOverviewResult } from "./useHomeOverview";

const CHIP_LIMIT = 8;

interface HomeAttentionCardProps {
  overview: HomeOverviewResult;
  documentsInUseTotal: number;
  billsInUseTotal: number;
}

export function HomeAttentionCard({
  overview,
  documentsInUseTotal,
  billsInUseTotal,
}: HomeAttentionCardProps) {
  const navigate = useNavigate();
  const { messages, t } = useI18n();
  const copy = messages.home;
  const [showAllChips, setShowAllChips] = useState(false);
  const inUseTotal = documentsInUseTotal + billsInUseTotal;

  const isWarning = inUseTotal > 0;

  return (
    <section
      className={`home-attention ${isWarning ? "home-attention--warning" : "home-attention--empty"}`}
    >
      <div className="home-attention__copy">
        <div className="home-attention__title-row">
          <h2 className="home-attention__title">{copy.attention.title}</h2>
        </div>
        <p className="home-attention__subtitle">{copy.attention.subtitle}</p>
      </div>

      {isWarning ? (
        <div className="home-attention__open">
          {documentsInUseTotal > 0 ? (
            <Button
              onClick={() =>
                void navigate(
                  tableLinkProps(
                    normalizeProfileSearch({
                      section: "documents",
                      document_status: "IN_USE",
                    }),
                  ),
                )
              }
              size="small"
              type="text"
            >
              {copy.attention.openDocuments}
            </Button>
          ) : null}
          {billsInUseTotal > 0 ? (
            <Button
              onClick={() =>
                void navigate(
                  tableLinkProps(
                    normalizeProfileSearch({
                      section: "bills",
                      bill_status: "IN_USE",
                    }),
                  ),
                )
              }
              size="small"
              type="text"
            >
              {copy.attention.openBills}
            </Button>
          ) : null}
        </div>
      ) : null}

      {overview.loading.inUse ? (
        <Skeleton active paragraph={{ rows: 1 }} title={false} />
      ) : inUseTotal === 0 ? (
        <span className="home-attention__none">{copy.attention.none}</span>
      ) : (
        <>
          <p className="home-attention__value">{inUseTotal.toLocaleString("pt-BR")}</p>
          {overview.inUseTypeChips.length > 0 ? (
            <div className="home-attention__chips">
              {(showAllChips
                ? overview.inUseTypeChips
                : overview.inUseTypeChips.slice(0, CHIP_LIMIT)
              ).map((chip: HomeInUseTypeChip) => (
                <InUseTypeChip chip={chip} key={chip.key} />
              ))}
              {overview.inUseTypeChips.length > CHIP_LIMIT ? (
                <Button
                  aria-expanded={showAllChips}
                  className="home-attention__more"
                  onClick={() => setShowAllChips((current) => !current)}
                  size="small"
                  type="text"
                >
                  {showAllChips
                    ? copy.attention.moreChipsCollapse
                    : t(copy.attention.moreChips, {
                        n: overview.inUseTypeChips.length - CHIP_LIMIT,
                      })}
                </Button>
              ) : null}
            </div>
          ) : null}
          {overview.inUseFetched < inUseTotal ? (
            <p className="home-attention__note">
              {t(copy.attention.truncatedNote, {
                fetched: overview.inUseFetched.toLocaleString("pt-BR"),
                total: inUseTotal.toLocaleString("pt-BR"),
              })}
            </p>
          ) : null}
        </>
      )}
    </section>
  );
}

function InUseTypeChip({ chip }: { chip: HomeInUseTypeChip }) {
  const search =
    chip.kind === "document"
      ? normalizeProfileSearch({
          section: "documents",
          document_type: chip.typeId,
          document_status: "IN_USE",
        })
      : normalizeProfileSearch({
          section: "bills",
          bill_type: chip.typeId,
          bill_status: "IN_USE",
        });
  return (
    <Link className="home-attention__chip" {...tableLinkProps(search)}>
      <span className="home-attention__chip-label">{chip.typeLabel}</span>
      <span className="home-attention__chip-count">{chip.count.toLocaleString("pt-BR")}</span>
    </Link>
  );
}
