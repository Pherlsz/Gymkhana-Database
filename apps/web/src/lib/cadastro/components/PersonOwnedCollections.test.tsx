import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../../../i18n";
import { createDocument, updateDocument, type Profile } from "../../api/client";
import { PersonOwnedCollections } from "./PersonOwnedCollections";

vi.mock("../../api/ocr", () => ({
  getOCRCapability: vi.fn().mockResolvedValue({ enabled: false }),
}));

vi.mock("../../api/attachments", () => ({
  uploadAttachment: vi.fn(),
}));

vi.mock("../useAttachmentsEnabled", () => ({
  useAttachmentsEnabled: () => ({ data: false, isFetched: true, isLoading: false }),
}));

vi.mock("../../api/client", () => ({
  listDocumentTypes: vi.fn().mockResolvedValue({
    types: [
      {
        id: "doc-type-rg",
        technical_key: "rg",
        label: "RG",
        active: true,
        uniqueness_policy: "PER_PROFILE",
        validation_regex: "",
        date_required: false,
      },
    ],
  }),
  listDocuments: vi.fn().mockResolvedValue({ documents: [] }),
  listBills: vi.fn().mockResolvedValue({ bills: [] }),
  listCustomFields: vi.fn().mockResolvedValue({ fields: [] }),
  createDocument: vi.fn().mockResolvedValue({
    id: "doc-kept",
    version: 1,
    type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
  }),
  updateDocument: vi.fn().mockResolvedValue({
    id: "doc-kept",
    version: 2,
    type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
  }),
  APIRequestError: class APIRequestError extends Error {},
}));

afterEach(() => {
  cleanup();
});

describe("PersonOwnedCollections", () => {
  it("updates the document already created when a later add step fails", async () => {
    const custom = await import("../../../RecordCustomFields");
    const spy = vi.spyOn(custom, "saveRecordCustomValues");
    spy.mockRejectedValueOnce(new Error("custom values failed"));
    vi.mocked(createDocument).mockClear();
    vi.mocked(updateDocument).mockClear();

    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });
    render(
      <QueryClientProvider client={client}>
        <I18nProvider locale="pt-BR">
          <PersonOwnedCollections
            editable
            profile={{ id: "profile-1", full_name: "Ana Néri", version: 1 } as Profile}
          />
        </I18nProvider>
      </QueryClientProvider>,
    );

    try {
      fireEvent.click(await screen.findByRole("button", { name: /Adicionar documento/i }));
      fireEvent.mouseDown(await screen.findByLabelText(/Tipo de documento/i));
      const rgOption = (await screen.findAllByText(/^RG$/)).find((node) =>
        node.classList.contains("ant-select-item-option-content"),
      );
      if (!rgOption) throw new Error("RG option missing");
      fireEvent.click(rgOption);
      fireEvent.change(screen.getByLabelText(/Número do documento/i), {
        target: { value: "111" },
      });
      fireEvent.click(screen.getByRole("button", { name: /^Adicionar$/i }));

      expect(await screen.findByText(/custom values failed/i)).not.toBeNull();
      expect(createDocument).toHaveBeenCalledTimes(1);
      expect(updateDocument).not.toHaveBeenCalled();

      fireEvent.click(screen.getByRole("button", { name: /^Adicionar$/i }));
      await waitFor(() => expect(updateDocument).toHaveBeenCalledTimes(1));
      expect(createDocument).toHaveBeenCalledTimes(1);
      expect(updateDocument).toHaveBeenCalledWith(
        "doc-kept",
        expect.objectContaining({ version: 1, identifier_value: "111" }),
      );
    } finally {
      spy.mockRestore();
    }
  });
});
