# Cadastro page

Overrides `design-system/gymkhana-cadastro/MASTER.md`. Product tokens stay in `docs/FRONTEND.md` (ink/paper, Geist/Oswald, Lucide, Ant Design). Do not apply the MASTER blue/orange palette on this screen.

## Hierarchy

This is a form page, not a spreadsheet and not a custom workspace. Use `.page-measure` and `PageHeader` like Admin / Search. The document scrolls. Do not fill `100dvh` the way Tables does.

1. `PageHeader` title **Cadastro** (Oswald). Actions: Segmented Pessoas / Documentos / Contas, then methods once a type is in play, plus **Trocar tipo**.
2. Pessoas opens the person form immediately (`ProfilePanel` embedded — no “Nova pessoa / Fechar”).
3. Documentos / Contas: the same grouped catalog rows as Home (`.home-catalog`), then owner + form.
4. Person select is `SearchField` `lookup` `mode="suggest"` — the same combobox as “Pessoa em uso”. Selected = name + **Trocar**. Do not list people as full-width buttons.

Do not reuse the tables inspector 30rem width on this page. Do not invent a second tile system or a framed “cadastro chrome”.

## Google Forms

Google Forms is a **submodule of Cadastro**, not a destination. URL: `/cadastro?mode=forms`. `/forms` only redirects here. Sidebar has Cadastro only.
