import type { SearchRequest, SearchResult } from "../api/client";

export type SearchModule = NonNullable<SearchRequest["modules"]>[number];

export type GlobalSearchState = {
  q: string;
  modules: string;
  fields: string;
  page: number;
  limit: 25 | 50 | 100;
  sort: "relevance" | "updated_at";
  order: "asc" | "desc";
  /** Profile id whose in-flow ficha is open; empty when none. */
  preview: string;
};

export type MatchRow = {
  fieldKey: string;
  fieldLabel: string;
  preview: string;
  /** Optional module key for related hits (documents, bills, …). */
  module?: string;
};


/** One row per matched field, grouped so the record is the unit of answer. */
export type ResultGroup = {
  key: string;
  module: SearchResult["module"];
  entityId: string;
  entityLabel: string;
  updatedAt: string;
  topScore: number;
  matches: MatchRow[];
};

/** One card per profile; person fields + related document/bill/custom hits. */
export type ProfileCard = {
  key: string;
  profileId: string;
  profileLabel: string;
  updatedAt: string;
  topScore: number;
  profileMatches: MatchRow[];
  relatedGroups: ResultGroup[];
};

export const SEARCH_MODULE_VALUES: SearchModule[] = [
  "profiles",
  "documents",
  "bills",
  "custom_data",
  "attachments",
];
