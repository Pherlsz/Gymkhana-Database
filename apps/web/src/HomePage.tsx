import { Link, useNavigate } from "@tanstack/react-router";
import { Alert, Button, Dropdown, Skeleton, Tag, Typography } from "antd";
import {
  Baby,
  BadgeCheck,
  Briefcase,
  Car,
  ChevronRight,
  ChevronUp,
  ClipboardList,
  CreditCard,
  Download,
  Droplet,
  FileText,
  GraduationCap,
  HardHat,
  Heart,
  HeartPulse,
  Plane,
  Plus,
  Receipt,
  Scale,
  Smile,
  Stethoscope,
  User,
  Vote,
  Wifi,
  Zap,
} from "lucide-react";
import { useMemo, useState, type ReactNode } from "react";
import { normalizeProfileSearch } from "./ProfilesPage";
import { useI18n } from "./i18n";
import {
  groupHomeCatalog,
  isCanonicalContextSlug,
  normalizeCatalogToken,
  type HomeCatalogItem,
} from "./lib/home/catalogTaxonomy";
import type { HomeInUseTypeChip } from "./lib/home/loadHomeOverview";
import { useHomeOverview } from "./lib/home/useHomeOverview";
import { canManageUsers } from "./lib/roles";
import { tableLinkProps } from "./lib/tables/tableRoutes";
import { useApplicationSession } from "./session";
import { ICON, ICON_STROKE } from "./components/icons";

type HomeCopy = ReturnType<typeof useI18n>["messages"]["home"];

