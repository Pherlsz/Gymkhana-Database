import { Alert, Button, Inline, Page, Stack, StatusBadge, Surface } from "@pherlsz/gymkhana-ui";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ocrRoute } from "./App";
import { APIRequestError } from "./lib/api/client";
import { downloadAttachmentByID } from "./lib/api/attachments";
import {
  applyOCRSuggestions,
  cancelOCRJob,
  consumeOCRJobEvents,
  getOCRCapability,
  getOCRJob,
  isActiveOCRJob,
  listOCRJobs,
  listOCRSuggestions,
  newOCRIdempotencyKey,
  reviewOCRSuggestion,
  startOCRJob,
  type OCRApplyReceipt,
  type OCRJob,
  type OCRJobEvent,
  type OCRSuggestion,
  type OCRSuggestionView,
} from "./lib/api/ocr";

export type OCRSearch = { job: string | undefined; attachment: string | undefined };

export function normalizeOCRSearch(search: Record<string, unknown>): OCRSearch {
  return {
    job: typeof search.job === "string" && search.job ? search.job : undefined,
    attachment:
      typeof search.attachment === "string" && search.attachment ? search.attachment : undefined,
  };
}

export function OCRPage() {
  const search = ocrRoute.useSearch();
  const navigate = ocrRoute.useNavigate();
  const queryClient = useQueryClient();
  const [attachmentID, setAttachmentID] = useState(search.attachment ?? "");
  const capability = useQuery({
    queryKey: ["ocr", "capability"],
    queryFn: ({ signal }) => getOCRCapability(signal),
    staleTime: 60_000,
  });
  const jobs = useQuery({
    queryKey: ["ocr", "jobs"],
    queryFn: ({ signal }) => listOCRJobs(signal),
    enabled: capability.data?.enabled === true,
  });
  const selectedJob = useQuery({
    queryKey: ["ocr", "job", search.job],
    queryFn: ({ signal }) => getOCRJob(search.job!, signal),
    enabled: capability.data?.enabled === true && Boolean(search.job),
  });

  const selectJob = useCallback(
    (job: string | undefined) => {
      void navigate({ replace: true, search: (current) => ({ ...current, job }) });
    },
    [navigate],
  );

  useEffect(() => {
    if (search.attachment) setAttachmentID(search.attachment);
  }, [search.attachment]);

  useEffect(() => {
    if (search.job || !jobs.data?.jobs.length) return;
    selectJob(jobs.data.jobs[0]?.id);
  }, [jobs.data, search.job, selectJob]);

  const refreshJobs = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ["ocr", "jobs"] });
  }, [queryClient]);

  const start = useMutation({
    mutationFn: ({ attachment, retryOf }: { attachment: string; retryOf?: string }) =>
      startOCRJob(attachment.trim(), newOCRIdempotencyKey("job"), retryOf),
    onSuccess: async (job) => {
      setAttachmentID("");
      await refreshJobs();
      void navigate({ replace: true, search: { job: job.id, attachment: undefined } });
      queryClient.setQueryData(["ocr", "job", job.id], job);
    },
  });

  if (capability.isPending) {
    return <OCRPageState title="Verificando o OCR">Validando a configuração segura.</OCRPageState>;
  }
  if (capability.isError) {
    return (
      <OCRPageState title="OCR indisponível" tone="danger">
        {ocrErrorMessage(capability.error)}
      </OCRPageState>
    );
  }
  if (!capability.data.enabled) {
    return (
      <OCRPageState title="OCR ainda não ativado" tone="warning">
        O recurso permanece bloqueado até que provedor e modelo sejam escolhidos. Nenhum anexo é
        enviado a um modelo enquanto estiver desativado.
      </OCRPageState>
    );
  }

  return (
    <Page.Root maxWidth="full">
      <Page.Header>
        <Page.Eyebrow>Extração privada · revisão humana obrigatória</Page.Eyebrow>
        <Page.Title>OCR de anexos</Page.Title>
        <Page.Description>
          Extraia campos de PDF, JPEG ou PNG, confira evidências e aplique somente as sugestões
          aprovadas explicitamente.
        </Page.Description>
      </Page.Header>
      <Page.Content>
        <Stack gap="5">
          <Alert title="Nenhuma alteração é automática" tone="info">
            A extração apenas cria sugestões. Aceitar uma sugestão ainda não altera o cadastro; a
            aplicação é uma segunda ação explícita e protegida por versão.
          </Alert>
          <Surface className="ocr-start" tone="raised">
            <Stack gap="3">
              <h2>Nova extração</h2>
              <label>
                Identificador do anexo
                <input
                  aria-label="Identificador do anexo"
                  placeholder="00000000-0000-0000-0000-000000000000"
                  value={attachmentID}
                  onChange={(event) => setAttachmentID(event.target.value)}
                />
              </label>
              <Inline>
                <Button
                  disabled={start.isPending || !attachmentID.trim()}
                  onClick={() => start.mutate({ attachment: attachmentID })}
                >
                  {start.isPending ? "Enfileirando" : "Extrair campos"}
                </Button>
                <span className="ocr-muted">
                  Máximo de {formatBytes(capability.data.maximum_source_bytes)},{" "}
                  {capability.data.maximum_pages} páginas ou{" "}
                  {formatNumber(capability.data.maximum_pixels)} pixels.
                </span>
              </Inline>
            </Stack>
          </Surface>
          {start.isError ? (
            <Alert title="Não foi possível iniciar a extração" tone="danger">
              {ocrErrorMessage(start.error)}
            </Alert>
          ) : null}
          {jobs.isError ? (
            <Alert title="Não foi possível listar as extrações" tone="danger">
              {ocrErrorMessage(jobs.error)}
            </Alert>
          ) : null}
          <div className="ocr-workspace">
            <JobSidebar
              jobs={jobs.data?.jobs ?? []}
              pending={jobs.isPending}
              selectedID={search.job}
              onSelect={selectJob}
            />
            {search.job ? (
              selectedJob.isError ? (
                <Surface className="ocr-empty" tone="raised">
                  <Alert title="Não foi possível abrir a extração" tone="danger">
                    {ocrErrorMessage(selectedJob.error)}
                  </Alert>
                </Surface>
              ) : selectedJob.data ? (
                <OCRJobWorkspace
                  job={selectedJob.data}
                  key={selectedJob.data.id}
                  onJobChanged={async (job) => {
                    queryClient.setQueryData(["ocr", "job", job.id], job);
                    await refreshJobs();
                  }}
                  onRetry={(job) =>
                    start.mutate({ attachment: job.attachment_id, retryOf: job.id })
                  }
                />
              ) : (
                <Surface className="ocr-empty" tone="raised">
                  <p role="status">Carregando extração…</p>
                </Surface>
              )
            ) : (
              <Surface className="ocr-empty" tone="raised">
                <Stack gap="2">
                  <h2>Selecione uma extração</h2>
                  <p>Os jobs e suas evidências são privados para o usuário autenticado.</p>
                </Stack>
              </Surface>
            )}
          </div>
        </Stack>
      </Page.Content>
    </Page.Root>
  );
}

