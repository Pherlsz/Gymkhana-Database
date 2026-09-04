import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../../i18n";
import { CadastroSingleScreen } from "./CadastroSingleScreen";
import { createBill, createDocument, createProfile } from "../api/client";

afterEach(cleanup);

vi.mock("../api/client", () => ({
  listDocumentTypes: vi.fn().mockResolvedValue({
    types: [
      { id: "doc-type-rg", code: "rg", label: "RG", description: "" },
      { id: "doc-type-cpf", code: "cpf", label: "CPF", description: "" },
    ],
  }),
  listBillTypes: vi.fn().mockResolvedValue({
    types: [{ id: "bill-type-luz", code: "luz", label: "Energia Elétrica", description: "" }],
  }),
  listProfilesLookup: vi.fn().mockResolvedValue({ profiles: [] }),
  createProfile: vi.fn().mockResolvedValue({ id: "profile-1", full_name: "Novo Usuário" }),
  createDocument: vi.fn().mockResolvedValue({ id: "doc-1" }),
  createBill: vi.fn().mockResolvedValue({ id: "bill-1" }),
}));

function renderSingleScreen(
  targetTable: "people" | "documents" | "bills" = "people",
  onCancel = vi.fn(),
  onSuccess = vi.fn(),
) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
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
      fireEvent.click(screen.getByRole("button", { name: /← Cadastro/i }));
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
      expect(await screen.findByText(/Número do documento/i)).not.toBeNull();
    });

    it("saves new person when clicking save button", async () => {
      const onSuccess = vi.fn();
      renderSingleScreen("people", vi.fn(), onSuccess);

      const nameInput = screen.getByLabelText(/Nome do titular/i);
      fireEvent.change(nameInput, { target: { value: "Carlos Drummond" } });

      const saveBtn = screen.getByRole("button", { name: /Salvar cadastro de pessoa/i });
      fireEvent.click(saveBtn);

      await waitFor(() => {
        expect(createProfile).toHaveBeenCalledWith(
          expect.objectContaining({ full_name: "Carlos Drummond" }),
        );
        expect(onSuccess).toHaveBeenCalledWith("Carlos Drummond");
      });
    });

    it("renders family and complementary sections and saves extra fields", async () => {
      renderSingleScreen("people");

      // Expand Family section
      const familyHeader = screen.getByRole("heading", { name: /Família/i });
      fireEvent.click(familyHeader);
      expect(await screen.findByLabelText(/Nome do pai/i)).not.toBeNull();
      expect(screen.getByLabelText(/Nome da mãe/i)).not.toBeNull();

      const fatherInput = screen.getByLabelText(/Nome do pai/i);
      fireEvent.change(fatherInput, { target: { value: "Alberto Santos" } });

      // Expand Complementary section
      const compHeader = screen.getByRole("heading", { name: /complementares/i });
      fireEvent.click(compHeader);
      expect(await screen.findByLabelText(/Modelo do veículo/i)).not.toBeNull();

      const vehicleInput = screen.getByLabelText(/Modelo do veículo/i);
      fireEvent.change(vehicleInput, { target: { value: "Toyota Corolla" } });

      const nameInput = screen.getByLabelText(/Nome do titular/i);
      fireEvent.change(nameInput, { target: { value: "Beatriz Santos" } });

      const saveBtn = screen.getByRole("button", { name: /Salvar cadastro de pessoa/i });
      fireEvent.click(saveBtn);

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
  });

  describe("Document mode (targetTable='documents')", () => {
    it("renders document fields as primary and optional holder section", () => {
      renderSingleScreen("documents");
      expect(screen.getByRole("heading", { name: /Dados do documento/i })).not.toBeNull();
      expect(screen.getByLabelText(/Número do documento/i)).not.toBeNull();
      expect(screen.getByLabelText(/Data de emissão/i)).not.toBeNull();
      expect(screen.getByLabelText(/Validade/i)).not.toBeNull();
      expect(screen.getByText(/Titular \/ Proprietário/i)).not.toBeNull();
      expect(screen.getByRole("button", { name: /Salvar documento/i })).not.toBeNull();
    });

    it("saves document and auto-creates profile if not pre-linked", async () => {
      const onSuccess = vi.fn();
      renderSingleScreen("documents", vi.fn(), onSuccess);

      const docNumberInput = screen.getByLabelText(/Número do documento/i);
      fireEvent.change(docNumberInput, { target: { value: "MG-12.345.678" } });

      const holderInput = screen.getByLabelText(/Nome do titular/i);
      fireEvent.change(holderInput, { target: { value: "Ana Néri" } });

      const saveBtn = screen.getByRole("button", { name: /Salvar documento/i });
      fireEvent.click(saveBtn);

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
      expect(screen.getByLabelText(/Fornecedor \/ Concessionária/i)).not.toBeNull();
      expect(screen.getByLabelText(/Número de instalação \/ Conta/i)).not.toBeNull();
      expect(screen.getByLabelText(/Competência \/ Vencimento/i)).not.toBeNull();
      expect(screen.getByLabelText(/Valor \(R\$\)/i)).not.toBeNull();
      expect(screen.getByRole("button", { name: /Salvar conta/i })).not.toBeNull();
    });

    it("saves bill and auto-creates profile from bill printed holder", async () => {
      const onSuccess = vi.fn();
      renderSingleScreen("bills", vi.fn(), onSuccess);

      const provInput = screen.getByLabelText(/Fornecedor \/ Concessionária/i);
      fireEvent.change(provInput, { target: { value: "Sabesp" } });

      const instInput = screen.getByLabelText(/Número de instalação \/ Conta/i);
      fireEvent.change(instInput, { target: { value: "987654321" } });

      const printHolderInput = screen.getByLabelText(/Nome impresso na fatura/i);
      fireEvent.change(printHolderInput, { target: { value: "Clarice Lispector" } });

      const saveBtn = screen.getByRole("button", { name: /Salvar conta/i });
      fireEvent.click(saveBtn);

      await waitFor(() => {
        expect(createProfile).toHaveBeenCalledWith(
          expect.objectContaining({ full_name: "Clarice Lispector" }),
        );
        expect(createBill).toHaveBeenCalledWith(
          expect.objectContaining({
            printed_holder_name: "Clarice Lispector",
            reference_value: "987654321",
            owner_profile_id: "profile-1",
          }),
        );
        expect(onSuccess).toHaveBeenCalledWith("Clarice Lispector");
      });
    });
  });
});
