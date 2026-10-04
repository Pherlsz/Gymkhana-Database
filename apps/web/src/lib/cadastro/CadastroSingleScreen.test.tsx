import { message } from "antd";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../../i18n";
import { CadastroSingleScreen } from "./CadastroSingleScreen";
import { createBill, createDocument, createProfile } from "../api/client";

let activeClient: QueryClient | null = null;

afterEach(() => {
  cleanup();
  message.destroy();
  activeClient?.clear();
  activeClient = null;
});

vi.mock("./useAttachmentsEnabled", () => ({
  useAttachmentsEnabled: () => ({ data: false, isFetched: true, isLoading: false }),
}));

vi.mock("../api/ocr", () => ({
  getOCRCapability: vi.fn().mockResolvedValue({ enabled: false }),
}));

vi.mock("../api/attachments", () => ({
  uploadAttachment: vi.fn(),
}));

vi.mock("../api/client", () => ({
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
      {
        id: "doc-type-cpf",
        technical_key: "cpf",
        label: "CPF",
        active: true,
        uniqueness_policy: "PER_PROFILE",
        validation_regex: "",
        date_required: false,
      },
    ],
  }),
  listBillTypes: vi.fn().mockResolvedValue({
    types: [{ id: "bill-type-luz", technical_key: "luz", label: "Energia Elétrica", active: true }],
  }),
  listProfilesLookup: vi.fn().mockResolvedValue({ profiles: [] }),
  listCustomFields: vi.fn().mockResolvedValue({ fields: [] }),
  createProfile: vi.fn().mockResolvedValue({
    id: "profile-1",
    full_name: "Novo Usuário",
    version: 1,
  }),
  updateProfile: vi.fn(),
  createDocument: vi.fn().mockResolvedValue({
    id: "doc-1",
    version: 1,
    type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
  }),
  updateDocument: vi.fn().mockResolvedValue({
    id: "doc-1",
    version: 2,
    type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
  }),
  createBill: vi.fn().mockResolvedValue({
    id: "bill-1",
    version: 1,
    type: { id: "bill-type-luz", technical_key: "luz", label: "Energia Elétrica" },
  }),
  updateBill: vi.fn().mockResolvedValue({
    id: "bill-1",
    version: 2,
    type: { id: "bill-type-luz", technical_key: "luz", label: "Energia Elétrica" },
  }),
  APIRequestError: class APIRequestError extends Error {},
}));

function renderSingleScreen(
  targetTable: "people" | "documents" | "bills" = "people",
  onCancel = vi.fn(),
  onSuccess = vi.fn(),
) {
  const client = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        gcTime: 0,
      },
    },
  });
  activeClient = client;
  return {
    onCancel,
    onSuccess,
    ...render(
      <QueryClientProvider client={client}>
        <I18nProvider locale="pt-BR">
          <CadastroSingleScreen
            targetTable={targetTable}
            onCancel={onCancel}
            onSuccess={onSuccess}
          />
        </I18nProvider>
      </QueryClientProvider>,
    ),
  };
}

async function confirmSave() {
  fireEvent.click(screen.getByRole("button", { name: /Salvar/i }));
}

function fillPersonMinimum(name: string) {
  fireEvent.change(screen.getByLabelText(/Nome do titular/i), { target: { value: name } });
}

