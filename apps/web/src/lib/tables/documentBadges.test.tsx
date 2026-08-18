import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ProfileDocumentBadge } from "../api/client";
import { DocumentBadgeChip, documentTypeAcronym } from "./documentBadges";

const marks = {
  number: { glyph: "nº", label: "Número informado" },
  physical: { label: "Exemplar físico" },
  digital: { label: "Exemplar digital" },
};

function badge(partial: Partial<ProfileDocumentBadge> & Pick<ProfileDocumentBadge, "badge">) {
  return {
    document_type_id: "type-rg",
    technical_key: "rg",
    label: "RG",
    claim: "informed_number",
    has_physical: false,
    has_digital: false,
    in_hands: false,
    ...partial,
  } satisfies ProfileDocumentBadge;
}

afterEach(() => {
  cleanup();
});

describe("documentTypeAcronym", () => {
  it("uses compact type labels and known keys", () => {
    expect(documentTypeAcronym("rg", "RG")).toBe("RG");
    expect(documentTypeAcronym("cpf", "Cadastro de Pessoa Física")).toBe("CPF");
    expect(documentTypeAcronym("voter_id", "Título de Eleitor")).toBe("TE");
    expect(documentTypeAcronym("birth_certificate", "Certidão de Nascimento")).toBe("CN");
    expect(documentTypeAcronym("custom_pass", "ABC")).toBe("ABC");
  });
});

describe("DocumentBadgeChip", () => {
  it("draws a file icon for a physical original, not the letter F", () => {
    render(
      <DocumentBadgeChip
        badge={badge({ badge: "physical" })}
        marks={marks}
        withOwnerLabel="Com o dono"
      />,
    );
    expect(screen.getByText("RG")).toBeInTheDocument();
    expect(screen.getByLabelText("Exemplar físico")).toBeInTheDocument();
    expect(screen.queryByText("F")).toBeNull();
    expect(screen.getByLabelText("Exemplar físico").querySelector("svg")).toBeTruthy();
  });

  it("draws a scan icon for a digital copy, not the letter D", () => {
    render(
      <DocumentBadgeChip
        badge={badge({ badge: "digital" })}
        marks={marks}
        withOwnerLabel="Com o dono"
      />,
    );
    expect(screen.getByLabelText("Exemplar digital")).toBeInTheDocument();
    expect(screen.queryByText("D")).toBeNull();
    expect(screen.getByLabelText("Exemplar digital").querySelector("svg")).toBeTruthy();
  });

  it("keeps the gold person mark when the original is with the owner", () => {
    render(
      <DocumentBadgeChip
        badge={badge({ badge: "physical_with_owner", has_physical: true })}
        marks={marks}
        withOwnerLabel="Com o dono"
      />,
    );
    expect(screen.getByLabelText("Exemplar físico")).toBeInTheDocument();
    expect(screen.getByLabelText("Com o dono")).toBeInTheDocument();
    expect(screen.queryByText("(i)")).toBeNull();
    expect(screen.getByLabelText("Com o dono").querySelector("svg")).toBeTruthy();
  });

  it("keeps the nº glyph for an informed number", () => {
    render(
      <DocumentBadgeChip
        badge={badge({ badge: "informed_number" })}
        marks={marks}
        withOwnerLabel="Com o dono"
      />,
    );
    expect(screen.getByLabelText("Número informado")).toHaveTextContent("nº");
  });
});
