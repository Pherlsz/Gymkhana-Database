function toneToType(tone: string): "info" | "success" | "warning" | "error" {
  return tone === "danger" ? "error" : (tone as any);
}

import { Alert, Button, Card, Flex, Layout, Tag, Typography } from "antd";
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
    <Layout style={{ maxWidth: "full", margin: "0 auto" }}>
      <header className="page-header">
        <div className="page-eyebrow">Extração privada · revisão humana obrigatória</div>
        <Typography.Title level={1} className="page-title">
          OCR de anexos
        </Typography.Title>
        <Typography.Paragraph className="page-description">
          Extraia campos de PDF, JPEG ou PNG, confira evidências e aplique somente as sugestões
          aprovadas explicitamente.
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex vertical gap="1.25rem">
          <Alert
            message="Nenhuma alteração é automática"
            type="info"
            description="A extração apenas cria sugestões. Aceitar uma sugestão ainda não altera o cadastro; a
            aplicação é uma segunda ação explícita e protegida por versão."
          />
          <Card className="ocr-start" style={{ padding: "1rem" }}>
            <Flex vertical gap="0.75rem">
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
              <Flex>
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
              </Flex>
            </Flex>
          </Card>
          {start.isError ? (
            <Alert
              message="Não foi possível iniciar a extração"
              type="error"
              description={<>{ocrErrorMessage(start.error)}</>}
            />
          ) : null}
          {jobs.isError ? (
            <Alert
              message="Não foi possível listar as extrações"
              type="error"
              description={<>{ocrErrorMessage(jobs.error)}</>}
            />
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
                <Card className="ocr-empty" style={{ padding: "1rem" }}>
                  <Alert
                    message="Não foi possível abrir a extração"
                    type="error"
                    description={<>{ocrErrorMessage(selectedJob.error)}</>}
                  />
                </Card>
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
                <Card className="ocr-empty" style={{ padding: "1rem" }}>
                  <p role="status">Carregando extração…</p>
                </Card>
              )
            ) : (
              <Card className="ocr-empty" style={{ padding: "1rem" }}>
                <Flex vertical gap="0.5rem">
                  <h2>Selecione uma extração</h2>
                  <p>Os jobs e suas evidências são privados para o usuário autenticado.</p>
                </Flex>
              </Card>
            )}
          </div>
        </Flex>
      </div>
    </Layout>
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
    <Layout style={{ maxWidth: "lg", margin: "0 auto" }}>
      <header className="page-header">
        <div className="page-eyebrow">Extração privada</div>
        <Typography.Title level={1} className="page-title">
          OCR de anexos
        </Typography.Title>
      </header>
      <div className="page-content">
        <Alert title={title} type={toneToType(tone)} description={<>{children}</>} />
      </div>
    </Layout>
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
    <Card className="ocr-jobs" style={{ padding: "1rem" }}>
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
            <Tag color={jobTone(job)}>{jobStateLabel(job.state)}</Tag>
          </button>
        ))}
      </nav>
    </Card>
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
      <Card className="ocr-job-summary" style={{ padding: "1rem" }}>
        <Flex vertical gap="0.75rem">
          <Flex align="center">
            <div>
              <h2>Extração {shortIdentifier(job.id)}</h2>
              <p className="ocr-muted">
                Anexo {shortIdentifier(job.attachment_id)} · {mimeLabel(job.source_mime)} ·{" "}
                {formatBytes(job.source_bytes)}
              </p>
            </div>
            <Tag color={jobTone(job)}>{jobStateLabel(job.state)}</Tag>
          </Flex>
          <div className="ocr-job-metrics">
            <Metric label="Tentativa" value={`${job.attempt_count}/3`} />
            <Metric label="Páginas" value={String(job.page_count)} />
            <Metric label="Sugestões" value={String(job.suggestion_count)} />
            <Metric label="Uso do provedor" value={formatNumber(job.provider_usage)} />
            <Metric label="Atualizado" value={formatDateTime(job.updated_at)} />
          </div>
          {isActiveOCRJob(job) ? <p aria-live="polite">{activity}</p> : null}
          {job.error_code ? (
            <Alert
              message="Extração encerrada"
              type="error"
              description={<>Código seguro: {job.error_code}</>}
            />
          ) : null}
          {streamError ? (
            <Alert
              message="Acompanhamento em tempo real interrompido"
              type="warning"
              description={
                <>{ocrErrorMessage(streamError)} Atualize o job para consultar o estado durável.</>
              }
            />
          ) : null}
          {cancel.isError ? (
            <Alert
              message="Não foi possível cancelar"
              type="error"
              description={<>{ocrErrorMessage(cancel.error)}</>}
            />
          ) : null}
          {download.isError ? (
            <Alert
              message="Não foi possível abrir o anexo"
              type="error"
              description={<>{ocrErrorMessage(download.error)}</>}
            />
          ) : null}
          <Flex>
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
          </Flex>
        </Flex>
      </Card>
      {job.state === "COMPLETED" ? <SuggestionReview job={job} /> : null}
      {job.state === "QUEUED" || job.state === "RUNNING" ? (
        <Card className="ocr-empty" style={{ padding: "1rem" }}>
          <Flex vertical gap="0.5rem">
            <h2>Processando com limites seguros</h2>
            <p>O arquivo permanece privado e as sugestões aparecerão após a validação completa.</p>
          </Flex>
        </Card>
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
      <Card className="ocr-empty" style={{ padding: "1rem" }}>
        <p role="status">Carregando sugestões e versões atuais…</p>
      </Card>
    );
  }
  if (suggestions.isError) {
    return (
      <Alert
        message="Não foi possível carregar as sugestões"
        type="error"
        description={<>{ocrErrorMessage(suggestions.error)}</>}
      />
    );
  }

  return (
    <Flex vertical gap="1rem">
      <Card className="ocr-apply-bar" style={{ padding: "1rem" }}>
        <Flex align="center">
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
        </Flex>
      </Card>
      {confirmingApply ? (
        <Card className="ocr-confirm" style={{ padding: "1rem" }}>
          <Flex vertical gap="0.75rem">
            <Alert
              message="Confirmar alteração dos cadastros"
              type="warning"
              description={
                <>
                  Esta ação aplicará {acceptedSelected.length} valor(es) já revisado(s) usando as
                  versões atuais. Conflitos não serão sobrescritos.
                </>
              }
            />
            <ul>
              {acceptedSelected.map((suggestion) => (
                <li key={suggestion.id}>{suggestion.field_label}</li>
              ))}
            </ul>
            <Flex>
              <Button disabled={apply.isPending} onClick={() => apply.mutate()}>
                {apply.isPending ? "Aplicando" : "Confirmar aplicação"}
              </Button>
              <Button disabled={apply.isPending} onClick={() => setConfirmingApply(false)}>
                Voltar à revisão
              </Button>
            </Flex>
          </Flex>
        </Card>
      ) : null}
      {review.isError ? (
        <Alert
          message="Não foi possível registrar a revisão"
          type="error"
          description={<>{ocrErrorMessage(review.error)}</>}
        />
      ) : null}
      {apply.isError ? (
        <Alert
          message="Não foi possível aplicar as sugestões"
          type="error"
          description={<>{ocrErrorMessage(apply.error)}</>}
        />
      ) : null}
      {receipt ? <ApplicationReceipt receipt={receipt} /> : null}
      {suggestions.data.suggestions.length === 0 ? (
        <Alert
          message="Nenhum campo reconhecido"
          type="info"
          description="O job foi concluído com segurança, mas o provedor não retornou sugestões válidas."
        />
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
    </Flex>
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
    <Card className="ocr-suggestion" style={{ padding: "1rem" }}>
      <Flex vertical gap="1rem">
        <Flex align="center">
          <div>
            <span className="ocr-field-key">{suggestion.field_key}</span>
            <h3>{suggestion.field_label}</h3>
          </div>
          <Flex align="center">
            {view.stale ? <Tag color="warning">Destino alterado</Tag> : null}
            <Tag color={reviewTone(suggestion.review_state)}>
              {reviewStateLabel(suggestion.review_state)}
            </Tag>
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
          </Flex>
        </Flex>
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
          <Flex>
            <Button disabled={pending || !draft.trim()} onClick={onAccept}>
              Aceitar valor revisado
            </Button>
            <Button disabled={pending} onClick={onReject}>
              Rejeitar sugestão
            </Button>
          </Flex>
        ) : null}
      </Flex>
    </Card>
  );
}

function ApplicationReceipt({ receipt }: { receipt: OCRApplyReceipt }) {
  const applied = receipt.results.filter((result) => result.outcome === "APPLIED").length;
  const stale = receipt.results.filter((result) => result.outcome === "STALE").length;
  const failed = receipt.results.filter((result) => result.outcome === "FAILED").length;
  return (
    <Alert
      title="Aplicação concluída"
      type={toneToType(failed || stale ? "warning" : "success")}
      description={
        <>
          {applied} aplicada(s), {stale} desatualizada(s) e {failed} com falha. Recibo{" "}
          {shortIdentifier(receipt.id)}.
        </>
      }
    />
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