function OCRPageState({
  title,
  tone = "info",
  children,
}: {
  title: string;
  tone?: "info" | "warning" | "danger";
  children: ReactNode;
}) {
  return (
    <Page.Root maxWidth="lg">
      <Page.Header>
        <Page.Eyebrow>Extração privada</Page.Eyebrow>
        <Page.Title>OCR de anexos</Page.Title>
      </Page.Header>
      <Page.Content>
        <Alert title={title} tone={tone}>
          {children}
        </Alert>
      </Page.Content>
    </Page.Root>
  );
}

function JobSidebar({
  jobs,
  pending,
  selectedID,
  onSelect,
}: {
  jobs: OCRJob[];
  pending: boolean;
  selectedID: string | undefined;
  onSelect: (id: string) => void;
}) {
  return (
    <Surface className="ocr-jobs" tone="raised">
      <h2>Extrações</h2>
      {pending ? <p role="status">Carregando extrações…</p> : null}
      {!pending && jobs.length === 0 ? <p>Nenhuma extração criada.</p> : null}
      <nav aria-label="Extrações OCR">
        {jobs.map((job) => (
          <button
            aria-current={job.id === selectedID ? "page" : undefined}
            className="ocr-job-link"
            key={job.id}
            type="button"
            onClick={() => onSelect(job.id)}
          >
            <span>
              <strong>{mimeLabel(job.source_mime)}</strong>
              <small>{shortIdentifier(job.attachment_id)}</small>
            </span>
            <StatusBadge tone={jobTone(job)}>{jobStateLabel(job.state)}</StatusBadge>
          </button>
        ))}
      </nav>
    </Surface>
  );
}

