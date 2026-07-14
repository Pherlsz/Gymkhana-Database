import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

function healthResponse(): Response {
  return new Response(JSON.stringify({ status: "ok", request_id: "request-123" }), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

describe("App", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("consumes the released UI foundations and refreshes typed API health", async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(healthResponse()));
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);

    expect(screen.getByRole("heading", { name: "Gymkhana Database" })).toBeInTheDocument();
    expect(screen.getByText("v0.2.1")).toBeInTheDocument();
    expect(screen.getByText("v0.3.0")).toBeInTheDocument();
    expect(await screen.findByText("Disponível")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Verificar API" }));

    expect(await screen.findByText("Disponível")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
