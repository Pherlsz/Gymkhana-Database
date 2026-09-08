import { useNavigate } from "@tanstack/react-router";
import { Alert, Button, Dropdown, Skeleton } from "antd";
import {
  ChevronRight,
  ChevronUp,
  ClipboardList,
  Download,
  FileText,
  Plus,
  Receipt,
  RefreshCw,
  User,
} from "lucide-react";
import { useMemo, useState } from "react";
import { normalizeProfileSearch } from "./ProfilePanel";
import { CADASTRO_SEARCH_DEFAULTS } from "./lib/cadastro/cadastroSearch";
import { useI18n } from "./i18n";
import { groupHomeCatalog } from "./lib/home/catalogTaxonomy";
import { groupCategoryIcon, groupCategoryTheme } from "./lib/home/catalogIcons";
import { CatalogRow } from "./lib/home/CatalogRow";
import { HomeAttentionCard } from "./lib/home/HomeAttentionCard";
import { useHomeOverview } from "./lib/home/useHomeOverview";
import { useContainerColumns } from "./hooks/useContainerColumns";
import { canManageUsers } from "./lib/roles";
import { tableLinkProps } from "./lib/tables/tableRoutes";
import "./home.css";
import { useApplicationSession } from "./session";
import { ICON, ICON_STROKE } from "./components/icons";
import { AppCard } from "./components/AppCard";

export function HomePage() {
  const session = useApplicationSession();
  const navigate = useNavigate();
  const { messages, t } = useI18n();
  const copy = messages.home;
  const overview = useHomeOverview();
  const admin = canManageUsers(session.user.role);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());

  const columnsCount = useContainerColumns();

  const documentsInUseTotal =
    typeof overview.documentsInUse === "number" ? overview.documentsInUse : 0;
  const billsInUseTotal = typeof overview.billsInUse === "number" ? overview.billsInUse : 0;

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
        labels: copy.tables.contexts,
        documentsLoading: overview.loading.documentTypes,
        billsLoading: overview.loading.billTypes,
      }).filter((group) => group.key !== "personal"),
    [
      copy.tables.contexts,
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

  const catalogCards = useMemo(() => {
    if (catalogGroups.length === 0) {
      return (["bills", "identity", "work"] as const).map((key) => {
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
      });
    }

    return catalogGroups.map((group) => {
      const Icon = groupCategoryIcon(group.key);
      const theme = groupCategoryTheme(group.key);
      const extra = group.items.length;
      const isGroupLoading =
        (group.key === "bills" && overview.loading.billTypes) ||
        (group.key !== "bills" && overview.loading.documentTypes);
      const isGroupEmpty = !isGroupLoading && group.items.length === 0;
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
    });
  }, [
    catalogGroups,
    collapsedGroups,
    copy.quickActions.documents,
    copy.quickActions.newBill,
    copy.tables.emptyGroupDescription,
    copy.tables.emptyGroupTitle,
    copy.tables.groups,
    copy.tables.singleTypeBadge,
    copy.tables.typesBadge,
    navigate,
    overview.loading.billTypes,
    overview.loading.documentTypes,
    t,
  ]);

  const effectiveColumns = Math.min(columnsCount, Math.max(1, catalogCards.length));
  const masonryColumns = useMemo(() => {
    const cols: React.ReactNode[][] = Array.from({ length: effectiveColumns }, () => []);
    catalogCards.forEach((card, idx) => {
      const target = cols[idx % effectiveColumns];
      if (target) {
        target.push(card);
      }
    });
    return cols;
  }, [catalogCards, effectiveColumns]);

  return (
    <div className="home-page">
      <header className="home-header-hero">
        <div className="home-header-hero__copy">
          <h1 className="home-header-hero__title">
            {t(copy.welcome, { name: session.user.display_name || session.user.login })}
          </h1>
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

        <HomeAttentionCard
          billsInUseTotal={billsInUseTotal}
          documentsInUseTotal={documentsInUseTotal}
          overview={overview}
        />
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
          {masonryColumns.map((colCards, colIdx) => (
            <div key={colIdx} className="home-bento-grid__col">
              {colCards}
            </div>
          ))}
        </div>
      </main>
    </div>
  );
}
