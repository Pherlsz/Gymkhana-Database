import {
  groupHomeCatalog,
  isCanonicalContextSlug,
  type HomeCatalogGroupKey,
  type HomeCatalogKind,
} from "../home/catalogTaxonomy";
import type { CatalogV1 } from "../../i18n/v1/pt-BR";

export type TypeFilterGroup = {
  key: HomeCatalogGroupKey;
  label: string;
  options: { value: string; label: string }[];
};

export function groupedTypeFilterOptions(
  types: { id: string; label: string; technicalKey: string }[],
  kind: Exclude<HomeCatalogKind, "people">,
  copy: CatalogV1["home"]["tables"],
): TypeFilterGroup[] {
  const items = types.map((type) => ({
    ...type,
    kind,
    count: 0,
    loading: false,
  }));
  const groups = groupHomeCatalog({
    people: {
      id: "people",
      kind: "people",
      label: copy.personalRecord,
      technicalKey: "dados-pessoais",
      count: 0,
      loading: false,
    },
    documents: kind === "document" ? items : [],
    bills: kind === "bill" ? items : [],
    labels: copy.contexts,
  });

  return groups.flatMap((group) => {
    if (group.key === "personal") return [];
    const options = group.items
      .filter((item) => !isCanonicalContextSlug(item.id))
      .map((item) => ({ value: item.id, label: item.label }));
    if (options.length === 0) return [];
    return [{ key: group.key, label: copy.groups[group.key], options }];
  });
}