export function HomePage() {
  const session = useApplicationSession();
  const navigate = useNavigate();
  const { messages } = useI18n();
  const copy = messages.home;
  const overview = useHomeOverview();
  const admin = canManageUsers(session.user.role);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());
  const inUseTotal = sumTotals(overview.documentsInUse, overview.billsInUse);
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

  return (
    <div className="home-page">
      <header className="home-page__header">
        <Typography.Title level={1} style={{ margin: 0 }}>
          {copy.welcome.replace("{name}", session.user.display_name)}
        </Typography.Title>
      </header>

      {overview.failed || overview.inUseFailed ? (
        <Alert showIcon title={copy.overviewError} type="warning" />
      ) : null}

      <div className="home-page__rail">
        <section className="home-page__actions">
          <Typography.Text className="home-section__title">
            {copy.quickActions.title}
          </Typography.Text>
          <div className="quick-actions">
            <Dropdown
              menu={{
                items: [
                  {
                    key: "document",
                    icon: <FileText aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />,
                    label: copy.quickActions.documents,
                    onClick: () =>
                      void navigate(
                        tableLinkProps(
                          normalizeProfileSearch({ mode: "create", section: "documents" }),
                        ),
                      ),
                  },
                  {
                    key: "bill",
                    icon: <Receipt aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />,
                    label: copy.quickActions.newBill,
                    onClick: () =>
                      void navigate(
                        tableLinkProps(
                          normalizeProfileSearch({ mode: "create", section: "bills" }),
                        ),
                      ),
                  },
                  ...(admin
                    ? [
                        {
                          type: "group" as const,
                          label: copy.quickActions.assisted,
                          children: [
                            {
                              key: "batch",
                              icon: (
                                <Download aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
                              ),
                              label: copy.quickActions.batch,
                              onClick: () => void navigate({ to: "/admin" }),
                            },
                          ],
                        },
                      ]
                    : []),
                ],
              }}
              trigger={["click"]}
            >
              <Button
                aria-label={copy.quickActions.newRecord}
                className="quick-action quick-action--primary"
                icon={<Plus aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                type="primary"
              >
                <span className="quick-action__copy">
                  <span className="quick-action__label">{copy.quickActions.newRecord}</span>
                  <span className="quick-action__hint">
                    {copy.quickActions.newRecordDescription}
                  </span>
                </span>
              </Button>
            </Dropdown>
            <div className="quick-row">
              {admin ? (
                <Button
                  aria-label={copy.quickActions.googleForm}
                  className="quick-action"
                  icon={<ClipboardList aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />}
                  onClick={() => void navigate({ to: "/forms" })}
                >
                  <span className="quick-action__copy">
                    <span className="quick-action__label">{copy.quickActions.googleForm}</span>
                    <span className="quick-action__hint">
                      {copy.quickActions.googleFormDescription}
                    </span>
                  </span>
                </Button>
              ) : null}
            </div>
          </div>
        </section>

        <div className="home-complement__metrics">
          <section
            className={
              inUseTotal > 0
                ? "home-attention home-attention--warning"
                : "home-attention home-attention--empty"
            }
          >
            <div className="home-attention__copy">
              <Typography.Title className="home-attention__title" level={2}>
                {copy.attention.title}
              </Typography.Title>
              <Typography.Text type="secondary">{copy.attention.subtitle}</Typography.Text>
            </div>
            {inUseTotal > 0 ? (
              <Button
                className="home-attention__open"
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
                    {overview.inUseTypeChips.map((chip) => (
                      <InUseTypeChip chip={chip} key={chip.key} />
                    ))}
                  </div>
                ) : null}
              </>
            )}
          </section>

          <Link
            aria-label={copy.hero.open}
            className="home-hero"
            {...tableLinkProps(normalizeProfileSearch({}))}
          >
            <span className="home-hero__icon">
              <User aria-hidden size={ICON.lg} strokeWidth={ICON_STROKE} />
            </span>
            <span className="home-hero__copy">
              <span className="home-hero__label">{copy.hero.people}</span>
              {overview.loading.profiles ? (
                <Skeleton.Input active size="small" />
              ) : typeof overview.profileTotal === "number" ? (
                <span className="home-hero__value">
                  {overview.profileTotal.toLocaleString("pt-BR")}{" "}
                  <span className="home-hero__unit">{copy.hero.unit}</span>
                </span>
              ) : (
                <span className="home-hero__empty">—</span>
              )}
            </span>
            <span aria-hidden className="home-hero__open">
              <ChevronRight aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
            </span>
          </Link>
        </div>
      </div>
      <div className="home-complement">
        <section>
          <div className="home-section__head">
            <Typography.Text className="home-section__title" style={{ marginBottom: 0 }}>
              {copy.tables.title}
            </Typography.Text>
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
          <div className="home-catalog">
            {catalogGroups.map((group) => (
              <CatalogPanel
                extra={group.items.length}
                key={group.key}
                label={copy.tables.groups[group.key]}
                onToggle={() => toggleGroup(group.key)}
                open={!collapsedGroups.has(group.key)}
              >
                <div className="home-catalog__rows">
                  {group.items.map((item) => (
                    <CatalogRow copy={copy} item={item} key={`${item.kind}-${item.id}`} />
                  ))}
                </div>
              </CatalogPanel>
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}

function CatalogPanel({
  extra,
  label,
  open,
  onToggle,
  children,
}: {
  extra: number;
  label: string;
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  return (
    <section className="home-catalog__group">
      <Button
        aria-expanded={open}
        className="home-catalog__trigger"
        icon={
          <ChevronRight
            aria-hidden
            className={
              open ? "home-catalog__chevron home-catalog__chevron--open" : "home-catalog__chevron"
            }
            size={ICON.md}
            strokeWidth={ICON_STROKE}
          />
        }
        onClick={onToggle}
        type="text"
      >
        <span className="home-catalog__trigger-label">{label}</span>
        <span className="home-catalog__trigger-extra">{extra}</span>
      </Button>
      {open ? <div className="home-catalog__body">{children}</div> : null}
    </section>
  );
}

function CatalogRow({ copy, item }: { copy: HomeCopy; item: HomeCatalogItem }) {
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
  return (
    <Link className="home-catalog__row" {...tableLinkProps(search)}>
      <span className="home-catalog__row-icon">
        <CatalogIcon item={item} />
      </span>
      <span className="home-catalog__row-name">{item.label}</span>
      {item.loading ? (
        <Skeleton.Input active size="small" />
      ) : typeof item.count === "number" ? (
        <span
          className={
            item.count > 0
              ? "home-catalog__row-count"
              : "home-catalog__row-count home-catalog__row-count--empty"
          }
        >
          {item.count.toLocaleString("pt-BR")} <span>{copy.tables.inPossession}</span>
        </span>
      ) : (
        <span className="home-catalog__row-count home-catalog__row-count--empty">—</span>
      )}
      <ChevronRight
        aria-hidden
        className="home-catalog__row-open"
        size={ICON.md}
        strokeWidth={ICON_STROKE}
      />
    </Link>
  );
}

function CatalogIcon({ item }: { item: HomeCatalogItem }) {
  const Icon = catalogIcon(item);
  return <Icon aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />;
}

function catalogIcon(item: HomeCatalogItem) {
  if (item.kind === "people") return User;
  const token = `${normalizeCatalogToken(item.technicalKey)} ${normalizeCatalogToken(item.label)}`;
  if (item.kind === "bill") {
    if (token.includes("agua")) return Droplet;
    if (token.includes("internet") || token.includes("wifi")) return Wifi;
    return Zap;
  }
  if (token.includes("cnh")) return Car;
  if (token.includes("passaporte")) return Plane;
  if (token.includes("rg")) return BadgeCheck;
  if (token.includes("ctps") || token.includes("pis") || token.includes("pasep")) return Briefcase;
  if (token.includes("crea")) return HardHat;
  if (token.includes("oab")) return Scale;
  if (token.includes("crm")) return Stethoscope;
  if (token.includes("coren")) return HeartPulse;
  if (token.includes("cro")) return Smile;
  if (token.includes("titulo")) return Vote;
  if (token.includes("sus")) return Heart;
  if (token.includes("cidadao")) return CreditCard;
  if (token.includes("estudant")) return GraduationCap;
  if (token.includes("nascimento")) return Baby;
  if (token.includes("casamento")) return Heart;
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
    <Link {...tableLinkProps(search)}>
      <Tag>
        {chip.typeLabel} <strong>{chip.count}</strong>
      </Tag>
    </Link>
  );
}

function sumTotals(...values: Array<number | null | undefined>): number {
  return values.reduce<number>((sum, value) => sum + (typeof value === "number" ? value : 0), 0);
}
