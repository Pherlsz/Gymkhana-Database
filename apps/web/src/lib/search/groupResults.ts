import { normalizeProfileSearch } from "../../ProfilesPage";
import type { ProfileListSearch, SearchResult } from "../api/client";
import type { MatchRow, ProfileCard, ResultGroup } from "./types";

export function groupSearchResults(results: SearchResult[]): ResultGroup[] {
  const groups = new Map<string, ResultGroup>();
  for (const result of results) {
    const key = `${result.module}:${result.entity_id}`;
    const existing = groups.get(key);
    if (existing) {
      existing.matches.push({
        fieldKey: result.field_key,
        fieldLabel: result.field_label,
        preview: result.preview,
      });
      existing.topScore = Math.max(existing.topScore, result.score);
      continue;
    }
    groups.set(key, {
      key,
      module: result.module,
      entityId: result.entity_id,
      entityLabel: result.entity_label,
      updatedAt: result.updated_at,
      topScore: result.score,
      matches: [
        { fieldKey: result.field_key, fieldLabel: result.field_label, preview: result.preview },
      ],
    });
  }
  return [...groups.values()].toSorted((a, b) => b.topScore - a.topScore);
}

export function buildProfileCards(results: SearchResult[]): ProfileCard[] {
  const labelsByProfileId = new Map<string, { label: string; updatedAt: string }>();
  for (const result of results) {
    if (result.module === "profiles") {
      const existing = labelsByProfileId.get(result.entity_id);
      if (!existing || result.score > 0) {
        labelsByProfileId.set(result.entity_id, {
          label: result.entity_label,
          updatedAt: result.updated_at,
        });
      }
    }
  }

  const cards = new Map<string, ProfileCard>();
  for (const group of groupSearchResults(results)) {
    const anchorId =
      group.module === "profiles"
        ? group.entityId
        : (results.find(
            (result) => result.module !== "profiles" && result.entity_id === group.entityId,
          )?.profile_id ?? "");
    if (!anchorId) continue;
    const anchorMeta =
      labelsByProfileId.get(anchorId) ?? ({ label: anchorId, updatedAt: group.updatedAt } as const);
    let card = cards.get(anchorId);
    if (!card) {
      card = {
        key: anchorId,
        profileId: anchorId,
        profileLabel: anchorMeta.label,
        updatedAt: anchorMeta.updatedAt,
        topScore: group.topScore,
        profileMatches: [],
        relatedGroups: [],
      };
      cards.set(anchorId, card);
    }
    card.updatedAt = [card.updatedAt, anchorMeta.updatedAt].toSorted().at(-1) ?? card.updatedAt;
    card.topScore = Math.max(card.topScore, group.topScore);
    if (group.module === "profiles") {
      card.profileMatches.push(...group.matches);
    } else {
      card.relatedGroups.push(group);
    }
  }
  return [...cards.values()].toSorted((a, b) => b.topScore - a.topScore);
}

export function profileSearchForResult(result: SearchResult): ProfileListSearch | null {
  if (result.target_kind === "profile") {
    return normalizeProfileSearch({ selected: result.target_id, mode: "view" });
  }
  if (!result.profile_id) return null;
  if (result.target_kind === "document") {
    return normalizeProfileSearch({
      selected: result.profile_id,
      mode: "view",
      section: "documents",
      document_selected: result.target_id,
      document_mode: "view",
    });
  }
  if (result.target_kind === "bill") {
    return normalizeProfileSearch({
      selected: result.profile_id,
      mode: "view",
      section: "bills",
      bill_selected: result.target_id,
      bill_mode: "view",
    });
  }
  return normalizeProfileSearch({ selected: result.profile_id, mode: "view" });
}

export function groupAsResult(group: ResultGroup): SearchResult {
  const first = group.matches[0];
  return {
    module: group.module,
    entity_kind:
      group.module === "documents"
        ? "document"
        : group.module === "bills"
          ? "bill"
          : "custom_entity",
    entity_id: group.entityId,
    target_kind:
      group.module === "documents" ? "document" : group.module === "bills" ? "bill" : "profile",
    target_id: group.entityId,
    entity_label: group.entityLabel,
    field_key: first?.fieldKey ?? "",
    field_label: first?.fieldLabel ?? "",
    preview: first?.preview ?? "",
    score: group.topScore,
    updated_at: group.updatedAt,
  };
}

/** Closed-card evidence: up to 6 cells, title echoes omitted, related fills gaps. */
export function evidenceRowsForCard(card: ProfileCard): MatchRow[] {
  const title = normalizeEvidenceText(card.profileLabel);
  const seen = new Set<string>();
  const out: MatchRow[] = [];

  const push = (row: MatchRow) => {
    if (out.length >= 6) return;
    const preview = normalizeEvidenceText(row.preview);
    if (!preview) return;
    if (isTitleEcho(preview, title)) return;
    if (isFullNameKey(row.fieldKey) && titlesOverlap(preview, title)) return;
    const dedupe = `${row.fieldKey}|${preview}`;
    if (seen.has(dedupe)) return;
    seen.add(dedupe);
    out.push(row);
  };

  for (const match of card.profileMatches) {
    push({
      fieldKey: match.fieldKey,
      fieldLabel: match.fieldLabel,
      preview: match.preview,
      module: "profiles",
    });
  }
  for (const group of card.relatedGroups) {
    for (const match of group.matches) {
      push({
        fieldKey: `${group.key}:${match.fieldKey}`,
        fieldLabel: match.fieldLabel,
        preview: match.preview || group.entityLabel,
        module: group.module,
      });
    }
  }
  return out;
}

function normalizeEvidenceText(value: string): string {
  return value.trim().toLocaleLowerCase("pt-BR").replace(/\s+/g, " ");
}

function isFullNameKey(fieldKey: string): boolean {
  return /full_name|fullname|entity_label/i.test(fieldKey);
}

function isTitleEcho(preview: string, title: string): boolean {
  return preview === title;
}

function titlesOverlap(preview: string, title: string): boolean {
  return preview === title || title.startsWith(preview) || preview.startsWith(title);
}

export function shouldFetchPreview(preview: string, profileId: string): boolean {
  return preview.length > 0 && preview === profileId;
}
