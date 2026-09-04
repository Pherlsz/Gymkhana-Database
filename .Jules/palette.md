## 2026-03-30 - Form Accessibility and Ant Design Component Standardization

**Learning:** When refactoring raw HTML form elements (`<input type="checkbox">`, `window.confirm`, or custom delete boxes) in data-heavy screens, replacing them with standardized Ant Design components (like `<Checkbox>`) and generated `useId()` pairings for labels drastically improves screen reader compatibility and keyboard focus consistency without altering event handlers (`event.target.checked`).

**Action:** Prefer Ant Design primitives over native inputs in admin/data-heavy panels and ensure destructive dialogs use a unified `ConfirmDelete` component with explicit `htmlFor` and `aria-required="true"`.
