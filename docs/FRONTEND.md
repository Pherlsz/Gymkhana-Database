# Frontend conventions

This document defines implementation conventions for the React/Vite frontend. It is technical guidance, not a project-status tracker. Product, UX, and route rules live in [`ORCHESTRATION.md`](ORCHESTRATION.md) §12.

## Visual components

Orchestration §12.3: Ant Design is the visual base. Visual composition stays in this repository; there is no shared UI package. Do not adopt Material UI, shadcn, or a from-scratch design system as the principal visual dependency.

**Color:** every color literal lives in the token blocks of `apps/web/src/shell.css`; nothing else in the app declares one. The world is ink and paper — high-contrast neutrals at hue 85 with chroma ≤0.006, hairlines instead of gray fills, gold reserved for what was earned or for "you are here", red for destructive.

Map Ant component colors in `ConfigProvider` (`cssVar`, `hashed: false`) and product wrappers (`.spreadsheet-table`, `.nav-item`). Do not restyle Ant internals with `.ant-*` color selectors or color `!important`; the virtual scrollbar thumb in `tables.css` is the single documented exception, because Ant paints it inline. Mixable seeds (`colorPrimary`, `colorError`, `colorWarning`, `colorSuccess`, `colorInfo`, `colorTextLightSolid`) stay hex literals in `theme.tsx` mirroring the matching `--md-*` role, because FastColor turns a CSS variable into black.

Roles:

- `--md-primary` (`#ffc53d`) is **fill only**, and only on the earned owner disc. Never put it on paper as text, as a 1px mark, as a checkbox, or as a button. It is ~1.5:1 on paper.
- `--md-primary-text` is the "you are here" gold: the active nav inset rule, the matching nav icon, and the focus ring. It clears 4.5:1 on paper, on the container tint and on `--md-state-selected`. Do not use it for hover borders, chip outlines, filter values, or control fills.
- `--md-state-hover` / `-pressed` / `-selected` are ink washes (5% / 9% / 12% of on-surface). States are never gold; that is what keeps gold meaningful.
- `--md-nav-active` is the faint gold surface behind the active nav row. The signal is the `inset 2px` `--md-primary-text` rule plus the gold icon, not the wash.
- Solid and outlined primary buttons are ink on paper, not gold slabs. Checkboxes and radios fill with `--md-on-surface`.
- Feedback: `--md-error`, `--md-warning`, `--md-info`, `--md-success`. Hues stay apart (27 / 45 / 250 / 155) and away from gold (68). Do not reuse gold for info. Do not gold the presence marks: physical is `--mark-physical`, digital is `--mark-digital`.
- Also available: `--md-scrim`, `--md-disabled`, `--md-surface-elevated`, `--shadow-card`, `--shadow-overlay`, `--mark-physical` / `--mark-digital` (Orchestration §12.1 presence marks), and `--gym-highlight` / `--gym-hairline` / `--gym-shadow-ambient` for the login glass.

**Type:** Geist carries body, tables, forms and numbers. Page titles (`.page-header__title`) use Oswald condensed; never put the display face on a cell, chip, or identifier. Identifier and amount columns use `font-variant-numeric: tabular-nums`.

Form pages use `.page-measure` (`--page-measure: 64rem`); the spreadsheet does not.

## Page chrome and query states

Authenticated screens compose chrome and async states from `apps/web/src/components/`:

- **`PageShell`** wraps the page class (`tables-page`, `admin-page`, `cadastro-page`, `search-page`). Form pages pass `measure` for `.page-measure`. Optional `title` / `description` / `actions` render `PageHeader` (Oswald on `.page-header__title`). Home keeps its catalog hero. Cadastro work modes (Forms / XLSX) keep `CadastroWorkShell` crumbs instead of a second page title.
- **`QueryView`** is the exclusive pending / error / empty switch over `StateCard`. Do not name it `QueryState` — that file encodes table URL plans. Search results are not exclusive: catalog and result errors can coexist, so they use banners plus `InlineStatus`.
- **`StatusBanner`** is the shared Ant `Alert` for success, mutation failure, OAuth notices, and 409 conflict (warning). Optional `action` covers retry / “vincular existente”. Inline list footnotes use `InlineStatus`. Spreadsheet / popover empty stays Ant `Empty`. Home catalog loading stays `AppCard` row skeletons.

**Icons:** product chrome uses Lucide (`lucide-react`), `strokeWidth={1.75}`. Do not import `@ant-design/icons` in app source. Ant Design may still render its own glyphs inside Select, DatePicker, and Pagination. Brand marks (Google) stay as local SVG. Document presence marks (Orchestration §6.4) are Lucide `File` (physical), `ScanLine` (digital) and `UserRound` (with the owner), not the letters `F` / `D` / `(i)`. The number mark stays the `nº` glyph.

Do not rebuild an Ant Design control in raw HTML only to restyle it. Pages compose Ant Design (`Table`, `Form`, `Select`, `Pagination`, overlays) plus module-owned wrappers under `apps/web/src/lib/<area>/`.

