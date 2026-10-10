import { describe, expect, it } from "vitest";
import { formatOCRConfidence, ocrJobStateLabel } from "./ocrDisplay";

describe("ocrDisplay", () => {
  it("formats confidence from ratio, percent, or basis points", () => {
    expect(formatOCRConfidence(null)).toBe("—");
    expect(formatOCRConfidence(0.87)).toBe("87%");
    expect(formatOCRConfidence(87)).toBe("87%");
    expect(formatOCRConfidence(8750)).toBe("88%");
    expect(formatOCRConfidence(9000)).toBe("90%");
  });

  it("maps job states through the catalog and keeps unknown values", () => {
    expect(ocrJobStateLabel("COMPLETED", { COMPLETED: "Concluído" })).toBe("Concluído");
    expect(ocrJobStateLabel("OTHER", { COMPLETED: "Concluído" })).toBe("OTHER");
  });
});
