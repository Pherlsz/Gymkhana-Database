import { afterEach, describe, expect, it, vi } from "vitest";
import {
  RECORTE_XLSX_FALLBACK,
  downloadChatResultXlsx,
  filenameFromContentDisposition,
} from "./recorteExport";

describe("recorte xlsx export", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("reads the attachment filename", () => {
    expect(filenameFromContentDisposition(null)).toBe(RECORTE_XLSX_FALLBACK);
    expect(
      filenameFromContentDisposition('attachment; filename="recorte-20260925-031504.xlsx"'),
    ).toBe("recorte-20260925-031504.xlsx");
    expect(filenameFromContentDisposition("attachment; filename*=UTF-8''recorte%20clip.xlsx")).toBe(
      "recorte clip.xlsx",
    );
  });

  it("downloads only the named recorte workbook", async () => {
    const createObjectURL = vi.fn(() => "blob:recorte");
    const revokeObjectURL = vi.fn();
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(new Uint8Array([0x50, 0x4b, 0x03, 0x04]), {
          status: 200,
          headers: {
            "content-type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
            "content-disposition": 'attachment; filename="recorte-test.xlsx"',
          },
        }),
      ),
    );
    vi.stubGlobal("URL", { ...URL, createObjectURL, revokeObjectURL });

    await downloadChatResultXlsx("11111111-1111-4111-8111-111111111111");

    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/chat/result-references/11111111-1111-4111-8111-111111111111/xlsx",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(createObjectURL).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:recorte");
  });
});
