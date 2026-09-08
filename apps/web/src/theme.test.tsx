import { StrictMode } from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ThemeProvider, ThemeToggle } from "./theme";

function renderToggle(strict = false) {
  const ui = (
    <ThemeProvider>
      <ThemeToggle
        activateDark="activate dark"
        activateLight="activate light"
        darkLabel="dark"
        lightLabel="light"
      />
    </ThemeProvider>
  );
  return render(strict ? <StrictMode>{ui}</StrictMode> : ui);
}

describe("ThemeProvider", () => {
  afterEach(() => {
    cleanup();
    localStorage.removeItem("gymkhana-theme");
    document.documentElement.classList.remove("dark");
    document.documentElement.removeAttribute("data-theme-boot");
  });

  it("syncs html.dark from stored theme without toggling twice", async () => {
    localStorage.setItem("gymkhana-theme", "dark");
    renderToggle();
    await waitFor(() => expect(document.documentElement).toHaveClass("dark"));
    expect(localStorage.getItem("gymkhana-theme")).toBe("dark");
    expect(screen.getByRole("button", { name: "activate light" })).toBeInTheDocument();
  });

  it("adds and removes html.dark once per click", async () => {
    renderToggle();
    expect(document.documentElement).not.toHaveClass("dark");

    fireEvent.click(screen.getByRole("button", { name: "activate dark" }));
    await waitFor(() => expect(document.documentElement).toHaveClass("dark"));
    expect(localStorage.getItem("gymkhana-theme")).toBe("dark");

    fireEvent.click(screen.getByRole("button", { name: "activate light" }));
    await waitFor(() => expect(document.documentElement).not.toHaveClass("dark"));
    expect(localStorage.getItem("gymkhana-theme")).toBe("light");
  });

  it("does not double-toggle html.dark under StrictMode", async () => {
    renderToggle(true);
    fireEvent.click(screen.getByRole("button", { name: "activate dark" }));
    await waitFor(() => expect(document.documentElement).toHaveClass("dark"));
    expect(localStorage.getItem("gymkhana-theme")).toBe("dark");

    fireEvent.click(screen.getByRole("button", { name: "activate light" }));
    await waitFor(() => expect(document.documentElement).not.toHaveClass("dark"));
    expect(localStorage.getItem("gymkhana-theme")).toBe("light");
  });

  it("removes data-theme-boot after the first frame", async () => {
    document.documentElement.setAttribute("data-theme-boot", "");
    renderToggle();
    await waitFor(() =>
      expect(document.documentElement.hasAttribute("data-theme-boot")).toBe(false),
    );
  });
});