describe("CadastroSingleScreen", () => {
  describe("Person mode (targetTable='people')", () => {
    it("renders titular holder field, contact fields, and staged sections", async () => {
      renderSingleScreen("people");
      expect(screen.getByLabelText(/Nome do titular/i)).not.toBeNull();
      expect(screen.getByText(/Identidade & contato/i)).not.toBeNull();
      expect(screen.getByText(/Documentos/i)).not.toBeNull();
      expect(screen.getByText(/Contas de consumo/i)).not.toBeNull();
    });

    it("calls onCancel when clicking breadcrumb back button", () => {
      const onCancel = vi.fn();
      renderSingleScreen("people", onCancel);
      fireEvent.click(screen.getByRole("button", { name: /^Cadastro$/i }));
      expect(onCancel).toHaveBeenCalled();
    });

    it("shows requirement status pill", () => {
      renderSingleScreen("people");
      expect(screen.getByText(/Documento oficial pendente/i)).not.toBeNull();
    });

    it("toggles inline document add form", async () => {
      renderSingleScreen("people");
      const addBtn = screen.getByRole("button", { name: /Adicionar documento/i });
      fireEvent.click(addBtn);
      expect(await screen.findByText(/Preenchimento inteligente via OCR/i)).not.toBeNull();
      expect(screen.getByText(/Número do documento/i)).not.toBeNull();
    });

    it("keeps Salvar disabled until the person name is filled", () => {
      renderSingleScreen("people");
      const save = screen.getByRole("button", { name: /Salvar/i });
      expect(save).toBeDisabled();

      fireEvent.change(screen.getByLabelText(/Nome do titular/i), { target: { value: "Ana" } });
      expect(save).toBeEnabled();
    });

    it("saves new person when clicking save button", async () => {
      const onSuccess = vi.fn();
      renderSingleScreen("people", vi.fn(), onSuccess);

      fillPersonMinimum("Carlos Drummond");

      await confirmSave();

      await waitFor(() => {
        expect(createProfile).toHaveBeenCalledWith(
          expect.objectContaining({ full_name: "Carlos Drummond" }),
        );
        expect(onSuccess).toHaveBeenCalledWith("Carlos Drummond");
      });
    });

    it("renders family and complementary sections and saves extra fields", async () => {
      renderSingleScreen("people");

      const familyHeader = screen.getByRole("heading", { name: /Família/i });
      fireEvent.click(familyHeader);
      expect(await screen.findByLabelText(/Nome do pai/i)).not.toBeNull();
      expect(screen.getByLabelText(/Nome da mãe/i)).not.toBeNull();

      const fatherInput = screen.getByLabelText(/Nome do pai/i);
      fireEvent.change(fatherInput, { target: { value: "Alberto Santos" } });

      const compHeader = screen.getByRole("heading", { name: /complementares/i });
      fireEvent.click(compHeader);
      expect(await screen.findByLabelText(/Modelo do veículo/i)).not.toBeNull();

      const vehicleInput = screen.getByLabelText(/Modelo do veículo/i);
      fireEvent.change(vehicleInput, { target: { value: "Toyota Corolla" } });

      fillPersonMinimum("Beatriz Santos");

      await confirmSave();

      await waitFor(() => {
        expect(createProfile).toHaveBeenCalledWith(
          expect.objectContaining({
            full_name: "Beatriz Santos",
            father_name: "Alberto Santos",
            vehicle_model: "Toyota Corolla",
          }),
        );
      });
    });

    it("keeps typed CPF when the holder search changes", () => {
      renderSingleScreen("people");
      fireEvent.change(screen.getByLabelText(/^CPF$/i), { target: { value: "52998224725" } });
      fillPersonMinimum("Ana Néri");
      expect(screen.getByLabelText(/^CPF$/i)).toHaveValue("529.982.247-25");
    });

    it("updates an existing person without clearing notes", async () => {
      const { listProfilesLookup, updateProfile } = await import("../api/client");
      vi.mocked(listProfilesLookup).mockResolvedValue({
        profiles: [
          {
            id: "profile-ana",
            full_name: "Ana Néri",
            notes: "Observação da mesa",
            version: 3,
          } as any,
        ],
        page: { total: 1, limit: 10, offset: 0, sort_field: "full_name", sort_order: "asc" },
      });
      vi.mocked(updateProfile).mockResolvedValue({
        id: "profile-ana",
        full_name: "Ana Néri",
        notes: "Observação da mesa",
        version: 4,
      } as any);

      renderSingleScreen("people");
      const combobox = screen.getByRole("combobox", { name: /buscar pessoa/i });
      fireEvent.mouseDown(combobox);
      fireEvent.click(await screen.findByText(/^Ana Néri$/i));
      await confirmSave();

      await waitFor(() => {
        expect(updateProfile).toHaveBeenCalledWith(
          "profile-ana",
          expect.objectContaining({ notes: "Observação da mesa", version: 3 }),
        );
      });
    });

    it("retries a failed document save against the person already created", async () => {
      const { createDocument, createProfile, updateProfile } = await import("../api/client");
      vi.mocked(createProfile).mockClear();
      vi.mocked(createDocument).mockClear();
      vi.mocked(updateProfile).mockClear();
      vi.mocked(createDocument).mockClear();
      vi.mocked(createDocument).mockRejectedValueOnce(new Error("mobile_phone: not_mobile"));
      vi.mocked(updateProfile).mockResolvedValue({
        id: "profile-1",
        full_name: "Ana Néri",
        version: 2,
      } as any);

      renderSingleScreen("documents");
      fireEvent.mouseDown(screen.getByLabelText(/Tipo de documento/i));
      fireEvent.click(await screen.findByText(/^RG$/));
      fireEvent.change(screen.getByLabelText(/Número do documento/i), {
        target: { value: "MG-12.345.678" },
      });
      fireEvent.change(screen.getByLabelText(/Nome do titular/i), {
        target: { value: "Ana Néri" },
      });
      await confirmSave();
      expect(await screen.findByText(/not_mobile/i)).not.toBeNull();
      expect(createProfile).toHaveBeenCalledTimes(1);

      vi.mocked(createDocument).mockResolvedValue({
        id: "doc-1",
        version: 1,
        type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
      } as any);
      await confirmSave();

      await waitFor(() => {
        expect(updateProfile).toHaveBeenCalled();
        expect(createProfile).toHaveBeenCalledTimes(1);
      });
    });

    it("does not create a saved document again when a later one fails", async () => {
      const { createDocument, createProfile, updateProfile } = await import("../api/client");
      vi.mocked(createProfile).mockClear();
      vi.mocked(createDocument).mockClear();
      let failSecond = true;
      vi.mocked(createDocument).mockImplementation(async (input) => {
        if (input.identifier_value === "222" && failSecond) {
          failSecond = false;
          throw new Error("second document failed");
        }
        return {
          id: input.identifier_value === "111" ? "doc-1" : "doc-2",
          version: 1,
          type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
        } as never;
      });
      vi.mocked(updateProfile).mockResolvedValue({
        id: "profile-1",
        full_name: "Ana Néri",
        version: 2,
      } as never);

      try {
        renderSingleScreen("people");
        fillPersonMinimum("Ana Néri");
        const stage = async (number: string) => {
          fireEvent.click(screen.getByRole("button", { name: /Adicionar documento/i }));
          fireEvent.mouseDown(await screen.findByLabelText(/Tipo de documento/i));
          const rgOption = (await screen.findAllByText(/^RG$/)).find((node) =>
            node.classList.contains("ant-select-item-option-content"),
          );
          if (!rgOption) throw new Error("RG option missing");
          fireEvent.click(rgOption);
          fireEvent.change(screen.getByLabelText(/Número do documento/i), {
            target: { value: number },
          });
          fireEvent.click(screen.getByRole("button", { name: /^Adicionar$/i }));
          expect(await screen.findByText(new RegExp(`nº ${number}`))).not.toBeNull();
        };
        await stage("111");
        await stage("222");

        await confirmSave();
        expect(await screen.findByText(/second document failed/i)).not.toBeNull();
        expect(createProfile).toHaveBeenCalledTimes(1);
        expect(vi.mocked(createDocument).mock.calls.map((call) => call[0].identifier_value)).toEqual([
          "111",
          "222",
        ]);
        expect(screen.getByText(/Salvo/)).not.toBeNull();

        await confirmSave();
        await waitFor(() => {
          const values = vi
            .mocked(createDocument)
            .mock.calls.map((call) => call[0].identifier_value);
          expect(values.filter((value) => value === "222")).toEqual(["222", "222"]);
        });
        const numbers = vi
          .mocked(createDocument)
          .mock.calls.map((call) => call[0].identifier_value);
        expect(numbers.filter((value) => value === "111")).toEqual(["111"]);
        expect(numbers.filter((value) => value === "222")).toEqual(["222", "222"]);
        expect(createProfile).toHaveBeenCalledTimes(1);
      } finally {
        vi.mocked(createDocument).mockResolvedValue({
          id: "doc-1",
          version: 1,
          type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
        } as never);
      }
    });

    it("keeps the created document id when a later save step fails", async () => {
      const { createDocument, createProfile, updateDocument, updateProfile } = await import(
        "../api/client"
      );
      const custom = await import("../../RecordCustomFields");
      vi.mocked(createProfile).mockClear();
      vi.mocked(createDocument).mockClear();
      vi.mocked(updateDocument).mockClear();
      vi.mocked(createDocument).mockResolvedValue({
        id: "doc-kept",
        version: 1,
        type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
      } as never);
      vi.mocked(updateDocument).mockResolvedValue({
        id: "doc-kept",
        version: 2,
        type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
      } as never);
      vi.mocked(updateProfile).mockResolvedValue({
        id: "profile-1",
        full_name: "Ana Néri",
        version: 2,
      } as never);
      const followUp = vi.spyOn(custom, "saveRecordCustomValues");
      followUp.mockRejectedValueOnce(new Error("custom values failed"));

      try {
        renderSingleScreen("people");
        fillPersonMinimum("Ana Néri");
        fireEvent.click(screen.getByRole("button", { name: /Adicionar documento/i }));
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
        expect(await screen.findByText(/nº 111/)).not.toBeNull();

        await confirmSave();
        expect(await screen.findByText(/custom values failed/i)).not.toBeNull();
        expect(screen.getByText(/Salvo/)).not.toBeNull();
        expect(createDocument).toHaveBeenCalledTimes(1);
        expect(updateDocument).not.toHaveBeenCalled();

        await confirmSave();
        await waitFor(() => {
          expect(updateDocument).toHaveBeenCalledTimes(1);
        });
        expect(createDocument).toHaveBeenCalledTimes(1);
        expect(updateDocument).toHaveBeenCalledWith(
          "doc-kept",
          expect.objectContaining({ version: 1, identifier_value: "111" }),
        );
      } finally {
        followUp.mockRestore();
        vi.mocked(createDocument).mockResolvedValue({
          id: "doc-1",
          version: 1,
          type: { id: "doc-type-rg", technical_key: "rg", label: "RG" },
        } as never);
      }
    });

    it("creates only one profile when save is clicked twice", async () => {
      const { createProfile } = await import("../api/client");
      vi.mocked(createProfile).mockClear();
      let resolveProfile: (profile: unknown) => void = () => undefined;
      vi.mocked(createProfile).mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveProfile = resolve;
          }) as never,
      );
      try {
        renderSingleScreen("people");
        fillPersonMinimum("Ana Néri");
        const save = screen.getByRole("button", { name: /Salvar/i });
        fireEvent.click(save);
        fireEvent.click(save);
        expect(createProfile).toHaveBeenCalledTimes(1);
        resolveProfile({ id: "profile-1", full_name: "Ana Néri", version: 1 });
        await waitFor(() => expect(createProfile).toHaveBeenCalledTimes(1));
      } finally {
        vi.mocked(createProfile).mockResolvedValue({
          id: "profile-1",
          full_name: "Novo Usuário",
          version: 1,
        } as never);
      }
    });

    it("allows direct editing of titular fields in people mode", () => {
      renderSingleScreen("people");
      const nameInput = screen.getByLabelText(/Nome do titular/i);
      expect(nameInput).not.toBeDisabled();
      const cpfInput = screen.getByLabelText(/^CPF$/i);
      expect(cpfInput).not.toBeDisabled();
      const emailInput = screen.getByLabelText(/E-mail/i);
      expect(emailInput).not.toBeDisabled();
    });
  });

  describe("Document mode (targetTable='documents')", () => {
    it("clears auto-filled fields when unlinking titular", async () => {
      const { listProfilesLookup } = await import("../api/client");
      vi.mocked(listProfilesLookup).mockResolvedValue({
        profiles: [
          {
            id: "profile-pedro",
            full_name: "Pedro Henrique Silveira Lopes",
            cpf: "03650716097",
            mobile_phone: "51998606105",
            email: "pedro@example.com",
            birth_date: "1998-02-24",
            address: { street: "Rua Oito", number: "120", city: "Butiá", state: "RS" },
          } as any,
        ],
        page: { total: 1, limit: 10, offset: 0, sort_field: "full_name", sort_order: "asc" },
      });

      renderSingleScreen("documents");

      const combobox = screen.getByRole("combobox", { name: /Nome do titular/i });
      fireEvent.mouseDown(combobox);

      const option = await screen.findByText(/Pedro Henrique Silveira Lopes/i);
      fireEvent.click(option);

      await waitFor(() => {
        expect(screen.getByLabelText(/^CPF$/i)).toHaveValue("03650716097");
      });

      const unlinkBtn = screen.getByRole("button", { name: /Desvincular/i });
      fireEvent.click(unlinkBtn);

      await waitFor(() => {
        expect(screen.getByLabelText(/^CPF$/i)).toHaveValue("");
      });
    });
    it("renders document fields as primary and optional holder section", () => {
      renderSingleScreen("documents");
      expect(screen.getByRole("heading", { name: /Dados do documento/i })).not.toBeNull();
      expect(screen.getByText(/Preenchimento inteligente via OCR/i)).not.toBeNull();
      expect(screen.getByLabelText(/Número do documento/i)).not.toBeNull();
      expect(screen.getByLabelText(/Data de emissão/i)).not.toBeNull();
      expect(screen.getByLabelText(/Validade/i)).not.toBeNull();
      expect(screen.getByText(/Titular \/ Proprietário/i)).not.toBeNull();
      expect(screen.getByRole("button", { name: /Salvar documento/i })).not.toBeNull();
    });

    it("saves document and auto-creates profile if not pre-linked", async () => {
      const onSuccess = vi.fn();
      renderSingleScreen("documents", vi.fn(), onSuccess);

      const typeSelect = screen.getByLabelText(/Tipo de documento/i);
      fireEvent.mouseDown(typeSelect);
      fireEvent.click(await screen.findByText(/^RG$/));
      const docNumberInput = screen.getByLabelText(/Número do documento/i);
      fireEvent.change(docNumberInput, { target: { value: "MG-12.345.678" } });

      const holderInput = screen.getByLabelText(/Nome do titular/i);
      fireEvent.change(holderInput, { target: { value: "Ana Néri" } });

      await confirmSave();

      await waitFor(() => {
        expect(createProfile).toHaveBeenCalledWith(
          expect.objectContaining({ full_name: "Ana Néri" }),
        );
        expect(createDocument).toHaveBeenCalledWith(
          expect.objectContaining({
            identifier_value: "MG-12.345.678",
            owner_profile_id: "profile-1",
          }),
        );
        expect(onSuccess).toHaveBeenCalledWith("Ana Néri");
      });
    });
  });

  describe("Bill mode (targetTable='bills')", () => {
    it("renders bill fields as primary and optional holder section", () => {
      renderSingleScreen("bills");
      expect(screen.getByText(/Dados da fatura \/ serviço/i)).not.toBeNull();
      expect(screen.getByText("Consumo")).not.toBeNull();
      expect(screen.getByText(/Preenchimento inteligente via OCR/i)).not.toBeNull();
      expect(screen.getByText(/IA \/ OCR/i)).not.toBeNull();
      expect(screen.getByLabelText(/Número de instalação \/ Conta/i)).not.toBeNull();
      expect(screen.getByLabelText(/Competência \/ Vencimento/i)).not.toBeNull();
      expect(screen.getByLabelText(/Valor \(R\$\)/i)).not.toBeNull();
      expect(screen.getByLabelText(/CPF/i)).not.toBeNull();
      expect(screen.getByRole("button", { name: /Salvar conta/i })).not.toBeNull();
    });

    it("saves bill and auto-creates profile with CPF from bill holder", async () => {
      const onSuccess = vi.fn();
      renderSingleScreen("bills", vi.fn(), onSuccess);

      const serviceSelect = screen.getByLabelText(/Serviço/i);
      fireEvent.mouseDown(serviceSelect);
      fireEvent.click(await screen.findByText(/Energia Elétrica/i));
      const instInput = screen.getByLabelText(/Número de instalação \/ Conta/i);
      fireEvent.change(instInput, { target: { value: "987654321" } });

      const competenceInput = screen.getByLabelText(/Competência \/ Vencimento/i);
      fireEvent.change(competenceInput, { target: { value: "2026-09" } });

      const amountInput = screen.getByLabelText(/Valor \(R\$\)/i);
      fireEvent.change(amountInput, { target: { value: "142,50" } });

      fireEvent.click(screen.getByRole("button", { name: /\+ Dados extras da fatura/i }));
      const printHolderInput = screen.getByLabelText(/Nome impresso na fatura/i);
      fireEvent.change(printHolderInput, { target: { value: "Clarice Lispector" } });

      const cpfInput = screen.getByLabelText(/CPF/i);
      fireEvent.change(cpfInput, { target: { value: "123.456.789-00" } });

      await confirmSave();

      await waitFor(() => {
        expect(createProfile).toHaveBeenCalledWith(
          expect.objectContaining({
            full_name: "Clarice Lispector",
            cpf: "123.456.789-00",
          }),
        );
        expect(createBill).toHaveBeenCalledWith(
          expect.objectContaining({
            printed_holder_name: "Clarice Lispector",
            reference_value: "987654321",
            competence: "2026-09",
            amount: "142.50",
            owner_profile_id: "profile-1",
          }),
        );
        expect(onSuccess).toHaveBeenCalledWith("Clarice Lispector");
      });
    });
  });
});