Custom React markup and CSS remain appropriate for:

- semantic document structure that should remain native HTML, such as `main`, `nav`, or meaningful sections;
- decorative presentation with no component behavior, such as a background or glow layer;
- application-specific composition around Ant Design controls;
- behavior that Ant Design does not provide and that has a demonstrated product requirement.

## File-based routing

TanStack Router file-based routing is the frontend routing standard. Route declarations live under `apps/web/src/routes/`; `App.tsx` owns application bootstrap and authentication gating, not route registration.

Approved SPA destinations: `/`, `/tables/people`, `/tables/documents`, `/tables/bills`, `/search`, `/admin`, `/cadastro`. `/tables` without a type redirects to `/tables/people`. `/profiles` may redirect for compatibility; it is not a destination. `/forms` redirects to `/cadastro?mode=forms` (Google Forms is a Cadastro submodule, not a nav destination).

Do not add `/chat`, `/query`, `/tasks`, `/ocr`, `/operations`, `/matching`, `/custom-data`, or `/google-forms` as destinations.

The Vite TanStack Router plugin must remain before the React plugin in `apps/web/vite.config.ts`. It reads `src/routes` and regenerates `src/routeTree.gen.ts`. The generated route tree is committed because the repository runs TypeScript validation before the Vite build on a fresh checkout. Never hand-edit the generated file as normal feature work; change route files and regenerate it through the router plugin.

Keep route modules thin. A route file should normally declare its URL, search validation, and page component. Reusable page or shell implementations belong outside `src/routes` when that avoids coupling or circular imports.

In particular, route modules must not import `App.tsx`. The dependency direction is:

```text
App -> router -> routeTree.gen -> routes -> page/shell components
```

Shared authenticated session state lives in `src/session.ts`. Shared role predicates live in `src/lib/roles.ts`. Do not duplicate either inside route modules.

When adding or changing routes:

1. add or update the corresponding file under `src/routes`;
2. preserve URL-backed search normalization when the page has query state;
3. regenerate `src/routeTree.gen.ts` through Vite/TanStack Router tooling;
4. run frontend typechecking/tests and verify direct navigation to the changed route;
5. do not recreate a parallel manual `createRoute` tree in `App.tsx`.

## Hooks, components, and file size

Orchestration §23.5: extract what can be a hook or a named component. Reusable units (two or more call sites, or the app shell) live in `apps/web/src/hooks/` and `apps/web/src/components/`. Feature-owned units stay under `apps/web/src/lib/<area>/` (for example `lib/tables`, `lib/home`). Pages orchestrate; they do not accumulate thousands of lines. A 1000+ line implementation file is not acceptable — extract in the same change when touching a large file. Do not move code to the global folders without a second consumer, and do not split into shallow pass-through files.

## Internationalization contract

User-facing application copy must not be hardcoded inside React components. Copy belongs in the versioned catalog under `apps/web/src/i18n/` and is consumed through `useI18n()`.

Developer-only diagnostics, stable technical identifiers, API paths, error codes, test fixtures, and non-user-facing log text do not belong in the translation catalog.

The initial locale is `pt-BR`. Unsupported browser locales currently fall back to `pt-BR`.

### Catalog versioning

The current catalog contract is `v1`, exposed as `I18N_CATALOG_VERSION = 1`.

`apps/web/src/i18n/v1/catalog.ts` defines the typed message shape. Every locale for v1 must implement that shape.

The following are backward-compatible and do not require a new catalog major version:

- changing wording or punctuation in an existing translation;
- adding a new locale that implements the existing v1 contract;
- adding a new message key without removing or changing existing keys.

Create a new catalog major version when an existing contract is broken, such as:

- removing a message key;
- renaming or moving an existing key;
- changing a key from one semantic purpose to another;
- restructuring a catalog subtree in a way that requires consumers to change.

A catalog-version change must be explicit in code and migrated deliberately. Do not silently repurpose old keys.

### Adding a language

To add another language to the current catalog version:

1. add a locale file under `apps/web/src/i18n/v1/` that satisfies `CatalogV1`;
2. register it in the `catalogs` map in `apps/web/src/i18n/index.tsx`;
3. extend `resolveAppLocale` with the supported browser-language mapping;
4. add locale-resolution and representative UI tests;
5. keep Ant Design locale in sync with the catalog locale so application text and library-owned controls do not use different languages.

### Component usage

Components read copy through:

```ts
const { messages } = useI18n();
```

Then consume typed keys, for example:

```ts
messages.auth.login.googleButton;
```

Do not introduce a second translation object inside a page or component. Shared or new product copy should extend the active catalog contract instead.

## Login implementation

The login page preserves the visual composition of the legacy Gymkhana Database login (card sizing, glow/background, spacing). Custom markup on that screen is leftover until login is migrated to Ant Design chrome.
