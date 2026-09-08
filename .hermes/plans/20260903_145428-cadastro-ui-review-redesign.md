# Cadastro Flow & Layout — Review + Redesign Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Make `/cadastro` clean, modern, and easier to use: fix the UX mechanics (lost save feedback, no progress indication, mismatched header controls, hover-only disabled reasons) AND give the page a calmer visual treatment — hairline cards with soft shadow, a numbered step trail, and consistent spacing — all inside the existing ink/paper token system.

**Architecture:** Two phases. **Phase A (mechanics):** state machine in `cadastroSearch.ts` stays untouched; feedback + progress plumbing. **Phase B (visuals):** one new `.cadastro-card` surface treatment applied to each work block, a numbered `CadastroSteps` trail replacing the floating "Trocar tipo" button, and spacing/typography polish. No new color literals anywhere — every color comes from the token blocks in `shell.css`. **Do NOT apply the blue/orange palette from `design-system/gymkhana-cadastro/MASTER.md`** — `pages/cadastro.md` overrides it for this screen.

**Tech Stack:** React 19, Ant Design 6, TanStack Router/Query, Vitest + Testing Library, i18n catalog v1 (pt-BR).

---

## Review findings (evidence)

| #   | Finding                                                                                                                                    | Evidence                                                                          | Severity      |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------- | ------------- |
| R1  | Success notice never seen: `onSaved` sets `notice` then `goToRecord()` navigates away                                                      | `CadastroPage.tsx:300-303`                                                        | High          |
| R2  | No progress indication Tipo → Dono → Registro                                                                                              | `CadastroPage.tsx:234-348`; `pages/mode-picker.md` itself lists this anti-pattern | Medium        |
| R3  | Header tools row mixes `SegmentedTabs` with a text `Button` ("Trocar tipo") — different volumes in one scope                               | `CadastroPage.tsx:200-224`; Pedro's UI rule                                       | Medium        |
| R4  | OCR/Forms disabled reasons only in `title` tooltips                                                                                        | `CadastroPage.tsx:175-187`                                                        | Medium        |
| V1  | **Flat, harsh page**: work blocks stack as bare columns with no surface hierarchy; everything sits at the same visual level on gray canvas | `cadastro.css` (107 lines, mostly flex/gap only)                                  | High (visual) |
| V2  | **Crowded header**: two Segmented controls + a text button jammed into `PageHeader` actions with a 0.5rem gap                              | `CadastroPage.tsx:200-224`                                                        | Medium        |
| V3  | **No rhythm between steps**: catalog → owner → record blocks appear/disappear with no container to anchor the eye                          | `CadastroPage.tsx:234-348`                                                        | Medium        |
| R5  | Dead `manual + people` null branch in `CadastroPanel.tsx:42-44`                                                                            | —                                                                                 | Low           |
| R6  | Catalog loading reuses OCR copy (`ocrCheckingHint`)                                                                                        | `CadastroTypePicker.tsx:124-125`                                                  | Low           |

Already good (keep): URL-search state machine + tests, owner combobox via `SearchField lookup suggest`, `.home-catalog` rows, `.page-measure` form-page layout.

---

# Phase A — Mechanics

### Task 1: Make save feedback visible (fix R1)

**Objective:** After creating a record the user actually sees the success message.

**Files:**

- Modify: `apps/web/src/CadastroPage.tsx` (~lines 230-232, 299, 326)
- Create: `apps/web/src/lib/cadastro/cadastroFeedback.ts`
- Test: `apps/web/src/lib/cadastro/cadastroFeedback.test.tsx`

**Step 1: Verify antd message wiring (read-only)**

Run: `grep -rn "App.useApp" apps/web/src --include=*.tsx | head`
If the app is wrapped in antd `<App>`, plan `App.useApp().message`; else static `message` from `antd`. Note the choice.

**Step 2: Write failing test**

```tsx
// apps/web/src/lib/cadastro/cadastroFeedback.test.tsx
import { describe, expect, it, vi } from "vitest";
import { announceSaved } from "./cadastroFeedback";

it("shows the saved message", () => {
  const success = vi.fn();
  announceSaved({ success }, "Documento salvo");
  expect(success).toHaveBeenCalledWith("Documento salvo");
});
```

**Step 3: Run test to verify failure**

Run: `pnpm --filter @pherlsz/gymkhana-database-web exec vitest run src/lib/cadastro/cadastroFeedback.test.tsx`
Expected: FAIL — cannot resolve `./cadastroFeedback`.

**Step 4: Minimal implementation**

