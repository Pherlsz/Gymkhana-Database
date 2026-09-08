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
  preview: string;
};

export type MatchRow = {
  fieldKey: string;
  fieldLabel: string;
  preview: string;
  module?: string;
};

export type ResultGroup = {
  key: string;
  module: SearchResult["module"];
  entityId: string;
  entityLabel: string;
  updatedAt: string;
  topScore: number;
  matches: MatchRow[];
};

export type ProfileCard = {
  key: string;
  profileId: string;
  profileLabel: string;
  updatedAt: string;
  topScore: number;
  profileMatches: MatchRow[];
  relatedGroups: ResultGroup[];
};

export type SearchPagination = {
  page: number;
  limit: number;
  total: number;
};

export const SEARCH_MODULE_VALUES: SearchModule[] = [
  "profiles",
  "documents",
  "bills",
  "custom_data",
  "attachments",
];

