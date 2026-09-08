import { Alert, Button, Flex, Input, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import {
  applyOCRSuggestions,
  consumeOCRJobEvents,
  getOCRCapability,
  getOCRJob,
  listOCRSuggestions,
  newOCRIdempotencyKey,
  reviewOCRSuggestion,
  startOCRJob,
  type OCRJob,
  type OCRSuggestion,
} from "../api/ocr";
import { listAttachments, type AttachmentOwner } from "../api/attachments";
import { APIRequestError } from "../api/client";

export function OcrReviewPanel({ owner }: { owner: AttachmentOwner }) {
  const queryClient = useQueryClient();
  const capability = useQuery({
    queryKey: ["ocr-capability"],
    queryFn: ({ signal }) => getOCRCapability(signal),
  });
  const attachments = useQuery({
    queryKey: ["attachments", owner, false],
    queryFn: ({ signal }) => listAttachments(owner, false, signal),
  });
  const [attachmentID, setAttachmentID] = useState("");
  const [activeJobID, setActiveJobID] = useState<string>();
  const [notice, setNotice] = useState<string | null>(null);
  const job = useQuery({
    queryKey: ["ocr-job", activeJobID],
    queryFn: ({ signal }) => getOCRJob(activeJobID!, signal),
    enabled: Boolean(activeJobID),
    refetchInterval: (query) =>
      query.state.data && !["COMPLETED", "FAILED", "CANCELLED"].includes(query.state.data.state)
        ? 1_500
        : false,
  });
  const suggestions = useQuery({
    queryKey: ["ocr-suggestions", activeJobID],
    queryFn: ({ signal }) => listOCRSuggestions(activeJobID!, signal),
    enabled: Boolean(activeJobID && job.data?.state === "COMPLETED"),
  });
  const ocrAttachments = useMemo(
    () =>
      (attachments.data ?? []).filter((item) =>
        [item.detected_mime, item.declared_mime].some((mime) =>
          ["application/pdf", "image/jpeg", "image/png"].includes(mime),
        ),
      ),
    [attachments.data],
  );

  useEffect(() => {
    if (!attachmentID && ocrAttachments[0]) setAttachmentID(ocrAttachments[0].id);
  }, [attachmentID, ocrAttachments]);

  useEffect(() => {
    if (!activeJobID || !job.data) return;
    if (["COMPLETED", "FAILED", "CANCELLED"].includes(job.data.state)) return;
    const controller = new AbortController();
    void consumeOCRJobEvents(
      activeJobID,
      0,
      () => {
        void queryClient.invalidateQueries({ queryKey: ["ocr-job", activeJobID] });
        void queryClient.invalidateQueries({ queryKey: ["ocr-suggestions", activeJobID] });
      },
      controller.signal,
    );
    return () => controller.abort();
  }, [activeJobID, job.data?.state, queryClient]);

  const start = useMutation({
    mutationFn: () => startOCRJob(attachmentID, newOCRIdempotencyKey("job")),
    onSuccess: (value) => {
      setActiveJobID(value.id);
      setNotice(null);
    },
  });

  if (!capability.data?.enabled) {
    return (
      <Alert
        message="OCR indisponível"
        type="warning"
        description="Ative OCR e anexos privados conforme docs/OCR.md e docs/CADASTRO_R2.md."
      />
    );
  }

  return (
    <section aria-label="Revisão OCR" className="ocr-review-panel">
      <p className="ocr-review-panel__lead">
        Extração gera sugestões com evidência. Nada é aplicado até você confirmar.
      </p>
      {notice ? <Alert showIcon type="success" title={notice} /> : null}
      {start.error ? (
        <Alert
          message="Não foi possível iniciar o OCR"
          type="error"
          description={ocrError(start.error)}
        />
      ) : null}
      <label className="ocr-review-panel__field">
        Anexo (PDF, JPEG ou PNG)
        <select
          aria-label="Anexo para OCR"
          value={attachmentID}
          onChange={(event) => setAttachmentID(event.target.value)}
        >
          <option value="">Selecione</option>
          {ocrAttachments.map((item) => (
            <option key={item.id} value={item.id}>
              {item.original_filename}
            </option>
          ))}
        </select>
      </label>
      <Button
        disabled={!attachmentID || start.isPending}
        type="primary"
        onClick={() => start.mutate()}
      >
        {start.isPending ? "Iniciando…" : "Extrair dados (OCR)"}
      </Button>
      {job.data ? <OcrJobStatus job={job.data} /> : null}
      {(suggestions.data?.suggestions.length ?? 0) > 0 ? (
        <OcrSuggestionList
          job={job.data!}
          suggestions={suggestions.data!.suggestions.map((view) => view.suggestion)}
          onApplied={(message) => {
            setNotice(message);
            void queryClient.invalidateQueries({ queryKey: ["ocr-suggestions", activeJobID] });
          }}
        />
      ) : null}
    </section>
  );
}

function OcrJobStatus({ job }: { job: OCRJob }) {
  return (
    <Flex align="center" aria-live="polite" className="ocr-review-panel__status" gap="0.5rem">
      <Tag>{job.state}</Tag>
      <span>{job.error_code ?? "Processando extração…"}</span>
    </Flex>
  );
}

function OcrSuggestionList({
  job,
  suggestions,
  onApplied,
}: {
  job: OCRJob;
  suggestions: OCRSuggestion[];
  onApplied: (message: string) => void;
}) {
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const review = useMutation({
    mutationFn: (input: {
      suggestion: OCRSuggestion;
      action: "ACCEPT" | "REJECT";
      value?: string;
    }) =>
      reviewOCRSuggestion(
        input.suggestion,
        input.action,
        input.action === "ACCEPT" ? input.value : undefined,
      ),
  });
  const apply = useMutation({
    mutationFn: () => {
      const accepted = suggestions.filter((item) => item.review_state === "ACCEPTED");
      return applyOCRSuggestions(job.id, accepted, newOCRIdempotencyKey("apply"));
    },
    onSuccess: () => onApplied("Valores aceitos foram aplicados ao registro."),
  });

  return (
    <div className="ocr-review-panel__suggestions">
      <h3>Sugestões</h3>
      <table className="ocr-review-panel__table">
        <thead>
          <tr>
            <th scope="col">Campo</th>
            <th scope="col">Valor sugerido</th>
            <th scope="col">Confiança</th>
            <th scope="col">Revisão</th>
          </tr>
        </thead>
        <tbody>
          {suggestions.map((suggestion) => (
            <tr key={suggestion.id}>
              <td>{suggestion.field_label}</td>
              <td>
                <Input
                  aria-label={`Valor para ${suggestion.field_label}`}
                  disabled={suggestion.review_state === "REJECTED"}
                  value={drafts[suggestion.id] ?? suggestion.proposed_value ?? ""}
                  onChange={(event) =>
                    setDrafts((current) => ({ ...current, [suggestion.id]: event.target.value }))
                  }
                />
              </td>
              <td>{formatOCRConfidence(suggestion.evidence.confidence)}</td>
              <td>
                <Flex gap="0.25rem">
                  <Button
                    size="small"
                    onClick={() =>
                      review.mutate({
                        suggestion,
                        action: "ACCEPT",
                        value: drafts[suggestion.id] ?? suggestion.proposed_value ?? "",
                      })
                    }
                  >
                    Aceitar
                  </Button>
                  <Button
                    size="small"
                    onClick={() => review.mutate({ suggestion, action: "REJECT" })}
                  >
                    Rejeitar
                  </Button>
                </Flex>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <Button
        disabled={apply.isPending || !suggestions.some((item) => item.review_state === "ACCEPTED")}
        onClick={() => apply.mutate()}
      >
        Aplicar aceitos
      </Button>
      {apply.error ? (
        <Alert
          message="Não foi possível aplicar"
          type="error"
          description={ocrError(apply.error)}
        />
      ) : null}
    </div>
  );
}

export function formatOCRConfidence(value: number | null | undefined): string {
  if (value == null) return "—";
  return `${Math.round(value / 100)}%`;
}

function ocrError(error: unknown) {
  if (error instanceof APIRequestError) return error.message;
  if (error instanceof Error) return error.message;
  return "Erro inesperado.";
}
