import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { normalizeMatchingSearch } from "./MatchingPage";

const caseID = "22222222-2222-4222-8222-222222222222";
const leftID = "33333333-3333-4333-8333-333333333333";
const rightID = "44444444-4444-4444-8444-444444444444";
const analysisID = "11111111-1111-4111-8111-111111111111";
const cancellationAnalysisID = "55555555-5555-4555-8555-555555555555";
const now = "2026-07-17T21:00:00Z";

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "content-type": "application/json", "x-request-id": "matching-test-request" },
  });
}

function session(role: "MEMBER" | "ADMIN") {
  return {
    authenticated: true,
    user: { login: "reviewer", display_name: "Revisor", role },
  };
}

function catalog(canMerge: boolean) {
  return {
    evidence: [
      { kind: "EMAIL_EXACT", label: "E-mail igual" },
      { kind: "NAME_SIMILAR", label: "Nome semelhante" },
    ],
    can_merge: canMerge,
    maximum_candidates: 2_000,
    maximum_page_size: 100,
    maximum_analyses_per_window: 5,
    analysis_window_seconds: 3_600,
  };
}

function matchingCase(state = "PENDING") {
  return {
    id: caseID,
    left_profile_id: leftID,
    right_profile_id: rightID,
    left_profile_version: 2,
    right_profile_version: 3,
    score: 95,
    score_band: "HIGH",
    state,
    evidence: [
      { kind: "EMAIL_EXACT", strength: 100, contribution: 30 },
      { kind: "NAME_SIMILAR", strength: 92, contribution: 20 },
    ],
    left: {
      id: leftID,
      full_name: "Ana Silva",
      email: "ana@example.org",
      address_city: "Recife",
      address_state: "PE",
      version: 2,
      updated_at: now,
    },
    right: {
      id: rightID,
      full_name: "Ana da Silva",
      email: "ana@example.org",
      address_city: "Recife",
      address_state: "PE",
      version: 3,
      updated_at: now,
    },
    version: 4,
    created_at: now,
    updated_at: now,
  };
}

function analysis(state: "QUEUED" | "COMPLETED" | "CANCELLED", id: string = analysisID) {
  return {
    id,
    state,
    profiles_scanned: state === "COMPLETED" ? 12 : 0,
    candidate_count: state === "COMPLETED" ? 1 : 0,
    refreshed_count: 0,
    expires_at: "2026-07-18T21:00:00Z",
    ...(state === "CANCELLED" ? { completed_at: now, error_code: "cancelled" } : {}),
    version: state === "QUEUED" ? 1 : 2,
    created_at: now,
    updated_at: now,
  };
}

const dependencies = [
  { kind: "DOCUMENT_OWNER", count: 1 },
  { kind: "DOCUMENT_HOLDER", count: 0 },
  { kind: "BILL_OWNER", count: 2 },
  { kind: "BILL_HOLDER", count: 0 },
  { kind: "CUSTOM_ENTITY_OWNER", count: 0 },
  { kind: "CUSTOM_PROFILE_VALUE", count: 1 },
  { kind: "ATTACHMENT_INTENT", count: 0 },
  { kind: "ATTACHMENT", count: 1 },
];

function mergePreview(resolved: boolean) {
  const value = matchingCase();
  return {
    case_id: caseID,
    survivor: value.left,
    source: value.right,
    fields: [
      {
        key: "full_name",
        label: "Nome completo",
        kind: "TEXT",
        survivor_value: "Ana Silva",
        source_value: "Ana da Silva",
        conflict: true,
        choice_required: true,
        ...(resolved ? { selected_source: "SOURCE" } : {}),
      },
    ],
    dependencies,
    conflicts: [],
    unresolved_field_count: resolved ? 0 : 1,
    preview_fingerprint: "a".repeat(64),
    confirmation: "MESCLAR Ana Silva",
    generated_at: now,
  };
}

