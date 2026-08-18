# Ant Design is the visual base

The product is table-dense and mobile-first. A from-scratch UI was dropped: Ant Design is the component and chrome layer, including the `/tables/*` sheet (`Table` with `virtual`). Material UI, shadcn, and a private design system are rejected as the visual base. TanStack Table is not the gymkhana sheet; it may remain as a ColumnDef adapter on Search/Operations DataGrid until that stack is unified.

**Considered Options**: keep an in-repo visual system with Lucide and leftover Ant Design until replaced; use TanStack Table + Virtual as the grade engine with Ant Design only for chrome.

**Consequences**: Orchestration §12.3 and the frontend baseline name Ant Design as the principal visual dependency. Wrappers stay in this repository; there is still no shared UI package.
