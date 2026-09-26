# Design System Master File

> **LOGIC:** When building a specific page, first check `design-system/pages/[page-name].md`.
> If that file exists, its rules **override** this Master file.
> If not, strictly follow the rules below.

---

**Project:** Gymkhana Cadastro
**Generated:** 2026-08-27 20:24:12
**Category:** General
**Design Dials:** Variance 3/10 (Centered / Minimal) | Motion 4/10 (Standard) | Density 8/10 (Dense / Dashboard)

---

## ⚠️ Visual base override (authoritative)

**Ant Design is the visual base** (Orchestration §12.3, ADR 0002, FRONTEND.md). The component CSS below (`.btn-primary`, `.card`, `.input`, `.modal`, shadows, radii) is reference-only and MUST NOT be shipped into `apps/web`. Do not restyle Ant internals with `.ant-*` selectors; do not rebuild an Ant control in raw HTML.

Where this file conflicts with `docs/FRONTEND.md`, FRONTEND.md wins:

- **Colors:** use the ink/paper tokens in `apps/web/src/shell.css` (`--md-*`), mapped through `ConfigProvider` (`cssVar`, `hashed: false`) in `theme.tsx`. The blue/orange palette below is superseded and applies to nothing in the app.
- **Fonts:** Geist (body/tables/forms/numbers) + Oswald (page titles only). Outfit and Work Sans below are superseded.
- **Icons:** Lucide (`lucide-react`, strokeWidth 1.75); never `@ant-design/icons` in app source.
- **Components:** compose Ant Design (`Table`, `Form`, `Select`, `Steps`, `Modal`, `Drawer`, `Segmented`, overlays) plus module wrappers under `apps/web/src/lib/<area>/`. The tables page keeps antd `Table` with `virtual` — this is why antd is the base.

Everything below the fold survives only as mood/density guidance (Minimalism, density 8/10, anti-patterns, pre-delivery checklist).

---

## Global Rules

### Color Palette

| Role             | Hex       | CSS Variable               |
| ---------------- | --------- | -------------------------- |
| Primary          | `#2563EB` | `--color-primary`          |
| On Primary       | `#FFFFFF` | `--color-on-primary`       |
| Secondary        | `#3B82F6` | `--color-secondary`        |
| On Secondary     | `#000000` | `--color-on-secondary`     |
| Accent/CTA       | `#EA580C` | `--color-accent`           |
| On Accent/CTA    | `#000000` | `--color-on-accent`        |
| Background       | `#F8FAFC` | `--color-background`       |
| Foreground       | `#1E293B` | `--color-foreground`       |
| Card             | `#FFFFFF` | `--color-card`             |
| Card Foreground  | `#1E293B` | `--color-card-foreground`  |
| Muted            | `#E9EFF8` | `--color-muted`            |
| Muted Foreground | `#475569` | `--color-muted-foreground` |
| Border           | `#E2E8F0` | `--color-border`           |
| Destructive      | `#DC2626` | `--color-destructive`      |
| On Destructive   | `#FFFFFF` | `--color-on-destructive`   |
| Ring             | `#2563EB` | `--color-ring`             |

