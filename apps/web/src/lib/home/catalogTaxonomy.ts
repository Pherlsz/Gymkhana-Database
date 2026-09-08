export type HomeCatalogType = {
  id: string;
  label: string;
  technicalKey: string;
  count: number | null;
  loading: boolean;
};

export type HomeCatalogGroupKey =
  | "personal"
  | "bills"
  | "identity"
  | "work"
  | "socialHealth"
  | "education"
  | "certificates"
  | "otherDocuments";

export type HomeCatalogKind = "people" | "document" | "bill";

export type HomeCatalogItem = HomeCatalogType & {
  kind: HomeCatalogKind;
  technicalKey: string;
};

export type HomeCatalogGroup = {
  key: HomeCatalogGroupKey;
  items: HomeCatalogItem[];
};

export type HomeCatalogContextSlug =
  | "dados-pessoais"
  | "conta-luz"
  | "conta-agua"
  | "conta-internet"
  | "doc-rg"
  | "doc-cnh"
  | "doc-passaporte"
  | "doc-ctps"
  | "doc-pis"
  | "doc-crea"
  | "doc-oab"
  | "doc-crm"
  | "doc-cro"
  | "doc-coren"
  | "doc-titulo-eleitoral"
  | "doc-cartao-sus"
  | "doc-cartao-cidadao"
  | "doc-carteira-estudante"
  | "doc-certidao-nascimento"
  | "doc-certidao-casamento";

type CanonicalContext = {
  slug: HomeCatalogContextSlug;
  group: Exclude<HomeCatalogGroupKey, "otherDocuments">;
  kind: HomeCatalogKind;
  match: string[];
  labelKey: string;
};

const GROUP_ORDER: HomeCatalogGroupKey[] = [
  "personal",
  "bills",
  "identity",
  "work",
  "socialHealth",
  "education",
  "certificates",
  "otherDocuments",
];

const CANONICAL_CONTEXTS: CanonicalContext[] = [
  {
    slug: "dados-pessoais",
    group: "personal",
    kind: "people",
    match: ["dados-pessoais"],
    labelKey: "personalData",
  },
  {
    slug: "conta-luz",
    group: "bills",
    kind: "bill",
    match: ["conta-luz", "energia", "luz"],
    labelKey: "electricityBill",
  },
  {
    slug: "conta-agua",
    group: "bills",
    kind: "bill",
    match: ["conta-agua", "agua"],
    labelKey: "waterBill",
  },
  {
    slug: "conta-internet",
    group: "bills",
    kind: "bill",
    match: ["conta-internet", "internet", "wifi"],
    labelKey: "internetBill",
  },
  { slug: "doc-rg", group: "identity", kind: "document", match: ["doc-rg", "rg"], labelKey: "rg" },
  {
    slug: "doc-cnh",
    group: "identity",
    kind: "document",
    match: ["doc-cnh", "cnh"],
    labelKey: "cnh",
  },
  {
    slug: "doc-passaporte",
    group: "identity",
    kind: "document",
    match: ["doc-passaporte", "passaporte"],
    labelKey: "passport",
  },
  {
    slug: "doc-ctps",
    group: "work",
    kind: "document",
    match: ["doc-ctps", "ctps"],
    labelKey: "ctps",
  },
  {
    slug: "doc-pis",
    group: "work",
    kind: "document",
    match: ["doc-pis", "pis", "pasep"],
    labelKey: "pis",
  },
  {
    slug: "doc-crea",
    group: "work",
    kind: "document",
    match: ["doc-crea", "crea"],
    labelKey: "crea",
  },
  { slug: "doc-oab", group: "work", kind: "document", match: ["doc-oab", "oab"], labelKey: "oab" },
  { slug: "doc-crm", group: "work", kind: "document", match: ["doc-crm", "crm"], labelKey: "crm" },
  { slug: "doc-cro", group: "work", kind: "document", match: ["doc-cro", "cro"], labelKey: "cro" },
  {
    slug: "doc-coren",
    group: "work",
    kind: "document",
    match: ["doc-coren", "coren"],
    labelKey: "coren",
  },
  {
    slug: "doc-titulo-eleitoral",
    group: "socialHealth",
    kind: "document",
    match: ["doc-titulo-eleitoral", "titulo-eleitoral", "titulo-de-eleitor", "titulo"],
    labelKey: "voterId",
  },
  {
    slug: "doc-cartao-sus",
    group: "socialHealth",
    kind: "document",
    match: ["doc-cartao-sus", "cartao-sus", "sus"],
    labelKey: "susCard",
  },
  {
    slug: "doc-cartao-cidadao",
    group: "socialHealth",
    kind: "document",
    match: ["doc-cartao-cidadao", "cartao-cidadao", "cidadao"],
    labelKey: "citizenCard",
  },
  {
    slug: "doc-carteira-estudante",
    group: "education",
    kind: "document",
    match: ["doc-carteira-estudante", "carteira-estudantil", "carteira-estudante", "estudante"],
    labelKey: "studentId",
  },
  {
    slug: "doc-certidao-nascimento",
    group: "certificates",
    kind: "document",
    match: ["doc-certidao-nascimento", "certidao-nascimento", "nascimento"],
    labelKey: "birthCertificate",
  },
  {
    slug: "doc-certidao-casamento",
    group: "certificates",
    kind: "document",
    match: ["doc-certidao-casamento", "certidao-casamento", "casamento"],
    labelKey: "weddingCertificate",
  },
];