```ts
// apps/web/src/lib/cadastro/cadastroFeedback.ts
import { message } from "antd";

export function announceSaved(
  sink: { success: (content: string) => void } = message,
  content: string,
): void {
  sink.success(content);
}
```

**Step 5: Run test to verify pass**

Run: same vitest command. Expected: PASS.

**Step 6: Wire into `CadastroPage.tsx`**

- People path `onSaved` (~line 300): drop `setNotice(message)`; call `announceSaved(undefined, message);` then `goToRecord("people", value.id);`.
- Manual docs/bills path: replace `onNotice={setNotice}` with `onNotice={(m) => announceSaved(undefined, m)}`.
- Grep `setNotice`; if no writers remain, delete the `notice` state and the top `<Alert>` block (lines 51, 230-232).

**Step 7: Verify** — Run: `pnpm typecheck:web && pnpm test:web` — Expected: green.

**Step 8: Commit**

```bash
git add apps/web/src/CadastroPage.tsx apps/web/src/lib/cadastro/cadastroFeedback.ts apps/web/src/lib/cadastro/cadastroFeedback.test.tsx
git commit -m "fix(cadastro): show save feedback before navigating away"
```

---

### Task 2: i18n copy for steps + hints

**Objective:** All new strings land in the versioned catalog (no hardcoded copy).

**Files:** Modify `apps/web/src/i18n/v1/pt-BR.ts` (and `catalog.ts` type + any sibling locale); check `I18N_CATALOG_VERSION` policy in `apps/web/src/i18n/index.tsx`.

**Step 1: Add under `tables.cadastro`:**

```ts
stepsType: "Tipo",
stepsMethod: "Método",
stepsOwner: "Dono",
stepsRecord: "Registro",
typesLoadingHint: "Carregando tipos…",
methodDisabledOcr: "OCR indisponível no momento.",   // reuse copy.ocrDisabledR2 verbatim if identical
methodDisabledForms: "Formulários exigem permissão de administração.", // reuse copy.formsDisabled if identical
```

**Step 2: Verify** — Run: `pnpm typecheck:web && pnpm --filter @pherlsz/gymkhana-database-web exec vitest run src/i18n` — Expected: PASS.

**Step 3: Commit** — `git add apps/web/src/i18n && git commit -m "feat(i18n): cadastro step-trail and hint copy"`

---

# Phase B — Visual redesign

### Task 3: The `.cadastro-card` surface (fix V1, V3)

**Objective:** Give every work block a quiet surface — hairline border, soft shadow, generous padding — so steps read as separate, calm cards instead of a flat stack.

**Files:**

- Modify: `apps/web/src/cadastro.css`
- Modify: `apps/web/src/CadastroPage.tsx`
- Modify: `apps/web/src/lib/cadastro/CadastroTypePicker.tsx`, `CadastroManualPanel.tsx`, `CadastroOwnerPicker.tsx` (add `className="cadastro-card"` to their roots where they own a root element; otherwise wrap in `CadastroPage`)

**Step 1: Add the card class (DRY — one class, applied everywhere)**

Append to `cadastro.css`:

```css
/* One surface treatment for every cadastro work block. Hairline + soft
   shadow + raised paper: the card, not the canvas, is the unit of the flow. */
.cadastro-card {
  display: flex;
  flex-direction: column;
  gap: var(--gym-space-4);
  padding: clamp(1rem, 2.2vw, 1.5rem);
  background: var(--md-surface-elevated);
  border: 1px solid var(--md-outline-variant);
  border-radius: var(--gym-radius-lg);
  box-shadow: var(--shadow-card);
}

.cadastro-card + .cadastro-card {
  margin-top: 0; /* parent gap already spaces; guard against double spacing */
}

@media (max-width: 600px) {
  .cadastro-card {
    padding: 0.875rem;
    border-radius: var(--gym-radius-md);
  }
}
```

**Step 2: Apply it**

- `CadastroTypePicker` root → `cadastro-card`.
- `CadastroManualPanel` root → `cadastro-card`.
- `CadastroOwnerPicker` root → `cadastro-card`.
- In `CadastroPage.tsx`, wrap `TableRecordEditorPanel` usage (line ~322) in `<div className="cadastro-card">` only if the editor panel has no own surface; otherwise add the class to its wrapper div. Verify visually first — the editor already has `.table-record-editor` styles; if it has its own border, skip to avoid a double card.
- Remove the now-redundant `display:flex; gap` declarations from `.cadastro-catalog, .cadastro-panel, .cadastro-manual, .cadastro-owner` (lines 20-30) where `.cadastro-card` now provides them — keep the classes as hooks for inner spacing only.