function OCRJobWorkspace({
  job,
  onJobChanged,
  onRetry,
}: {
  job: OCRJob;
  onJobChanged: (job: OCRJob) => Promise<void>;
  onRetry: (job: OCRJob) => void;
}) {
  const queryClient = useQueryClient();
  const cursor = useRef(0);
  const [activity, setActivity] = useState("Job carregado.");
  const [streamError, setStreamError] = useState<Error | null>(null);
  const cancel = useMutation({
    mutationFn: () => cancelOCRJob(job.id),
    onSuccess: onJobChanged,
  });
  const download = useMutation({ mutationFn: () => downloadAttachmentByID(job.attachment_id) });

  useEffect(() => {
    if (!isActiveOCRJob(job)) return;
    const controller = new AbortController();
    setStreamError(null);
    void consumeOCRJobEvents(
      job.id,
      cursor.current,
      (event) => {
        cursor.current = event.sequence;
        setActivity(eventLabel(event));
        void queryClient.invalidateQueries({ queryKey: ["ocr", "job", job.id] });
        if (event.kind === "JOB_COMPLETED") {
          void queryClient.invalidateQueries({ queryKey: ["ocr", "suggestions", job.id] });
        }
      },
      controller.signal,
    ).catch((error: unknown) => {
      if (!controller.signal.aborted) setStreamError(asError(error));
    });
    return () => controller.abort();
  }, [job.id, job.state, queryClient]);

  return (
    <div className="ocr-review-workspace">
      <Surface className="ocr-job-summary" tone="raised">
        <Stack gap="3">
          <Inline align="center">
            <div>
              <h2>Extração {shortIdentifier(job.id)}</h2>
              <p className="ocr-muted">
                Anexo {shortIdentifier(job.attachment_id)} · {mimeLabel(job.source_mime)} ·{" "}
                {formatBytes(job.source_bytes)}
              </p>
            </div>
            <StatusBadge tone={jobTone(job)}>{jobStateLabel(job.state)}</StatusBadge>
          </Inline>
          <div className="ocr-job-metrics">
            <Metric label="Tentativa" value={`${job.attempt_count}/3`} />
            <Metric label="Páginas" value={String(job.page_count)} />
            <Metric label="Sugestões" value={String(job.suggestion_count)} />
            <Metric label="Uso do provedor" value={formatNumber(job.provider_usage)} />
            <Metric label="Atualizado" value={formatDateTime(job.updated_at)} />
          </div>
          {isActiveOCRJob(job) ? <p aria-live="polite">{activity}</p> : null}
          {job.error_code ? (
            <Alert title="Extração encerrada" tone="danger">
              Código seguro: {job.error_code}
            </Alert>
          ) : null}
          {streamError ? (
            <Alert title="Acompanhamento em tempo real interrompido" tone="warning">
              {ocrErrorMessage(streamError)} Atualize o job para consultar o estado durável.
            </Alert>
          ) : null}
          {cancel.isError ? (
            <Alert title="Não foi possível cancelar" tone="danger">
              {ocrErrorMessage(cancel.error)}
            </Alert>
          ) : null}
          {download.isError ? (
            <Alert title="Não foi possível abrir o anexo" tone="danger">
              {ocrErrorMessage(download.error)}
            </Alert>
          ) : null}
          <Inline>
            <Button disabled={download.isPending} onClick={() => download.mutate()}>
              {download.isPending ? "Preparando anexo" : "Abrir original"}
            </Button>
            {isActiveOCRJob(job) ? (
              <Button disabled={cancel.isPending} onClick={() => cancel.mutate()}>
                {cancel.isPending ? "Cancelando" : "Cancelar extração"}
              </Button>
            ) : null}
            {(job.state === "FAILED" || job.state === "CANCELLED") && (
              <Button onClick={() => onRetry(job)}>Tentar novamente</Button>
            )}
          </Inline>
        </Stack>
      </Surface>
      {job.state === "COMPLETED" ? <SuggestionReview job={job} /> : null}
      {job.state === "QUEUED" || job.state === "RUNNING" ? (
        <Surface className="ocr-empty" tone="raised">
          <Stack gap="2">
            <h2>Processando com limites seguros</h2>
            <p>O arquivo permanece privado e as sugestões aparecerão após a validação completa.</p>
          </Stack>
        </Surface>
      ) : null}
    </div>
  );
}

