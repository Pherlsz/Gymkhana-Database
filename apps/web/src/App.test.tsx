import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

describe("App", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("shows the API as available and allows checking again", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);

    expect(screen.getByRole("heading", { name: "Gymkhana Database" })).toBeInTheDocument();
    expect(await screen.findByText("Disponível")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Verificar API" }));

    expect(await screen.findByText("Disponível")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});
