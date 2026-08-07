# Frontend conventions

This document defines implementation conventions for the React/Vite frontend. It is technical guidance, not a project-status tracker.

## Ant Design component policy

Ant Design is the default component layer for Gymkhana Database.

Use an Ant Design component whenever it already owns the relevant UI responsibility. Examples include buttons, cards, alerts, typography, dividers, images, forms, inputs, selects, tables, pagination, dialogs, drawers, tags, feedback, loading states, and layout primitives.

Custom React markup and CSS remain appropriate when they provide one of these responsibilities:

- semantic document structure that should remain native HTML, such as `main`, `nav`, or meaningful sections;
- decorative presentation with no component behavior, such as a background or glow layer;
- application-specific composition around Ant Design primitives;
- behavior that Ant Design does not provide and that has a demonstrated product requirement.

Do not recreate an Ant Design control with raw HTML/CSS only to change its appearance. Prefer Ant Design props, theme tokens, component tokens, and a small class override around the Ant Design component.

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
3. register the matching Ant Design locale in `antdLocales`;
4. extend `resolveAppLocale` with the supported browser-language mapping;
5. add locale-resolution and representative UI tests.

The application locale and Ant Design locale must be selected together so application text and library-owned controls never use different languages.

### Component usage

Components read copy through:

```ts
const { messages } = useI18n();
```

Then consume typed keys, for example:

```ts
messages.auth.login.googleButton
```

Do not introduce a second translation object inside a page or component. Shared or new product copy should extend the active catalog contract instead.

## Login implementation

The login page intentionally preserves the visual composition of the legacy Gymkhana Database login while using Ant Design for component responsibilities. `Card`, `Flex`, `Typography`, `Divider`, `Alert`, `Image`, and `Button` remain Ant Design components. Custom login CSS is limited to the legacy page composition, card sizing, glow/background decoration, spacing, and small component overrides.