function SuggestionReview({ job }: { job: OCRJob }) {
  const queryClient = useQueryClient();
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [selected, setSelected] = useState<Set<string>>(() => new Set());
  const [receipt, setReceipt] = useState<OCRApplyReceipt | null>(null);
  const [confirmingApply, setConfirmingApply] = useState(false);
  const suggestions = useQuery({
    queryKey: ["ocr", "suggestions", job.id],
    queryFn: ({ signal }) => listOCRSuggestions(job.id, signal),
  });

  useEffect(() => {
    if (!suggestions.data) return;
    setDrafts((current) => {
      const next = { ...current };
      for (const view of suggestions.data.suggestions) {
        next[view.suggestion.id] ??=
          view.suggestion.reviewed_value ?? view.suggestion.proposed_value;
      }
      return next;
    });
  }, [suggestions.data]);

  const review = useMutation({
    mutationFn: ({
      suggestion,
      action,
      value,
    }: {
      suggestion: OCRSuggestion;
      action: "ACCEPT" | "REJECT";
      value?: string;
    }) => reviewOCRSuggestion(suggestion, action, value),
    onSuccess: async (updated) => {
      setSelected((current) => {
        const next = new Set(current);
        if (updated.review_state !== "ACCEPTED") next.delete(updated.id);
        return next;
      });
      await queryClient.invalidateQueries({ queryKey: ["ocr", "suggestions", job.id] });
    },
  });

  const acceptedSelected = useMemo(
    () =>
      (suggestions.data?.suggestions ?? [])
        .filter(
          (view) =>
            selected.has(view.suggestion.id) &&
            view.suggestion.review_state === "ACCEPTED" &&
            !view.stale,
        )
        .map((view) => view.suggestion),
    [selected, suggestions.data],
  );

  const apply = useMutation({
    mutationFn: () => applyOCRSuggestions(job.id, acceptedSelected, newOCRIdempotencyKey("apply")),
    onSuccess: async (value) => {
      setReceipt(value);
      setConfirmingApply(false);
      setSelected(new Set());
      await queryClient.invalidateQueries({ queryKey: ["ocr", "suggestions", job.id] });
    },
  });

  if (suggestions.isPending) {
    return (
      <Surface className="ocr-empty" tone="raised">
        <p role="status">Carregando sugestões e versões atuais…</p>
      </Surface>
    );
  }
  if (suggestions.isError) {
    return (
      <Alert title="Não foi possível carregar as sugestões" tone="danger">
        {ocrErrorMessage(suggestions.error)}
      </Alert>
    );
  }

  return (
    <Stack gap="4">
      <Surface className="ocr-apply-bar" tone="raised">
        <Inline align="center">
          <div>
            <strong>Aplicação explícita</strong>
            <p className="ocr-muted">
              {acceptedSelected.length} sugestão(ões) aceita(s), atual(is) e selecionada(s).
            </p>
          </div>
          {!confirmingApply ? (
            <Button
              disabled={apply.isPending || acceptedSelected.length === 0}
              onClick={() => setConfirmingApply(true)}
            >
              Revisar aplicação
            </Button>
          ) : null}
        </Inline>
      </Surface>
      {confirmingApply ? (
        <Surface className="ocr-confirm" tone="raised">
          <Stack gap="3">
            <Alert title="Confirmar alteração dos cadastros" tone="warning">
              Esta ação aplicará {acceptedSelected.length} valor(es) já revisado(s) usando as
              versões atuais. Conflitos não serão sobrescritos.
            </Alert>
            <ul>
              {acceptedSelected.map((suggestion) => (
                <li key={suggestion.id}>{suggestion.field_label}</li>
              ))}
            </ul>
            <Inline>
              <Button disabled={apply.isPending} onClick={() => apply.mutate()}>
                {apply.isPending ? "Aplicando" : "Confirmar aplicação"}
              </Button>
              <Button disabled={apply.isPending} onClick={() => setConfirmingApply(false)}>
                Voltar à revisão
              </Button>
            </Inline>
          </Stack>
        </Surface>
      ) : null}
      {review.isError ? (
        <Alert title="Não foi possível registrar a revisão" tone="danger">
          {ocrErrorMessage(review.error)}
        </Alert>
      ) : null}
      {apply.isError ? (
        <Alert title="Não foi possível aplicar as sugestões" tone="danger">
          {ocrErrorMessage(apply.error)}
        </Alert>
      ) : null}
      {receipt ? <ApplicationReceipt receipt={receipt} /> : null}
      {suggestions.data.suggestions.length === 0 ? (
        <Alert title="Nenhum campo reconhecido" tone="info">
          O job foi concluído com segurança, mas o provedor não retornou sugestões válidas.
        </Alert>
      ) : null}
      <div className="ocr-suggestions">
        {suggestions.data.suggestions.map((view) => (
          <SuggestionCard
            draft={drafts[view.suggestion.id] ?? view.suggestion.proposed_value}
            key={view.suggestion.id}
            pending={review.isPending}
            selected={selected.has(view.suggestion.id)}
            view={view}
            onDraft={(value) =>
              setDrafts((current) => ({ ...current, [view.suggestion.id]: value }))
            }
            onSelect={(checked) =>
              setSelected((current) => {
                const next = new Set(current);
                if (checked) next.add(view.suggestion.id);
                else next.delete(view.suggestion.id);
                return next;
              })
            }
            onAccept={() =>
              review.mutate({
                suggestion: view.suggestion,
                action: "ACCEPT",
                value: drafts[view.suggestion.id] ?? view.suggestion.proposed_value,
              })
            }
            onReject={() => review.mutate({ suggestion: view.suggestion, action: "REJECT" })}
          />
        ))}
      </div>
    </Stack>
  );
}

