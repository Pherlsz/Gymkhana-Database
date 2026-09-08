// @vitest-environment jsdom
import { message } from "antd";
import { describe, expect, it, vi } from "vitest";
import { announceSaved } from "./cadastroFeedback";

describe("announceSaved", () => {
  it("shows the saved message", () => {
    const success = vi.fn();
    announceSaved({ success }, "Documento salvo");
    expect(success).toHaveBeenCalledWith("Documento salvo");
  });

  it("falls back to the antd static message sink", () => {
    const spy = vi
      .spyOn(message, "success")
      .mockImplementation((() => Object.assign(() => {}, Promise.resolve(true))) as never);
    expect(() => announceSaved(undefined, "Documento salvo")).not.toThrow();
    expect(spy).toHaveBeenCalledWith("Documento salvo");
    spy.mockRestore();
  });
});