const DISCONTINUED_DOCUMENT_KEYS = new Set([
  "identidade",
  "identity",
  "cpf",
  "crea_oab",
  "crea-oab",
]);

export function isCanonicalContextSlug(id: string): boolean {
  return CANONICAL_CONTEXTS.some((context) => context.slug === id);
}

export function normalizeCatalogToken(value: string): string {
  return value
    .normalize("NFD")
    .replace(/\p{M}/gu, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function typeTokens(type: { technicalKey: string; label: string }): string[] {
  return [normalizeCatalogToken(type.technicalKey), normalizeCatalogToken(type.label)].filter(
    Boolean,
  );
}

function matchScore(
  type: { technicalKey: string; label: string },
  context: CanonicalContext,
): number {
  const values = typeTokens(type);
  let best = 0;
  for (const token of context.match) {
    for (const value of values) {
      if (value === token) return 1000 + token.length;
      if (value.includes(token) || token.includes(value)) best = Math.max(best, token.length);
    }
  }
  return best;
}

function bestContext(type: HomeCatalogItem, kind: HomeCatalogKind): CanonicalContext | null {
  let winner: CanonicalContext | null = null;
  let score = 0;
  for (const context of CANONICAL_CONTEXTS) {
    if (context.kind !== kind) continue;
    const next = matchScore(type, context);
    if (next > score) {
      score = next;
      winner = context;
    }
  }
  return score > 0 ? winner : null;
}

function placeholder(context: CanonicalContext, label: string, loading: boolean): HomeCatalogItem {
  return {
    id: context.slug,
    kind: context.kind,
    label,
    technicalKey: context.slug,
    count: loading ? null : 0,
    loading,
  };
}

export function groupHomeCatalog(input: {
  people: HomeCatalogItem;
  documents: HomeCatalogItem[];
  bills: HomeCatalogItem[];
  labels: Record<string, string>;
  documentsLoading?: boolean;
  billsLoading?: boolean;
}): HomeCatalogGroup[] {
  const assigned = new Set<string>();
  const extras: HomeCatalogItem[] = [];

  function isDiscontinuedDocument(type: { technicalKey: string; label: string }): boolean {
    return (
      DISCONTINUED_DOCUMENT_KEYS.has(type.technicalKey) ||
      DISCONTINUED_DOCUMENT_KEYS.has(normalizeCatalogToken(type.technicalKey)) ||
      DISCONTINUED_DOCUMENT_KEYS.has(normalizeCatalogToken(type.label))
    );
  }

  function overlay(
    context: CanonicalContext,
    types: HomeCatalogItem[],
    loading: boolean,
  ): HomeCatalogItem {
    const label = input.labels[context.labelKey] ?? context.slug;
    const match = types.find(
      (type) =>
        !assigned.has(type.id) &&
        !isDiscontinuedDocument(type) &&
        bestContext(type, context.kind)?.slug === context.slug,
    );
    if (!match) return placeholder(context, label, loading);
    assigned.add(match.id);
    return {
      ...match,
      kind: context.kind,
      label,
      technicalKey: context.slug,
    };
  }

  const grouped = new Map<HomeCatalogGroupKey, HomeCatalogItem[]>();
  grouped.set("personal", [{ ...input.people, technicalKey: "dados-pessoais" }]);
  grouped.set(
    "bills",
    CANONICAL_CONTEXTS.filter((context) => context.group === "bills").map((context) =>
      overlay(context, input.bills, Boolean(input.billsLoading)),
    ),
  );
  for (const group of ["identity", "work", "socialHealth", "education", "certificates"] as const) {
    grouped.set(
      group,
      CANONICAL_CONTEXTS.filter((context) => context.group === group).map((context) =>
        overlay(context, input.documents, Boolean(input.documentsLoading)),
      ),
    );
  }

  for (const type of input.bills) {
    if (!assigned.has(type.id)) extras.push({ ...type, kind: "bill" });
  }
  for (const type of input.documents) {
    if (isDiscontinuedDocument(type)) {
      continue;
    }
    if (!assigned.has(type.id)) extras.push({ ...type, kind: "document" });
  }
  if (extras.length > 0) grouped.set("otherDocuments", extras);

  return GROUP_ORDER.flatMap((key) => {
    const items = grouped.get(key);
    return items && items.length > 0 ? [{ key, items }] : [];
  });
}