**Step 3: Verify** — Run: `pnpm typecheck:web && pnpm test:web`. Manual: dev server, each card has border + shadow + inner breathing room; at 375px padding shrinks; no double borders around the record editor.

**Step 4: Commit**

```bash
git add apps/web/src/cadastro.css apps/web/src/CadastroPage.tsx apps/web/src/lib/cadastro/
git commit -m "feat(cadastro): card surface treatment for work blocks"
```

---

### Task 4: Numbered `CadastroSteps` trail (fix R2, R3, V2)

**Objective:** One visible trail — Tipo → Método → Dono → Registro — with numbered markers; clickable completed steps replace the floating "Trocar tipo" button.

**Files:**

- Create: `apps/web/src/lib/cadastro/CadastroSteps.tsx`
- Create: `apps/web/src/lib/cadastro/CadastroSteps.test.tsx`
- Modify: `apps/web/src/cadastro.css`
- Modify: `apps/web/src/CadastroPage.tsx`

**Step 1: Write failing test**

```tsx
// apps/web/src/lib/cadastro/CadastroSteps.test.tsx
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { CadastroSteps } from "./CadastroSteps";

const steps = [
  { key: "type", label: "Tipo" },
  { key: "owner", label: "Dono" },
  { key: "record", label: "Registro" },
];

it("marks the current step with aria-current", () => {
  render(<CadastroSteps current="owner" onStepClick={vi.fn()} steps={steps} />);
  expect(screen.getByRole("listitem", { name: /Dono/ })).toHaveAttribute("aria-current", "step");
});

it("clicking a completed step calls onStepClick", async () => {
  const onStepClick = vi.fn();
  render(<CadastroSteps current="owner" onStepClick={onStepClick} steps={steps} />);
  await userEvent.click(screen.getByRole("button", { name: /Tipo/ }));
  expect(onStepClick).toHaveBeenCalledWith("type");
});
```

**Step 2: Run to verify failure**

Run: `pnpm --filter @pherlsz/gymkhana-database-web exec vitest run src/lib/cadastro/CadastroSteps.test.tsx`
Expected: FAIL — module not found.

**Step 3: Implement**

```tsx
// apps/web/src/lib/cadastro/CadastroSteps.tsx
import { Check } from "lucide-react";
import { ICON, ICON_STROKE } from "../../components/icons";

export type CadastroStep = { key: string; label: string };

export function CadastroSteps({
  current,
  onStepClick,
  steps,
}: {
  current: string;
  onStepClick: (key: string) => void;
  steps: CadastroStep[];
}) {
  const currentIndex = steps.findIndex((step) => step.key === current);
  return (
    <ol className="cadastro-steps">
      {steps.map((step, index) => {
        const done = index < currentIndex;
        const active = index === currentIndex;
        return (
          <li
            aria-current={active ? "step" : undefined}
            className="cadastro-steps__item"
            key={step.key}
          >
            {done ? (
              <button
                className="cadastro-steps__link cadastro-steps__link--done"
                onClick={() => onStepClick(step.key)}
                type="button"
              >
                <span className="cadastro-steps__marker">
                  <Check aria-hidden size={ICON.badge} strokeWidth={ICON_STROKE} />
                </span>
                {step.label}
              </button>
            ) : (
              <span
                className={
                  active
                    ? "cadastro-steps__link cadastro-steps__link--active"
                    : "cadastro-steps__link"
                }
              >
                <span className="cadastro-steps__marker">{index + 1}</span>
                {step.label}
              </span>
            )}
          </li>
        );
      })}
    </ol>
  );
}
```

CSS (append to `cadastro.css`; ink/paper only — marker numbers in ink, active marker gets the `--md-primary-text` ring, never a gold fill):

```css
.cadastro-steps {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--gym-space-3);
  list-style: none;
  margin: 0;
  padding: 0;
}
.cadastro-steps__item {
  display: flex;
  align-items: center;
  gap: var(--gym-space-3);
}
/* hairline connector */
.cadastro-steps__item + .cadastro-steps__item::before {
  content: "";
  width: 1.25rem;
  height: 1px;
  background: var(--md-outline-variant);
}
.cadastro-steps__link {
  display: inline-flex;
  align-items: center;
  gap: var(--gym-space-2);
  font: inherit;
  font-size: 0.875rem;
  color: var(--md-on-surface-variant);
}
.cadastro-steps__marker {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  border: 1px solid var(--md-outline-variant);
  border-radius: var(--gym-radius-pill);
  background: var(--md-surface-low);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
}
.cadastro-steps__link--active {
  color: var(--md-on-surface);
  font-weight: 600;
}
.cadastro-steps__link--active .cadastro-steps__marker {
  border-color: var(--md-primary-text);
  color: var(--md-primary-text);
  background: var(--md-surface);
}
.cadastro-steps__link--done {
  background: transparent;
  border: 0;
  cursor: pointer;
  padding: 0;
}
.cadastro-steps__link--done .cadastro-steps__marker {
  border-color: var(--md-outline);
  background: var(--md-state-selected);
  color: var(--md-on-surface);
}
.cadastro-steps__link--done:hover {
  color: var(--md-on-surface);
}
.cadastro-steps__link--done:hover .cadastro-steps__marker {
  background: var(--md-state-pressed);
}
```