**Color Notes:** Trust blue + orange CTA contrast [Accent adjusted from #F97316]

### Typography

- **Heading Font:** Outfit
- **Body Font:** Work Sans
- **Mood:** geometric, modern, clean, balanced, contemporary, versatile
- **Google Fonts:** [Outfit + Work Sans](https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700&family=Work+Sans:wght@300;400;500;600;700&display=swap)

**CSS Import:**

```css
@import url("https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700&family=Work+Sans:wght@300;400;500;600;700&display=swap");
```

### Spacing Variables

_Density: 8/10 — Dense / Dashboard_

| Token         | Value              | Usage                     |
| ------------- | ------------------ | ------------------------- |
| `--space-xs`  | `2px` / `0.125rem` | Tight gaps                |
| `--space-sm`  | `4px` / `0.25rem`  | Icon gaps, inline spacing |
| `--space-md`  | `8px` / `0.5rem`   | Standard padding          |
| `--space-lg`  | `12px` / `0.75rem` | Section padding           |
| `--space-xl`  | `16px` / `1rem`    | Large gaps                |
| `--space-2xl` | `24px` / `1.5rem`  | Section margins           |
| `--space-3xl` | `32px` / `2rem`    | Hero padding              |

### Shadow Depths

| Level         | Value                          | Usage                       |
| ------------- | ------------------------------ | --------------------------- |
| `--shadow-sm` | `0 1px 2px rgba(0,0,0,0.05)`   | Subtle lift                 |
| `--shadow-md` | `0 4px 6px rgba(0,0,0,0.1)`    | Cards, buttons              |
| `--shadow-lg` | `0 10px 15px rgba(0,0,0,0.1)`  | Modals, dropdowns           |
| `--shadow-xl` | `0 20px 25px rgba(0,0,0,0.15)` | Hero images, featured cards |

---

## Component Specs

### Buttons

```css
/* Primary Button */
.btn-primary {
  background: #ea580c;
  color: white;
  padding: 12px 24px;
  border-radius: 8px;
  font-weight: 600;
  transition: all 200ms ease;
  cursor: pointer;
}

.btn-primary:hover {
  opacity: 0.9;
  transform: translateY(-1px);
}

/* Secondary Button */
.btn-secondary {
  background: transparent;
  color: #2563eb;
  border: 2px solid #2563eb;
  padding: 12px 24px;
  border-radius: 8px;
  font-weight: 600;
  transition: all 200ms ease;
  cursor: pointer;
}
```

### Cards

```css
.card {
  background: #f8fafc;
  border-radius: 12px;
  padding: 24px;
  box-shadow: var(--shadow-md);
  transition: all 200ms ease;
  cursor: pointer;
}

.card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-2px);
}
```

### Inputs

```css
.input {
  padding: 12px 16px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  font-size: 16px;
  transition: border-color 200ms ease;
}

.input:focus {
  border-color: #2563eb;
  outline: none;
  box-shadow: 0 0 0 3px #2563eb20;
}
```

### Modals

```css
.modal-overlay {
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(4px);
}

.modal {
  background: white;
  border-radius: 16px;
  padding: 32px;
  box-shadow: var(--shadow-xl);
  max-width: 500px;
  width: 90%;
}
```

---

## Style Guidelines

**Style:** Minimalism & Swiss Style

**Keywords:** Clean, simple, spacious, functional, white space, high contrast, geometric, sans-serif, grid-based, essential

**Best For:** Enterprise apps, dashboards, documentation sites, SaaS platforms, professional tools

**Key Effects:** Subtle hover (200-250ms), smooth transitions, sharp shadows if any, clear type hierarchy, fast loading

### Page Pattern

**Pattern Name:** Hero + Features + CTA

- **Conversion Strategy:** Deep CTA placement. For CTA label text, verify at least 4.5:1 against the button fill; use 7:1 only when the product explicitly targets AAA normal-text contrast. Keep focus and component boundaries independently visible. Disable hero parallax under reduced motion and render its static final state.
- **CTA Placement:** Hero (sticky) + Bottom
- **Section Order:** Hero with headline/image > Value prop > Key features (3-5) > CTA section > Footer

---

## Motion

**Stagger List** (Standard) — Trigger: load or scroll | Duration: 300-450ms | Easing: `back.out(1.4)`

```js
gsap.from(".grid-item", {
  opacity: 0,
  scale: 0.92,
  y: 16,
  duration: 0.4,
  stagger: { each: 0.06, from: "start", grid: "auto" },
  ease: "back.out(1.4)",
});
```

**Framework notes:** grid: 'auto' lets GSAP infer rows/columns from a CSS grid layout for a natural wave stagger; Use matchMedia('(prefers-reduced-motion: reduce)') to skip non-essential motion and render the final state immediately

- ✅ Combine with from: 'center' for a bento-grid layout to draw the eye inward first
- ❌ Don't use back.out on dense data tables; the overshoot reads as sloppy on informational UI
- ⚡ Group DOM writes; avoid interleaving layout reads (getBoundingClientRect) between staggered tweens

---

## Anti-Patterns (Do NOT Use)

### Additional Forbidden Patterns

- ❌ **Emojis as icons** — Use SVG icons (Heroicons, Lucide, Simple Icons)
- ❌ **Missing cursor:pointer** — All clickable elements must have cursor:pointer
- ❌ **Layout-shifting hovers** — Avoid scale transforms that shift layout
- ❌ **Low contrast text** — Maintain 4.5:1 minimum contrast ratio
- ❌ **Instant state changes** — Always use transitions (150-300ms)
- ❌ **Invisible focus states** — Focus states must be visible for a11y

---

## Pre-Delivery Checklist

Before delivering any UI code, verify:

- [ ] No emojis used as icons (use SVG instead)
- [ ] All icons from consistent icon set (Heroicons/Lucide)
- [ ] `cursor-pointer` on all clickable elements
- [ ] Hover states with smooth transitions (150-300ms)
- [ ] Light mode: text contrast 4.5:1 minimum
- [ ] Focus states visible for keyboard navigation
- [ ] `prefers-reduced-motion` respected
- [ ] Responsive: 375px, 768px, 1024px, 1440px
- [ ] No content hidden behind fixed navbars
- [ ] No horizontal scroll on mobile
