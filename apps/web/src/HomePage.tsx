import { Link, useNavigate } from "@tanstack/react-router";
import { Alert, Button, Dropdown, Skeleton, Typography } from "antd";
import {
  Baby,
  BookUser,
  Briefcase,
  Bus,
  Car,
  ChevronRight,
  ChevronUp,
  ClipboardList,
  Coins,
  CreditCard,
  Download,
  Droplet,
  Dumbbell,
  FileText,
  GraduationCap,
  HardHat,
  Heart,
  HeartHandshake,
  HeartPulse,
  IdCard,
  Plane,
  Plus,
  Receipt,
  RefreshCw,
  Scale,
  ScrollText,
  ShieldCheck,
  Smile,
  Stethoscope,
  Ticket,
  User,
  Vote,
  Wifi,
  Zap,
} from "lucide-react";
import { useMemo, useState } from "react";
import { normalizeProfileSearch } from "./ProfilePanel";
import { CADASTRO_SEARCH_DEFAULTS } from "./lib/cadastro/cadastroSearch";
import { useI18n } from "./i18n";
import {
  groupHomeCatalog,
  isCanonicalContextSlug,
  normalizeCatalogToken,
  type HomeCatalogItem,
} from "./lib/home/catalogTaxonomy";
import type { HomeInUseTypeChip } from "./lib/home/useHomeOverview";
import { useHomeOverview } from "./lib/home/useHomeOverview";
import { canManageUsers } from "./lib/roles";
import { tableLinkProps } from "./lib/tables/tableRoutes";
import { useApplicationSession } from "./session";
import { ICON, ICON_STROKE } from "./components/icons";
import { AppCard } from "./components/AppCard";

const CHIP_LIMIT = 8;

