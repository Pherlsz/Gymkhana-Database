import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { decodeQueryURLPlan } from "./lib/queryState";

const executionID = "11111111-1111-4111-8111-111111111111";
const catalogVersion = "a".repeat(64);

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function queryCatalog() {
  return {
    version: catalogVersion,
    entities: [
      {
        key: "documents",
        label: "Documentos",
        kind: "document",
        navigable: true,
        default_sort: "document.identifier",
      },
      {
        key: "profiles",
        label: "Pessoas",
        kind: "profile",
        navigable: true,
        default_sort: "profile.full_name",
      },
    ],
    fields: [
      {
        key: "document.identifier",
        entity: "documents",
        label: "Identificador",
        kind: "identifier",
        nullable: false,
        projectable: true,
        filterable: true,
        sortable: true,
        operators: ["contains", "eq"],
      },
      {
        key: "profile.full_name",
        entity: "profiles",
        label: "Nome completo",
        kind: "text",
        nullable: false,
        projectable: true,
        filterable: true,
        sortable: true,
        operators: ["contains", "eq"],
      },
      {
        key: "profile.address_city",
        entity: "profiles",
        label: "Cidade",
        kind: "text",
        nullable: true,
        projectable: true,
        filterable: true,
        sortable: true,
        operators: ["eq", "in", "is_null"],
      },
    ],
    relations: [
      {
        key: "profile.documents",
        from_entity: "profiles",
        to_entity: "documents",
        label: "Documentos da pessoa",
        cardinality: "MANY",
      },
    ],
    operators: [
      { key: "eq", label: "é igual a", minimum_values: 1, maximum_values: 1 },
      { key: "contains", label: "contém", minimum_values: 1, maximum_values: 1 },
      { key: "in", label: "está em", minimum_values: 1, maximum_values: 25 },
      { key: "is_null", label: "está vazio", minimum_values: 0, maximum_values: 0 },
    ],
    limits: {
      maximum_projections: 20,
      maximum_filter_nodes: 40,
      maximum_filter_depth: 6,
      maximum_relation_depth: 3,
      maximum_predicate_values: 25,
      maximum_sort_fields: 3,
      maximum_rows: 500,
      maximum_page_size: 100,
    },
  };
}

function completedExecution() {
  return {
    id: executionID,
    state: "COMPLETED",
    catalog_version: catalogVersion,
    root_entity: "profiles",
    maximum_rows: 100,
    row_count: 1,
    column_count: 2,
    started_at: "2026-07-17T18:00:00Z",
    completed_at: "2026-07-17T18:00:01Z",
    expires_at: "2026-07-17T19:00:00Z",
    version: 2,
  };
}