describe("MatchingPage", () => {
  beforeEach(() => {
    window.history.replaceState(
      null,
      "",
      "/matching?matching_state=PENDING&matching_band=HIGH&matching_page=1",
    );
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("lets a member run, explain and dismiss a candidate without exposing personal values in the URL", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(session("MEMBER")));
      if (url.endsWith("/api/v1/matching/catalog"))
        return Promise.resolve(jsonResponse(catalog(false)));
      if (url.includes("/api/v1/matching/cases?") && (!init?.method || init.method === "GET")) {
        return Promise.resolve(
          jsonResponse({ cases: [matchingCase()], total: 1, limit: 25, offset: 0 }),
        );
      }
      if (
        url.endsWith(`/api/v1/matching/cases/${caseID}`) &&
        (!init?.method || init.method === "GET")
      ) {
        return Promise.resolve(jsonResponse(matchingCase()));
      }
      if (url.endsWith(`/api/v1/matching/cases/${caseID}/dismiss`) && init?.method === "POST") {
        return Promise.resolve(jsonResponse(matchingCase("NOT_DUPLICATE")));
      }
      if (url.endsWith("/api/v1/matching/analyses") && init?.method === "POST") {
        return Promise.resolve(jsonResponse(analysis("QUEUED"), 202));
      }
      if (url.endsWith(`/api/v1/matching/analyses/${analysisID}`)) {
        return Promise.resolve(jsonResponse(analysis("COMPLETED")));
      }
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "Revisão de possíveis duplicidades" }),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Ordenar casos por"), {
      target: { value: "updated_at" },
    });
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).get("matching_sort")).toBe("updated_at"),
    );
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining("sort=updated_at&order=desc"),
        expect.anything(),
      ),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Revisar" }));
    expect(await screen.findByRole("table", { name: "Comparação dos perfis" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Comparação detalhada" })).toHaveFocus();
    expect(screen.getByText("E-mail igual")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Não são duplicados" })).toBeInTheDocument();
    expect(screen.queryByText("Mesclagem administrativa")).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Abrir cadastro" })).toHaveLength(2);
    expect(window.location.search).not.toContain("Ana");
    expect(window.location.search).not.toContain("example.org");

    fireEvent.click(screen.getByRole("button", { name: "Analisar perfis" }));
    expect(await screen.findByText("Concluída")).toBeInTheDocument();
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/matching/analyses"),
        expect.objectContaining({ method: "POST" }),
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "Não são duplicados" }));
    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        expect.stringContaining(`/api/v1/matching/cases/${caseID}/dismiss`),
        expect.objectContaining({ method: "POST", body: JSON.stringify({ version: 4 }) }),
      ),
    );
    await waitFor(() =>
      expect(new URLSearchParams(window.location.search).has("matching_case")).toBe(false),
    );
  });

  it("cancels an active analysis and keeps the empty queue usable", async () => {
    window.history.replaceState(
      null,
      "",
      "/matching?matching_state=PENDING&matching_band=LOW&matching_page=1",
    );
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session"))
        return Promise.resolve(jsonResponse(session("MEMBER")));
      if (url.endsWith("/api/v1/matching/catalog"))
        return Promise.resolve(jsonResponse(catalog(false)));
      if (url.includes("/api/v1/matching/cases?"))
        return Promise.resolve(jsonResponse({ cases: [], total: 0, limit: 25, offset: 0 }));
      if (url.endsWith("/api/v1/matching/analyses") && init?.method === "POST")
        return Promise.resolve(jsonResponse(analysis("QUEUED", cancellationAnalysisID), 202));
      if (
        url.endsWith(`/api/v1/matching/analyses/${cancellationAnalysisID}/cancel`) &&
        init?.method === "POST"
      )
        return Promise.resolve(jsonResponse(analysis("CANCELLED", cancellationAnalysisID)));
      if (url.endsWith(`/api/v1/matching/analyses/${cancellationAnalysisID}`))
        return Promise.resolve(jsonResponse(analysis("QUEUED", cancellationAnalysisID)));
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Analisar perfis" }));
    fireEvent.click(await screen.findByRole("button", { name: "Cancelar" }));
    expect(await screen.findByText("Cancelada")).toBeInTheDocument();
    expect(screen.getByText("Nenhum caso nesta seleção")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(`/api/v1/matching/analyses/${cancellationAnalysisID}/cancel`),
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("requires an admin to resolve fields, refresh the preview and type the exact merge confirmation", async () => {
    const previewBodies: unknown[] = [];
    let mergeBody: Record<string, unknown> | undefined;
    const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.endsWith("/api/auth/session")) return Promise.resolve(jsonResponse(session("ADMIN")));
      if (url.endsWith("/api/v1/matching/catalog"))
        return Promise.resolve(jsonResponse(catalog(true)));
      if (url.includes("/api/v1/matching/cases?") && (!init?.method || init.method === "GET")) {
        return Promise.resolve(
          jsonResponse({ cases: [matchingCase()], total: 1, limit: 25, offset: 0 }),
        );
      }
      if (
        url.endsWith(`/api/v1/matching/cases/${caseID}`) &&
        (!init?.method || init.method === "GET")
      ) {
        return Promise.resolve(jsonResponse(matchingCase()));
      }
      if (
        url.endsWith(`/api/v1/matching/cases/${caseID}/merge-preview`) &&
        init?.method === "POST"
      ) {
        const body = JSON.parse(String(init.body)) as { choices: unknown[] };
        previewBodies.push(body);
        return Promise.resolve(jsonResponse(mergePreview(body.choices.length === 1)));
      }
      if (url.endsWith(`/api/v1/matching/cases/${caseID}/merge`) && init?.method === "POST") {
        mergeBody = JSON.parse(String(init.body)) as Record<string, unknown>;
        return Promise.resolve(
          jsonResponse({
            case_id: caseID,
            survivor_profile_id: leftID,
            source_profile_id: rightID,
            survivor_version: 3,
            moved_dependencies: dependencies,
            merged_at: now,
          }),
        );
      }
      return Promise.resolve(jsonResponse({ status: "ok" }));
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Revisar" }));
    expect(await screen.findByText("Mesclagem administrativa")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Gerar prévia da mesclagem" }));
    const fieldChoice = await screen.findByLabelText("Valor preservado para Nome completo");
    expect(screen.getByText("Escolhas obrigatórias pendentes")).toBeInTheDocument();
    fireEvent.change(fieldChoice, { target: { value: "SOURCE" } });
    expect(screen.getByText("Prévia desatualizada")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Atualizar prévia" }));

    const confirmation = await screen.findByLabelText("Confirmação da mesclagem");
    const mergeButton = screen.getByRole("button", { name: "Mesclar perfis definitivamente" });
    expect(mergeButton).toBeDisabled();
    fireEvent.change(confirmation, { target: { value: "MESCLAR Ana Silva" } });
    expect(mergeButton).toBeEnabled();
    fireEvent.click(mergeButton);

    expect(await screen.findByText("Perfis mesclados")).toBeInTheDocument();
    expect(previewBodies).toHaveLength(2);
    expect(previewBodies[1]).toMatchObject({
      survivor_profile_id: leftID,
      source_profile_id: rightID,
      choices: [{ field_key: "full_name", source: "SOURCE" }],
    });
    expect(mergeBody).toMatchObject({
      survivor_profile_id: leftID,
      source_profile_id: rightID,
      choices: [{ field_key: "full_name", source: "SOURCE" }],
      preview_fingerprint: "a".repeat(64),
      confirmation: "MESCLAR Ana Silva",
    });
    expect(String(mergeBody?.idempotency_key)).toMatch(/^matching:merge:/);
  });

  it("normalizes only bounded filters and opaque case identifiers", () => {
    expect(
      normalizeMatchingSearch({
        matching_state: "DROP TABLE",
        matching_band: "EXTREME",
        matching_page: -4,
        matching_case: "Ana Silva",
        cpf: "52998224725",
      }),
    ).toEqual({
      matching_state: "PENDING",
      matching_band: "",
      matching_page: 1,
      matching_sort: "score",
      matching_order: "desc",
    });
    expect(
      normalizeMatchingSearch({
        matching_state: "MERGED",
        matching_band: "HIGH",
        matching_page: "2",
        matching_sort: "updated_at",
        matching_order: "asc",
        matching_case: caseID,
      }),
    ).toEqual({
      matching_state: "MERGED",
      matching_band: "HIGH",
      matching_page: 2,
      matching_sort: "updated_at",
      matching_order: "asc",
      matching_case: caseID,
    });
  });
});