function SuggestionCard({
  view,
  draft,
  selected,
  pending,
  onDraft,
  onSelect,
  onAccept,
  onReject,
}: {
  view: OCRSuggestionView;
  draft: string;
  selected: boolean;
  pending: boolean;
  onDraft: (value: string) => void;
  onSelect: (checked: boolean) => void;
  onAccept: () => void;
  onReject: () => void;
}) {
  const suggestion = view.suggestion;
  const immutable = suggestion.review_state === "APPLIED" || suggestion.review_state === "STALE";
  const canApply = suggestion.review_state === "ACCEPTED" && !view.stale;
  return (
    <Surface className="ocr-suggestion" tone="raised">
      <Stack gap="4">
        <Inline align="center">
          <div>
            <span className="ocr-field-key">{suggestion.field_key}</span>
            <h3>{suggestion.field_label}</h3>
          </div>
          <Inline align="center">
            {view.stale ? <StatusBadge tone="warning">Destino alterado</StatusBadge> : null}
            <StatusBadge tone={reviewTone(suggestion.review_state)}>
              {reviewStateLabel(suggestion.review_state)}
            </StatusBadge>
            {canApply ? (
              <label className="ocr-selection">
                <input
                  aria-label={`Selecionar ${suggestion.field_label}`}
                  checked={selected}
                  type="checkbox"
                  onChange={(event) => onSelect(event.target.checked)}
                />
                Aplicar
              </label>
            ) : null}
          </Inline>
        </Inline>
        <div className="ocr-comparison">
          <div>
            <span>Valor atual</span>
            <strong>{view.current_value || "—"}</strong>
          </div>
          <div>
            <span>Valor reconhecido</span>
            <strong>{suggestion.proposed_value}</strong>
          </div>
        </div>
        <label className="ocr-editor">
          Valor revisado
          {suggestion.value_kind === "LONG_TEXT" ? (
            <textarea
              disabled={immutable || pending}
              rows={4}
              value={draft}
              onChange={(event) => onDraft(event.target.value)}
            />
          ) : (
            <input
              disabled={immutable || pending}
              value={draft}
              onChange={(event) => onDraft(event.target.value)}
            />
          )}
        </label>
        <div className="ocr-evidence">
          <strong>Evidência imutável</strong>
          <p>{suggestion.evidence.excerpt || "Trecho não fornecido pelo provedor."}</p>
          <small>
            Página {suggestion.evidence.page}
            {suggestion.evidence.confidence !== undefined
              ? ` · confiança ${(suggestion.evidence.confidence / 10).toFixed(1)}%`
              : ""}
            {suggestion.evidence.region
              ? ` · região ${suggestion.evidence.region.x},${suggestion.evidence.region.y} ${suggestion.evidence.region.width}×${suggestion.evidence.region.height}`
              : ""}
          </small>
        </div>
        {!immutable ? (
          <Inline>
            <Button disabled={pending || !draft.trim()} onClick={onAccept}>
              Aceitar valor revisado
            </Button>
            <Button disabled={pending} onClick={onReject}>
              Rejeitar sugestão
            </Button>
          </Inline>
        ) : null}
      </Stack>
    </Surface>
  );
}