export function HomePage() {
  const session = useApplicationSession();
  const navigate = useNavigate();
  const { messages, t } = useI18n();
  const copy = messages.home;
  const overview = useHomeOverview();
  const admin = canManageUsers(session.user.role);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());
  const [showAllChips, setShowAllChips] = useState(false);
  const documentsInUseTotal =
    typeof overview.documentsInUse === "number" ? overview.documentsInUse : 0;
  const billsInUseTotal = typeof overview.billsInUse === "number" ? overview.billsInUse : 0;
  const inUseTotal = documentsInUseTotal + billsInUseTotal;

  const catalogGroups = useMemo(
    () =>
      groupHomeCatalog({
        people: {
          id: "people",
          kind: "people",
          label: copy.tables.personalRecord,
          technicalKey: "dados-pessoais",
          count: overview.profileTotal,
          loading: overview.loading.profiles,
        },
        documents: overview.documentTypes.map((type) => ({ ...type, kind: "document" as const })),
        bills: overview.billTypes.map((type) => ({ ...type, kind: "bill" as const })),
        labels: messages.common.labels,
        documentsLoading: overview.loading.documentTypes,
        billsLoading: overview.loading.billTypes,
      }).filter((group) => group.key !== "personal"),
    [
      messages.common.labels,
      copy.tables.personalRecord,
      overview.billTypes,
      overview.documentTypes,
      overview.loading.billTypes,
      overview.loading.documentTypes,
      overview.loading.profiles,
      overview.profileTotal,
    ],
  );
  const allGroupsOpen = catalogGroups.every((group) => !collapsedGroups.has(group.key));

  function toggleGroup(key: string) {
    setCollapsedGroups((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  return (
    <div className="home-page">
      <header className="home-header-hero">
        <div className="home-header-hero__copy">
          <Typography.Title level={1} className="home-header-hero__title" style={{ margin: 0 }}>
            {t(copy.welcome, { name: session.user.display_name || session.user.login })}
          </Typography.Title>
        </div>
        <div className="home-header-hero__actions">
          <Dropdown
            menu={{
              onClick: ({ key }) => {
                if (key === "person") {
                  void navigate({
                    to: "/cadastro",
                    search: { ...CADASTRO_SEARCH_DEFAULTS, table: "people", mode: "manual" },
                  });
                  return;
                }
                if (key === "document") {
                  void navigate({
                    to: "/cadastro",
                    search: { ...CADASTRO_SEARCH_DEFAULTS, table: "documents" },
                  });
                  return;
                }
                if (key === "bill") {
                  void navigate({
                    to: "/cadastro",
                    search: { ...CADASTRO_SEARCH_DEFAULTS, table: "bills" },
                  });
                  return;
                }
                if (key === "batch") {
                  void navigate({
                    to: "/cadastro",
                    search: { ...CADASTRO_SEARCH_DEFAULTS, table: "people", mode: "xlsx" },
                  });
                }
              },
              items: [
                {
                  key: "person",
                  icon: <User aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />,
                  label: copy.quickActions.newRecord,
                },
                {
                  key: "document",
                  icon: <FileText aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />,
                  label: copy.quickActions.documents,
                },
                {
                  key: "bill",
                  icon: <Receipt aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />,
                  label: copy.quickActions.newBill,
                },
                ...(admin
                  ? [
                      {
                        type: "divider" as const,
                      },
                      {
                        key: "batch",
                        icon: <Download aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />,
                        label: copy.quickActions.batch,
                      },
                    ]
                  : []),
              ],
            }}
            trigger={["click"]}
          >
            <Button
              aria-label={copy.quickActions.newRecord}
              className="home-header-hero__btn"
              icon={<Plus aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
              type="primary"
            >
              <span>{copy.quickActions.newRecord}</span>
            </Button>
          </Dropdown>

          {admin ? (
            <Button
              aria-label={copy.quickActions.googleForm}
              className="home-header-hero__action-btn"
              icon={<ClipboardList aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
              onClick={() =>
                void navigate({
                  search: { ...CADASTRO_SEARCH_DEFAULTS, mode: "forms" },
                  to: "/cadastro",
                })
              }
            >
              <span>{copy.quickActions.googleForm}</span>
            </Button>
          ) : null}
        </div>
      </header>

      <div className="home-top-strip">
        <AppCard
          aria-label={copy.hero.open}
          chevron
          className="home-hero-card"
          icon={<User size={22} strokeWidth={ICON_STROKE} />}
          subtitle={
            overview.loading.profiles ? (
              <Skeleton.Input active size="small" style={{ width: "5rem" }} />
            ) : typeof overview.profileTotal === "number" ? (
              <>
                <span className="app-card__stat-num">
                  {overview.profileTotal.toLocaleString("pt-BR")}
                </span>{" "}
                <span className="app-card__stat-unit">{copy.hero.unit}</span>
              </>
            ) : (
              "—"
            )
          }
          title={copy.hero.people}
          variant="people"
          {...tableLinkProps(normalizeProfileSearch({}))}
        />

        <section
          className={
            inUseTotal > 0
              ? "home-attention home-attention--warning"
              : "home-attention home-attention--empty"
          }
        >
          <div className="home-attention__copy">
            <div className="home-attention__title-row">
              <Typography.Title className="home-attention__title" level={2}>
                {copy.attention.title}
              </Typography.Title>
            </div>
            <Typography.Text type="secondary">{copy.attention.subtitle}</Typography.Text>
          </div>
          {inUseTotal > 0 ? (
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
            <Typography.Text className="home-attention__none" type="secondary">
              {copy.attention.none}
            </Typography.Text>
          ) : (
            <>
              <p className="home-attention__value">{inUseTotal.toLocaleString("pt-BR")}</p>
              {overview.inUseTypeChips.length > 0 ? (
                <div className="home-attention__chips">
                  {(showAllChips
                    ? overview.inUseTypeChips
                    : overview.inUseTypeChips.slice(0, CHIP_LIMIT)
                  ).map((chip) => (
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
      </div>

      {overview.failed || overview.inUseFailed ? (
        <Alert
          action={
            <Button
              icon={<RefreshCw aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />}
              onClick={() => void overview.refetch()}
              size="small"
            >
              {copy.retry}
            </Button>
          }
          role="alert"
          showIcon
          title={copy.overviewError}
          type="warning"
        />
      ) : null}

      <main className="home-catalog-section">
        <div className="home-section__head">
          <h2 className="home-section__title" style={{ marginBottom: 0 }}>
            {copy.tables.title}
          </h2>
          <Button
            aria-label={allGroupsOpen ? copy.tables.collapseAll : copy.tables.expandAll}
            icon={
              allGroupsOpen ? (
                <ChevronUp aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              ) : (
                <ChevronRight aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
              )
            }
            onClick={() =>
              setCollapsedGroups(
                allGroupsOpen ? new Set(catalogGroups.map((group) => group.key)) : new Set(),
              )
            }
            size="small"
            title={allGroupsOpen ? copy.tables.collapseAll : copy.tables.expandAll}
            type="text"
          />
        </div>

        <div className="home-catalog home-bento-grid">
          {catalogGroups.length === 0 ? (
            (["bills", "identity", "work"] as const).map((key) => {
              const Icon = groupCategoryIcon(key);
              const theme = groupCategoryTheme(key);
              const isCatalogLoading =
                overview.loading.documentTypes || overview.loading.billTypes;
              return (
                <AppCard
                  empty={!isCatalogLoading}
                  emptyDescription={copy.tables.emptyGroupDescription}
                  emptyTitle={copy.tables.emptyGroupTitle}
                  icon={<Icon size={18} strokeWidth={ICON_STROKE} />}
                  key={key}
                  loading={isCatalogLoading}
                  open
                  title={copy.tables.groups[key]}
                  variant={theme}
                />
              );
            })
          ) : (
            catalogGroups.map((group) => {
              const Icon = groupCategoryIcon(group.key);
              const theme = groupCategoryTheme(group.key);
              const extra = group.items.length;
              const isGroupLoading =
                (group.key === "bills" && overview.loading.billTypes) ||
                (group.key !== "bills" && overview.loading.documentTypes);
              const hasAnyRegistered = group.items.some(
                (item) => typeof item.count === "number" && item.count > 0,
              );
              const isGroupEmpty = !isGroupLoading && !hasAnyRegistered;
              const badgeText = isGroupLoading
                ? undefined
                : extra === 1
                  ? copy.tables.singleTypeBadge
                  : t(copy.tables.typesBadge, { count: extra });

              return (
                <AppCard
                  badge={badgeText}
                  empty={isGroupEmpty}
                  emptyAction={
                    <Button
                      onClick={(e) => {
                        e.stopPropagation();
                        void navigate({
                          to: "/cadastro",
                          search: {
                            ...CADASTRO_SEARCH_DEFAULTS,
                            table: group.key === "bills" ? "bills" : "documents",
                          },
                        });
                      }}
                      size="small"
                      type="primary"
                    >
                      {group.key === "bills"
                        ? copy.quickActions.newBill
                        : copy.quickActions.documents}
                    </Button>
                  }
                  emptyDescription={copy.tables.emptyGroupDescription}
                  emptyTitle={copy.tables.emptyGroupTitle}
                  icon={<Icon size={18} strokeWidth={ICON_STROKE} />}
                  key={group.key}
                  loading={isGroupLoading}
                  loadingRows={extra > 0 ? Math.min(extra, 4) : 3}
                  onToggle={() => toggleGroup(group.key)}
                  open={!collapsedGroups.has(group.key)}
                  title={copy.tables.groups[group.key]}
                  variant={theme}
                >
                  <div className="home-catalog__rows">
                    {group.items.map((item) => (
                      <CatalogRow item={item} key={`${item.kind}-${item.id}`} />
                    ))}
                  </div>
                </AppCard>
              );
            })
          )}
        </div>
      </main>
    </div>
  );
}

function CatalogRow({ item }: { item: HomeCatalogItem }) {
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

function groupCategoryIcon(key: string) {
  switch (key) {
    case "personal":
      return User;
    case "bills":
      return Zap;
    case "identity":
      return ShieldCheck;
    case "work":
      return Briefcase;
    case "socialHealth":
      return HeartPulse;
    case "education":
      return GraduationCap;
    case "certificates":
      return ScrollText;
    default:
      return ClipboardList;
  }
}

function groupCategoryTheme(
  key: string,
): "bills" | "identity" | "work" | "health" | "education" | "certificates" | "other" {
  switch (key) {
    case "bills":
      return "bills";
    case "identity":
      return "identity";
    case "work":
      return "work";
    case "socialHealth":
      return "health";
    case "education":
      return "education";
    case "certificates":
      return "certificates";
    default:
      return "other";
  }
}

function catalogSubIcon(item: HomeCatalogItem): typeof FileText {
  if (item.kind === "people") return User;
  const token = `${normalizeCatalogToken(item.technicalKey)} ${normalizeCatalogToken(item.label)}`;

  if (item.kind === "bill" || token.includes("luz") || token.includes("energia")) {
    if (token.includes("agua")) return Droplet;
    if (token.includes("internet") || token.includes("wifi")) return Wifi;
    return Zap;
  }

  if (token.includes("cnh") || token.includes("habilitacao")) return Car;
  if (token.includes("passaporte")) return Plane;
  if (token.includes("rg") || token.includes("identidade")) return IdCard;

  if (
    token.includes("ctps") ||
    token.includes("carteira de trabalho") ||
    token.includes("trabalho")
  )
    return BookUser;
  if (token.includes("pis") || token.includes("pasep")) return Coins;
  if (token.includes("crea")) return HardHat;
  if (token.includes("oab")) return Scale;
  if (token.includes("crm")) return Stethoscope;
  if (token.includes("cro")) return Smile;
  if (token.includes("coren")) return HeartPulse;

  if (token.includes("titulo")) return Vote;
  if (token.includes("sus")) return Heart;
  if (token.includes("cidadao")) return CreditCard;

  if (token.includes("estudant") || token.includes("escolar")) return GraduationCap;

  if (token.includes("nascimento")) return Baby;
  if (token.includes("casamento")) return HeartHandshake;

  if (token.includes("cref")) return Dumbbell;
  if (token.includes("teu")) return Bus;
  if (token.includes("tri")) return Ticket;

  return FileText;
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
