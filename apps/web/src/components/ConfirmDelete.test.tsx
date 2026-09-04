import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ConfirmDelete } from "./ConfirmDelete";

describe("ConfirmDelete", () => {
  afterEach(cleanup);

  it("keeps confirm button disabled until confirmation word matches exactly", () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();

    render(
      <ConfirmDelete
        cancelLabel="Cancelar"
        confirmLabel="Excluir permanentemente"
        confirmationLabel="Digite Confirmar para prosseguir"
        confirmationWord="Confirmar"
        description="Esta ação não pode ser desfeita."
        title="Exclusão de item"
        onCancel={onCancel}
        onConfirm={onConfirm}
      />,
    );

    const input = screen.getByLabelText("Digite Confirmar para prosseguir");
    const confirmButton = screen.getByRole("button", { name: "Excluir permanentemente" });

    expect(confirmButton).toBeDisabled();
    expect(input).toHaveAttribute("aria-required", "true");

    fireEvent.change(input, { target: { value: "confirmar" } });
    expect(confirmButton).toBeDisabled();

    fireEvent.change(input, { target: { value: "Confirmar" } });
    expect(confirmButton).not.toBeDisabled();

    fireEvent.click(confirmButton);
    expect(onConfirm).toHaveBeenCalledWith("Confirmar");
  });

  it("calls onCancel when cancel button is clicked", () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();

    render(
      <ConfirmDelete
        cancelLabel="Cancelar"
        confirmLabel="Excluir"
        confirmationLabel="Digite DELETAR"
        confirmationWord="DELETAR"
        description="Deseja mesmo deletar?"
        title="Confirmar exclusão"
        onCancel={onCancel}
        onConfirm={onConfirm}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Cancelar" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