**Step 4: Run test to verify pass** — same vitest command. Expected: PASS.

**Step 5: Integrate in `CadastroPage.tsx`**

```tsx
const steps =
  search.table === "people"
    ? [
        { key: "method", label: copy.stepsMethod },
        { key: "record", label: copy.stepsRecord },
      ]
    : [
        { key: "type", label: copy.stepsType },
        { key: "method", label: copy.stepsMethod },
        { key: "owner", label: copy.stepsOwner },
        { key: "record", label: copy.stepsRecord },
      ];

const currentStep =
  work === "catalog"
    ? "type"
    : search.table === "people"
      ? "record"
      : search.owner || search.import || search.record
        ? "record"
        : work === "manual" && !search.owner
          ? "owner"
          : "method";
```

Render `<CadastroSteps current={currentStep} onStepClick={onStepClick} steps={steps} />` between `PageHeader` and the work blocks.

```tsx
const onStepClick = (key: string) => {
  if (key === "type") goToCatalog();
  else if (key === "method") patchSearch({ owner: undefined, record: undefined });
  else if (key === "owner") patchSearch({ record: undefined });
};
```

**Delete** the "Trocar tipo" `Button` (lines 219-223) — the trail's clickable "Tipo" step replaces it.

**Step 6: Verify** — Run: `pnpm typecheck:web && pnpm test:web`. Manual: `/cadastro?table=documents` → marker 1 active; pick type → marker 1 shows check, marker 2 active; picking manual advances to Dono; clicking "Tipo" returns to catalog.

**Step 7: Commit**

```bash
git add apps/web/src/lib/cadastro/CadastroSteps.tsx apps/web/src/lib/cadastro/CadastroSteps.test.tsx apps/web/src/cadastro.css apps/web/src/CadastroPage.tsx
git commit -m "feat(cadastro): numbered step trail replaces change-type button"
```

---

### Task 5: Header breathing room + inline disabled hints (fix V2, R4)

**Objective:** Calmer header, and disabled OCR/Forms reasons shown as a one-line hint under the method tabs.

**Files:** Modify `apps/web/src/CadastroPage.tsx`, `apps/web/src/cadastro.css`.

**Step 1: Header spacing**

In `cadastro.css` update `.cadastro-page__tools`:

```css
.cadastro-page__tools {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: var(--gym-space-3);
  row-gap: var(--gym-space-2);
}
```

**Step 2: Inline hint**

Directly under `PageHeader`, when `work !== "catalog"`:

```tsx
{
  work !== "catalog" && ocrBlocked && search.table !== "people" ? (
    <p className="cadastro-catalog__hint">{ocrReason}</p>
  ) : null;
}
{
  work !== "catalog" && !canUseForms ? (
    <p className="cadastro-catalog__hint">{copy.formsDisabled}</p>
  ) : null;
}
```

Keep the `title` tooltips as well. (Render only the hint for the method the user is looking at if both conditions overlap — check `work === "ocr"` / `"forms"` to pick.)

**Step 3: Verify** — Run: `pnpm typecheck:web && pnpm test:web`. Manual: non-admin sees Forms reason without hovering; OCR-blocked env shows the OCR reason.

**Step 4: Commit** — `git add apps/web/src/CadastroPage.tsx apps/web/src/cadastro.css && git commit -m "fix(cadastro): header spacing and inline disabled-method hints"`

---

### Task 6: Type-picker polish (fix V1 follow-through)

**Objective:** The catalog cards should match the new surface language.

**Files:** Modify `apps/web/src/cadastro.css`.

**Step 1:** The `.home-catalog__group` boxes inside cadastro already carry border + radius (`shell.css:1588`). Add only what's missing — softer row hover already exists; verify no double background:

```css
/* Catalog groups sit inside a card: drop their own background so they read
   as sections of the card, not cards inside a card. */
.cadastro-catalog .home-catalog__group {
  background: var(--md-surface);
}
```

