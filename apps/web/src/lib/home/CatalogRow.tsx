import { Link } from "@tanstack/react-router";
import { Skeleton } from "antd";
import { ChevronRight } from "lucide-react";
import { isCanonicalContextSlug, type HomeCatalogItem } from "./catalogTaxonomy";
import { catalogSubIcon } from "./catalogIcons";
import { normalizeProfileSearch } from "../../ProfilePanel";
import { tableLinkProps } from "../tables/tableRoutes";
import { ICON, ICON_STROKE } from "../../components/icons";

export function CatalogRow({ item }: { item: HomeCatalogItem }) {
  const typedFilter = isCanonicalContextSlug(item.id)
    ? {}
    : item.kind === "document"
      ? { document_type: item.id }
      : { bill_type: item.id };
  const search =
    item.kind === "people"
      ? normalizeProfileSearch({})
      : item.kind === "document"
        ? normalizeProfileSearch({ section: "documents", ...typedFilter })
        : normalizeProfileSearch({ section: "bills", ...typedFilter });
  const Icon = catalogSubIcon(item);

  return (
    <Link className="home-catalog__row" {...tableLinkProps(search)}>
      <span className="home-catalog__row-icon">
        <Icon aria-hidden size={15} strokeWidth={ICON_STROKE} />
      </span>
      <span className="home-catalog__row-name">{item.label}</span>
      {item.loading ? (
        <Skeleton.Input active size="small" style={{ width: "4rem" }} />
      ) : typeof item.count !== "number" ? (
        <span className="home-catalog__row-count home-catalog__row-count--empty home-catalog__row-count--none">
          —
        </span>
      ) : item.count > 0 ? (
        <span className="home-catalog__row-count">{item.count.toLocaleString("pt-BR")}</span>
      ) : null}
      <ChevronRight
        aria-hidden
        className="home-catalog__row-open"
        size={ICON.md}
        strokeWidth={ICON_STROKE}
      />
    </Link>
  );
}