describe("QueryPage", () => {
  beforeEach(() => window.history.replaceState(null, "", "/query"));
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("builds nested logical plans, validates, executes, and renders typed results", async () => {
    const submittedPlans: unknown[] = [];
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "member", display_name: "Member", role: "EXTERNAL" },
          }),
        );
      if (url.endsWith("/api/v1/query/catalog"))
        return Promise.resolve(jsonResponse(queryCatalog()));
      if (url.endsWith("/api/v1/query/validate") && init?.method === "POST") {
        submittedPlans.push(JSON.parse(String(init.body)));
        return Promise.resolve(
          jsonResponse({
            valid: true,
            fingerprint: "b".repeat(64),
            cost: 180,
            columns: [
              { position: 0, field_key: "profile.full_name", label: "Nome completo", kind: "text" },
              { position: 1, field_key: "profile.address_city", label: "Cidade", kind: "text" },
            ],
          }),
        );
      }
      if (url.endsWith("/api/v1/query/executions") && init?.method === "POST") {
        const request = JSON.parse(String(init.body)) as { idempotency_key: string; plan: unknown };
        expect(request.idempotency_key).toMatch(/^query-/);
        submittedPlans.push(request.plan);
        return Promise.resolve(jsonResponse(completedExecution()));
      }
      if (url.includes(`/api/v1/query/executions/${executionID}/result?`))
        return Promise.resolve(
          jsonResponse({
            execution: completedExecution(),
            columns: [
              { position: 0, field_key: "profile.full_name", label: "Nome completo", kind: "text" },
              { position: 1, field_key: "profile.address_city", label: "Cidade", kind: "text" },
            ],
            rows: [
              {
                position: 0,
                entity_kind: "profile",
                entity_id: executionID,
                entity_label: "Ana Query",
                updated_at: "2026-07-17T18:00:00Z",
                cells: [
                  { column_position: 0, kind: "text", is_null: false, text_value: "Ana Query" },
                  { column_position: 1, kind: "text", is_null: false, text_value: "Recife" },
                ],
              },
            ],
            total: 1,
            limit: 100,
            offset: 0,
          }),
        );
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "Construtor visual de consultas" }),
    ).toBeInTheDocument();
    fireEvent.change(await screen.findByLabelText("Entidade raiz"), {
      target: { value: "profiles" },
    });
    await waitFor(() => expect(screen.getByText("Nome completo")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Adicionar condição" }));
    fireEvent.change(screen.getByLabelText("Campo da condição"), {
      target: { value: "profile.address_city" },
    });
    fireEvent.change(screen.getByLabelText("Cidade valor 1"), { target: { value: "Recife" } });

    fireEvent.click(screen.getByRole("button", { name: "Adicionar relação" }));
    const relationEditor = screen
      .getByLabelText("Relação do filtro")
      .closest(".query-filter-special");
    expect(relationEditor).not.toBeNull();
    fireEvent.click(
      within(relationEditor as HTMLElement).getByRole("button", { name: "Adicionar condição" }),
    );
    fireEvent.change(screen.getByLabelText("Identificador valor 1"), {
      target: { value: "%_literal" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Validar plano" }));
    expect(await screen.findByText(/Custo estimado: 180/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Executar consulta" }));

    const table = await screen.findByRole("table", { name: "Resultado da consulta" });
    expect(within(table).getAllByText("Ana Query")).not.toHaveLength(0);
    expect(within(table).getByText("Recife")).toBeInTheDocument();
    expect(within(table).getByRole("link", { name: "Ana Query" })).toHaveAttribute(
      "href",
      expect.stringContaining("/profiles"),
    );

    expect(submittedPlans).toHaveLength(2);
    const plan = submittedPlans[0] as {
      root_entity: string;
      filter: { kind: string; children: Array<{ kind: string; relation?: string }> };
    };
    expect(plan.root_entity).toBe("profiles");
    expect(plan.filter.kind).toBe("group");
    expect(plan.filter.children.some((node) => node.relation === "profile.documents")).toBe(true);
    expect(JSON.stringify(plan)).not.toMatch(/SELECT|FROM profiles|physical_table|sql/i);
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get("plan")).toBeTruthy(),
    );
    const urlPlan = decodeQueryURLPlan(
      new URLSearchParams(window.location.search).get("plan") ?? undefined,
    )!;
    expect(urlPlan.root_entity).toBe("profiles");
    expect(urlPlan.filter!.children!.some((node) => node.relation === "profile.documents")).toBe(
      true,
    );
    expect(window.localStorage).toHaveLength(0);
    expect(window.sessionStorage).toHaveLength(0);
  });

  it("explains stale catalogs without exposing server details", async () => {
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "member", display_name: "Member", role: "EXTERNAL" },
          }),
        );
      if (url.endsWith("/api/v1/query/catalog"))
        return Promise.resolve(jsonResponse(queryCatalog()));
      if (url.endsWith("/api/v1/query/validate") && init?.method === "POST")
        return Promise.resolve(
          jsonResponse(
            {
              error: {
                code: "query_catalog_stale",
                message: "internal catalog hash mismatch SELECT secret",
              },
            },
            409,
          ),
        );
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);
    await screen.findByRole("heading", { name: "Construtor visual de consultas" });
    fireEvent.change(await screen.findByLabelText("Entidade raiz"), {
      target: { value: "profiles" },
    });
    fireEvent.click(await screen.findByRole("button", { name: "Validar plano" }));
    expect(
      await screen.findByText("O catálogo mudou. Recarregue a página e revise o plano."),
    ).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent("SELECT secret");
  });

  it("refetches a stale catalog, preserves permitted nodes, and identifies removed nodes", async () => {
    let staleRequested = false;
    const updatedCatalog = queryCatalog();
    updatedCatalog.version = "c".repeat(64);
    updatedCatalog.fields = updatedCatalog.fields.filter(
      (field) => field.key !== "profile.address_city",
    );
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(
          jsonResponse({
            authenticated: true,
            user: { login: "member", display_name: "Member", role: "EXTERNAL" },
          }),
        );
      if (url.endsWith("/api/v1/query/catalog"))
        return Promise.resolve(jsonResponse(staleRequested ? updatedCatalog : queryCatalog()));
      if (url.endsWith("/api/v1/query/validate") && init?.method === "POST") {
        staleRequested = true;
        return Promise.resolve(
          jsonResponse(
            { error: { code: "query_catalog_stale", message: "internal stale details" } },
            409,
          ),
        );
      }
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    await screen.findByRole("heading", { name: "Construtor visual de consultas" });
    fireEvent.click(await screen.findByRole("button", { name: "Adicionar condição" }));
    fireEvent.change(screen.getByLabelText("Campo da condição"), {
      target: { value: "profile.address_city" },
    });
    fireEvent.change(screen.getByLabelText("Cidade valor 1"), { target: { value: "Recife" } });
    fireEvent.click(screen.getByRole("button", { name: "Validar plano" }));

    expect(
      await screen.findByText(
        /Os itens ainda permitidos foram preservados.*filtro.*versão do catálogo/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/Sem filtros/)).toBeInTheDocument();
    await waitFor(() => {
      const encoded = new URLSearchParams(window.location.search).get("plan");
      const recovered = decodeQueryURLPlan(encoded ?? undefined);
      expect(recovered!.catalog_version).toBe(updatedCatalog.version);
      expect(recovered!.filter).toBeUndefined();
    });
  });

  it("drops a forged URL plan before it reaches the builder", async () => {
    const forged = JSON.stringify({
      version: "v1",
      catalog_version: catalogVersion,
      root_entity: "profiles",
      projections: ["profile.full_name"],
      maximum_rows: 100,
      sql: "SELECT secret FROM profiles",
    });
    window.history.replaceState(null, "", `/query?plan=${encodeURIComponent(forged)}`);
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.endsWith("/api/auth/session"))
          return Promise.resolve(
            jsonResponse({
              authenticated: true,
              user: { login: "member", display_name: "Member", role: "EXTERNAL" },
            }),
          );
        if (url.endsWith("/api/v1/query/catalog"))
          return Promise.resolve(jsonResponse(queryCatalog()));
        return Promise.resolve(jsonResponse({ status: "ok" }));
      }),
    );
    render(<App />);

    expect(await screen.findByText("Plano da URL ignorado")).toBeInTheDocument();
    expect(document.body).not.toHaveTextContent("SELECT secret");
  });
});