function ApplicationReceipt({ receipt }: { receipt: OCRApplyReceipt }) {
  const applied = receipt.results.filter((result) => result.outcome === "APPLIED").length;
  const stale = receipt.results.filter((result) => result.outcome === "STALE").length;
  const failed = receipt.results.filter((result) => result.outcome === "FAILED").length;
  return (
    <Alert title="Aplicação concluída" tone={failed || stale ? "warning" : "success"}>
      {applied} aplicada(s), {stale} desatualizada(s) e {failed} com falha. Recibo{" "}
      {shortIdentifier(receipt.id)}.
    </Alert>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function eventLabel(event: OCRJobEvent): string {
  switch (event.kind) {
    case "JOB_ACCEPTED":
      return "Extração aceita pela fila.";
    case "JOB_STARTED":
      return "Leitura privada iniciada.";
    case "SOURCE_VALIDATED":
      return `Fonte validada${event.page_count ? `: ${event.page_count} página(s)` : ""}.`;
    case "SUGGESTIONS_READY":
      return `${event.suggestion_count ?? 0} sugestão(ões) pronta(s) para revisão.`;
    case "JOB_COMPLETED":
      return "Extração concluída.";
    case "JOB_FAILED":
      return "Extração encerrada com falha segura.";
    case "JOB_CANCELLED":
      return "Extração cancelada.";
  }
}

function jobTone(job: OCRJob): "neutral" | "info" | "success" | "warning" | "danger" {
  if (job.state === "COMPLETED") return "success";
  if (job.state === "FAILED") return "danger";
  if (job.state === "CANCELLED") return "warning";
  return job.state === "RUNNING" ? "info" : "neutral";
}

function reviewTone(
  state: OCRSuggestion["review_state"],
): "neutral" | "success" | "warning" | "danger" | "info" {
  if (state === "APPLIED") return "success";
  if (state === "ACCEPTED") return "info";
  if (state === "REJECTED") return "danger";
  if (state === "STALE") return "warning";
  return "neutral";
}

function jobStateLabel(state: OCRJob["state"]): string {
  return {
    QUEUED: "Na fila",
    RUNNING: "Processando",
    COMPLETED: "Concluído",
    FAILED: "Falhou",
    CANCELLED: "Cancelado",
  }[state];
}

function reviewStateLabel(state: OCRSuggestion["review_state"]): string {
  return {
    PENDING: "Pendente",
    ACCEPTED: "Aceita",
    REJECTED: "Rejeitada",
    APPLIED: "Aplicada",
    STALE: "Desatualizada",
  }[state];
}

function mimeLabel(mime: string): string {
  return mime === "application/pdf"
    ? "PDF"
    : mime === "image/jpeg"
      ? "JPEG"
      : mime === "image/png"
        ? "PNG"
        : mime;
}

function shortIdentifier(value: string): string {
  return value.length > 13 ? `${value.slice(0, 8)}…${value.slice(-4)}` : value;
}

function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat("pt-BR").format(value);
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(
    new Date(value),
  );
}

function ocrErrorMessage(error: unknown): string {
  if (error instanceof APIRequestError) {
    if (error.code === "ocr_stale_target")
      return "O cadastro mudou. Revise novamente antes de aplicar.";
    if (error.code === "ocr_unsafe_source")
      return "O arquivo não atende aos formatos ou limites seguros do OCR.";
    if (error.status === 409)
      return "O job ou a sugestão mudou em outra operação. Atualize e tente novamente.";
    if (error.status === 429)
      return "O limite seguro de extrações foi atingido. Aguarde antes de tentar novamente.";
    if (error.status === 503) return "O runtime OCR está indisponível no momento.";
    return error.message;
  }
  return error instanceof Error ? error.message : "Erro inesperado no OCR.";
}

function asError(error: unknown): Error {
  return error instanceof Error ? error : new Error("Erro inesperado no stream OCR.");
}
