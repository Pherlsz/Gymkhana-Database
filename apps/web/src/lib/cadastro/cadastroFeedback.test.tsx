// @vitest-environment jsdom
import { message } from "antd";
import { afterEach, describe, expect, it, vi } from "vitest";
import { announceSaved } from "./cadastroFeedback";

afterEach(() => {
  message.destroy();
});

describe("announceSaved", () => {
  it("shows the saved message", () => {
    const success = vi.fn();
    announceSaved({ success }, "Documento salvo");
    expect(success).toHaveBeenCalledWith("Documento salvo");
  });

  it("falls back to the antd static message sink", () => {
    expect(() => announceSaved(undefined, "Documento salvo")).not.toThrow();
  });
});
