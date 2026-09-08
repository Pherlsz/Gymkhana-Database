import { describe, expect, it } from "vitest";
import { formatOCRConfidence } from "./OcrReviewPanel";

describe("formatOCRConfidence", () => {
  it("renders Core 0..10000 confidence as a percent", () => {
    expect(formatOCRConfidence(undefined)).toBe("—");
    expect(formatOCRConfidence(9000)).toBe("90%");
    expect(formatOCRConfidence(10000)).toBe("100%");
  });
});