**Step 2:** Apply `cadastro-card` to the catalog wrapper if not done in Task 3 (`CadastroTypePicker` root).

**Step 3: Verify** — dev server: type picker renders as sections within one card; hover states unchanged.

**Step 4: Commit** — `git add apps/web/src/cadastro.css && git commit -m "style(cadastro): soften catalog groups inside cards"`

---

### Task 7: Cleanup (fix R5, R6)

**Objective:** Remove dead code and wrong copy reuse.

**Files:** Modify `apps/web/src/lib/cadastro/CadastroPanel.tsx` (drop the `manual + people` null branch, lines 42-44), `apps/web/src/lib/cadastro/CadastroTypePicker.tsx` (line 125: `copy.typesLoadingHint` instead of `copy.ocrCheckingHint`).

**Step 1:** Edit both files as described.

**Step 2: Verify** — Run: `pnpm typecheck:web && pnpm test:web`. Existing tests stay green: `cadastroSearch.test.ts`, `cadastroPicker.test.ts`, `CadastroManualPanel.test.tsx`.

**Step 3: Commit** — `git add apps/web/src/lib/cadastro/ && git commit -m "chore(cadastro): drop dead branch, fix loading copy"`

---

### Task 8: Visual QA pass

**Step 1:** `pnpm dev` (web workspace), open `/cadastro`.

**Step 2:** Walk every path at 375 / 768 / 1024px:

- people → manual create → toast visible → lands on profile.
- documents → catalog card → type → manual → owner card → record card; trail numbers/checks/back-clicks all correct.
- xlsx wizard; OCR when enabled; forms admin vs non-admin (inline hint).
- Check: no horizontal scroll at 375px; controls in the header row share identical heights (Pedro's rule); focus rings visible on markers and done-step buttons; **no gold fill anywhere on controls**; only `--md-primary-text` as the active marker accent; cards don't double up around `TableRecordEditorPanel`.

**Step 3:** Run: `pnpm check:web` — Expected: typecheck + lint + format + tests + build green.

**Step 4:** Format-fix commit only if needed: `git add -A && git commit -m "style(cadastro): format pass"`

---

## Files likely to change (summary)

| File                                                  | Change                                                                       |
| ----------------------------------------------------- | ---------------------------------------------------------------------------- |
| `apps/web/src/CadastroPage.tsx`                       | feedback wiring, steps integration, delete change-type button, hints         |
| `apps/web/src/cadastro.css`                           | `.cadastro-card`, `.cadastro-steps`, header spacing, catalog group softening |
| `apps/web/src/lib/cadastro/CadastroSteps.tsx`         | new                                                                          |
| `apps/web/src/lib/cadastro/CadastroSteps.test.tsx`    | new                                                                          |
| `apps/web/src/lib/cadastro/cadastroFeedback.ts`       | new                                                                          |
| `apps/web/src/lib/cadastro/cadastroFeedback.test.tsx` | new                                                                          |
| `apps/web/src/lib/cadastro/CadastroTypePicker.tsx`    | card class + loading copy                                                    |
| `apps/web/src/lib/cadastro/CadastroManualPanel.tsx`   | card class                                                                   |
| `apps/web/src/lib/cadastro/CadastroOwnerPicker.tsx`   | card class                                                                   |
| `apps/web/src/lib/cadastro/CadastroPanel.tsx`         | dead branch removal                                                          |
| `apps/web/src/i18n/v1/*`                              | new keys                                                                     |

## Validation

- `pnpm check:web`
- Manual walkthrough: 5 work modes × 3 tables at 375/768/1024px
- Existing tests stay green: `cadastroSearch.test.ts`, `cadastroPicker.test.ts`, `CadastroManualPanel.test.tsx`, `App.test.tsx`

## Risks, tradeoffs, open questions

- **Double-card risk** around `TableRecordEditorPanel`/`ProfilePanel` — they may already carry surfaces; Task 3 Step 2 says verify visually before wrapping. When in doubt, don't wrap.
- **antd static `message`** warns without an `<App>` wrapper — Task 1 Step 1 checks first.
- **Trail derivation duplicates state logic** implicit in the render tree — one `useMemo`, no new abstraction (YAGNI).
- **i18n version bump**: adding keys to v1 is additive; if `I18N_CATALOG_VERSION` policy requires a bump, follow it (Task 2).
- **Open question:** the people flow only gets Método → Registro in the trail; drop it entirely if Pedro prefers people to stay trail-less.
- **Not in scope (YAGNI):** OCR review internals, import wizard internals, Google Forms submodule UI.
