import { describe, expect, it, vi } from "vitest";
import { uploadAttachment } from "../api/attachments";
import { createDocument, updateDocument } from "../api/client";
import { saveRecordCustomValues } from "../../RecordCustomFields";
import { persistDocumentForm, storedRecordOf } from "./cadastroPersist";

vi.mock("../api/client", () => ({
  createDocument: vi.fn(),
  updateDocument: vi.fn(),
  createBill: vi.fn(),
  updateBill: vi.fn(),
  createProfile: vi.fn(),
  updateProfile: vi.fn(),
}));

vi.mock("../api/attachments", () => ({
  uploadAttachment: vi.fn(),
}));

vi.mock("../../RecordCustomFields", () => ({
  saveRecordCustomValues: vi.fn(),
}));

const fields = {
  docTypeId: "doc-type-rg",
  docIdentifier: "111",
  docMedium: "PHYSICAL" as const,
  docCustody: "ORGANIZATION" as const,
  docNotes: "",
  customDraft: {},
  file: new File(["page"], "rg.pdf", { type: "application/pdf" }),
};

describe("persistDocumentForm", () => {
  it("retries an upload with the version written by custom values", async () => {
    vi.mocked(createDocument).mockReset();
    vi.mocked(updateDocument).mockReset();
    vi.mocked(saveRecordCustomValues).mockReset();
    vi.mocked(uploadAttachment).mockReset();
    vi.mocked(createDocument).mockResolvedValue({
      id: "doc-kept",
      version: 1,
      type: { id: "doc-type-rg" },
    } as never);
    vi.mocked(saveRecordCustomValues).mockResolvedValue({
      target_kind: "DOCUMENT",
      target_id: "doc-kept",
      values: [],
      version: 4,
    });
    vi.mocked(uploadAttachment).mockRejectedValueOnce(new Error("upload failed"));

    await expect(
      persistDocumentForm({ ownerProfileId: "profile-1", fields }),
    ).rejects.toThrow(/upload failed/i);
    const failed = vi.mocked(uploadAttachment).mock.results[0]?.value;
    await expect(failed).rejects.toThrow(/upload failed/i);
    const stored = storedRecordOf(await failed.catch((error: unknown) => error));
    expect(stored).toEqual({ id: "doc-kept", version: 4 });
    expect(createDocument).toHaveBeenCalledTimes(1);

    vi.mocked(updateDocument).mockResolvedValue({
      id: "doc-kept",
      version: 5,
      type: { id: "doc-type-rg" },
    } as never);
    vi.mocked(uploadAttachment).mockResolvedValue({ id: "file-1" } as never);
    await persistDocumentForm({
      ownerProfileId: "profile-1",
      fields,
      existing: { id: "doc-kept", version: 4 },
    });
    expect(createDocument).toHaveBeenCalledTimes(1);
    expect(updateDocument).toHaveBeenCalledWith(
      "doc-kept",
      expect.objectContaining({ version: 4, identifier_value: "111" }),
    );
  });
});
