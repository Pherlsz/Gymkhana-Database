import {
  ArrowUp,
  Maximize2,
  Minimize2,
  Minus,
  PanelLeftClose,
  PanelLeftOpen,
  Pencil,
  Plus,
  Sparkles,
  Square,
  Trash2,
} from "lucide-react";
import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
} from "react";
import { ICON, ICON_STROKE } from "../../components/icons";
import { useI18n } from "../../i18n";
import { AssistantMarkdown } from "./assistantMarkdown";
import { useAssistantChat } from "./useAssistantChat";
import "./assistant.css";

export type AssistantWindowSize = "min" | "float" | "wide";

function CollapsedCopy({
  text,
  moreLabel,
  lessLabel,
}: {
  text: string;
  moreLabel: string;
  lessLabel: string;
}) {
  const clipRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [overflows, setOverflows] = useState(false);

  useLayoutEffect(() => {
    const node = clipRef.current;
    if (!node || open) return;
    setOverflows(node.scrollHeight > node.clientHeight + 1);
  }, [text, open]);

  return (
    <div className="assistant-msg__copy">
      <div className={open ? "assistant-msg__clip is-open" : "assistant-msg__clip"} ref={clipRef}>
        <AssistantMarkdown text={text} />
      </div>
      {overflows ? (
        <button
          className="assistant-msg__more"
          type="button"
          onClick={() => setOpen((current) => !current)}
        >
          {open ? lessLabel : moreLabel}
        </button>
      ) : null}
    </div>
  );
}

function ResultOnTableAction({
  referenceId,
  activeResultId,
  applyLabel,
  onTableLabel,
  onShow,
}: {
  referenceId: string;
  activeResultId?: string | undefined;
  applyLabel: string;
  onTableLabel: string;
  onShow: (id: string) => void;
}) {
  if (referenceId === activeResultId) {
    return <p className="assistant-msg__on-table">{onTableLabel}</p>;
  }
  return (
    <button className="assistant-msg__apply" type="button" onClick={() => onShow(referenceId)}>
      {applyLabel}
    </button>
  );
}

export function AssistantSessionFloat({
  size,
  tableLabel,
  activeResultId,
  onSizeChange,
  onShowResult,
}: {
  size: AssistantWindowSize;
  tableLabel: string;
  activeResultId?: string | undefined;
  onSizeChange: (size: AssistantWindowSize) => void;
  onShowResult?: ((referenceId: string) => void) | undefined;
}) {
  const { messages, t } = useI18n();
  const copy = messages.tables.assistant;
  const chat = useAssistantChat(size !== "min");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState("");
  const [draft, setDraft] = useState("");
  const [sessionsOpen, setSessionsOpen] = useState(true);
  const chatRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const seenThread = useRef<string | null>(null);
  const seenResultReference = useRef("");
  const pendingText = chat.pending?.text ?? "";

  useEffect(() => {
    if (!onShowResult || chat.messagesLoading) return;
    const threadId = chat.thread?.id ?? "";
    const latest = [...chat.messages]
      .toReversed()
      .find((message) => message.role === "ASSISTANT" && message.result_reference_ids.length > 0);
    if (seenThread.current !== threadId) {
      seenThread.current = threadId;
      seenResultReference.current =
        latest?.result_reference_ids[latest.result_reference_ids.length - 1] ?? "";
      return;
    }
    if (!latest || chat.pending) return;
    const referenceId = latest.result_reference_ids[latest.result_reference_ids.length - 1];
    if (!referenceId || referenceId === seenResultReference.current) return;
    seenResultReference.current = referenceId;
    onShowResult(referenceId);
  }, [chat.messages, chat.messagesLoading, chat.pending, chat.thread?.id, onShowResult]);

  useLayoutEffect(() => {
    const node = inputRef.current;
    if (!node) return;
    node.style.height = "auto";
    const style = getComputedStyle(node);
    const line = Number.parseFloat(style.lineHeight) || 20;
    const pad =
      (Number.parseFloat(style.paddingTop) || 0) + (Number.parseFloat(style.paddingBottom) || 0);
    node.style.height = `${Math.min(node.scrollHeight, line * 5 + pad)}px`;
  }, [draft]);

  useEffect(() => {
    const node = chatRef.current;
    if (node) node.scrollTop = node.scrollHeight;
  }, [chat.messages.length, pendingText, chat.thread?.id]);

  if (size === "min") return null;

  const chatReady = chat.chatReady;
  const composerLocked = !chatReady || chat.busy;
  const showComposer = chatReady || chat.capabilityLoading;
  const showUnavailable =
    chat.capabilityFailed || (Boolean(chat.capability) && !chat.capability?.enabled);
  const title = chat.thread?.title ?? copy.assistant;

  const commitRename = () => {
    if (editingId) chat.renameThread(editingId, editingTitle);
    setEditingId(null);
  };

  const sendDraft = (event?: FormEvent) => {
    event?.preventDefault();
    const text = draft.trim();
    if (!text || composerLocked) return;
    chat.send(text);
    setDraft("");
  };

  const onComposerKey = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      sendDraft();
    }
  };

  const errorText = (code: string | null | undefined) => {
    if (!code) return null;
    const key = code.replace(/^chat_/, "") as keyof typeof copy.errors;
    return copy.errors[key] ?? copy.errors.unknown;
  };

  return (
    <div
      aria-label={copy.sessions}
      className={[
        "assistant-float",
        size === "wide" ? "is-wide" : "is-float",
        sessionsOpen ? "" : "is-sessions-collapsed",
      ]
        .filter(Boolean)
        .join(" ")}
      role="dialog"
      tabIndex={-1}
      onKeyDown={(event) => {
        if (event.key === "Escape" && !editingId) onSizeChange("min");
      }}
    >
      {sessionsOpen ? (
        <nav aria-label={copy.sessionsList} className="assistant-float__sessions">
          <div className="assistant-float__sessions-head">
            <p className="assistant-float__sessions-label">{copy.sessionsList}</p>
            <button
              aria-expanded={true}
              aria-label={copy.collapseSessions}
              className="assistant-float__icon"
              title={copy.collapseSessions}
              type="button"
              onClick={() => setSessionsOpen(false)}
            >
              <PanelLeftClose aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
            </button>
          </div>
          {chat.threadsLoading ? (
            <div aria-busy="true" aria-live="polite" className="assistant-float__session-skel">
              <span className="visually-hidden">{copy.loadingSessions}</span>
              <span aria-hidden className="assistant-float__skel-row" />
              <span aria-hidden className="assistant-float__skel-row" />
              <span aria-hidden className="assistant-float__skel-row is-short" />
            </div>
          ) : null}
          {chat.threads.map((item) => {
            const active = item.id === chat.thread?.id;
            const editing = editingId === item.id;
            return (
              <div
                className={
                  active
                    ? editing
                      ? "assistant-float__session-row is-active is-editing"
                      : "assistant-float__session-row is-active"
                    : "assistant-float__session-row"
                }
                key={item.id}
              >
                {editing ? (
                  <input
                    aria-label={copy.renameSession}
                    autoFocus
                    className="assistant-float__rename"
                    maxLength={120}
                    value={editingTitle}
                    onBlur={commitRename}
                    onChange={(event) => setEditingTitle(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") commitRename();
                      if (event.key === "Escape") setEditingId(null);
                    }}
                  />
                ) : (
                  <button
                    className="assistant-float__session"
                    type="button"
                    onClick={() => chat.selectThread(item.id)}
                  >
                    {item.title}
                  </button>
                )}
                <span className="assistant-float__session-tools">
                  <button
                    aria-label={copy.renameSession}
                    className="assistant-float__icon"
                    type="button"
                    onClick={() => {
                      chat.selectThread(item.id);
                      setEditingTitle(item.title);
                      setEditingId(item.id);
                    }}
                  >
                    <Pencil aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
                  </button>
                  <button
                    aria-label={copy.deleteSession}
                    className="assistant-float__icon is-danger"
                    disabled={chat.busy && chat.pending?.threadId === item.id}
                    type="button"
                    onClick={() => chat.deleteThread(item.id)}
                  >
                    <Trash2 aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
                  </button>
                </span>
              </div>
            );
          })}
          <button
            className="assistant-float__create"
            disabled={!chatReady || chat.busy}
            type="button"
            onClick={() => {
              void chat.createThread().catch(() => undefined);
            }}
          >
            <Plus aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
            {copy.createSession}
          </button>
          {chat.sessionError ? (
            <p className="assistant-float__notice" role="alert">
              {errorText(chat.sessionError)}
            </p>
          ) : null}
        </nav>
      ) : null}
      <div className="assistant-float__pane">
        <header className="assistant-float__bar">
          {sessionsOpen ? null : (
            <button
              aria-expanded={false}
              aria-label={copy.expandSessions}
              className="assistant-float__icon"
              title={copy.expandSessions}
              type="button"
              onClick={() => setSessionsOpen(true)}
            >
              <PanelLeftOpen aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
            </button>
          )}
          <Sparkles
            aria-hidden
            className="assistant-float__mark"
            size={ICON.sm}
            strokeWidth={ICON_STROKE}
          />
          <div className="assistant-float__identity">
            <p className="assistant-float__title">{title}</p>
            <p className="assistant-float__scope">{t(copy.tableScope, { table: tableLabel })}</p>
          </div>
          <div className="assistant-float__tools">
            <button
              aria-label={size === "wide" ? copy.collapseWindow : copy.expand}
              className="assistant-float__icon"
              type="button"
              onClick={() => onSizeChange(size === "wide" ? "float" : "wide")}
            >
              {size === "wide" ? (
                <Minimize2 aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
              ) : (
                <Maximize2 aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
              )}
            </button>
            <button
              aria-label={copy.minimize}
              className="assistant-float__icon"
              type="button"
              onClick={() => onSizeChange("min")}
            >
              <Minus aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
            </button>
          </div>
        </header>
        <div className="assistant-float__chat" ref={chatRef}>
          {chat.messagesLoading ? (
            <div aria-busy="true" aria-live="polite" className="assistant-float__history-skel">
              <span className="visually-hidden">{copy.loadingHistory}</span>
              <span aria-hidden className="assistant-float__skel-bubble is-user" />
              <span aria-hidden className="assistant-float__skel-bubble is-assistant" />
              <span aria-hidden className="assistant-float__skel-bubble is-user is-short" />
            </div>
          ) : null}
          {chat.messagesRefreshing ? (
            <p aria-live="polite" className="assistant-float__refresh">
              {copy.loadingHistory}
            </p>
          ) : null}
          {!chat.messagesLoading && chat.messages.length === 0 && !chat.pending ? (
            <p className="assistant-float__empty">{copy.emptyThread}</p>
          ) : null}
          {!chat.messagesLoading
            ? chat.messages.map((message) => (
                <article
                  className={
                    message.role === "USER" ? "assistant-msg is-user" : "assistant-msg is-assistant"
                  }
                  key={message.id}
                >
                  {message.role === "ASSISTANT" ? (
                    <Sparkles
                      aria-hidden
                      className="assistant-msg__mark"
                      size={ICON.sm}
                      strokeWidth={ICON_STROKE}
                    />
                  ) : null}
                  <div className="assistant-msg__body">
                    <CollapsedCopy
                      lessLabel={copy.showLess}
                      moreLabel={copy.showMore}
                      text={message.content}
                    />
                    {message.role === "ASSISTANT" &&
                    onShowResult &&
                    message.result_reference_ids.length > 0 ? (
                      <ResultOnTableAction
                        activeResultId={activeResultId}
                        applyLabel={copy.applyToTable}
                        onShow={onShowResult}
                        onTableLabel={copy.onTable}
                        referenceId={
                          message.result_reference_ids[message.result_reference_ids.length - 1]!
                        }
                      />
                    ) : null}
                  </div>
                </article>
              ))
            : null}
          {chat.pending ? (
            <article aria-live="polite" className="assistant-msg is-assistant is-pending">
              <Sparkles
                aria-hidden
                className="assistant-msg__mark"
                size={ICON.sm}
                strokeWidth={ICON_STROKE}
              />
              <div className="assistant-msg__body">
                {chat.pending.text ? (
                  <div className="assistant-msg__copy">
                    <AssistantMarkdown text={chat.pending.text} />
                  </div>
                ) : null}
                {chat.pending.cancelled ? (
                  <p className="assistant-msg__status" role="status">
                    {copy.errors.cancelled}
                  </p>
                ) : chat.pending.errorCode ? (
                  <p className="assistant-msg__status is-error" role="alert">
                    {errorText(chat.pending.errorCode)}
                    {" · "}
                    <button className="assistant-msg__more" type="button" onClick={chat.retry}>
                      {copy.retry}
                    </button>
                    {" · "}
                    <button
                      className="assistant-msg__more"
                      type="button"
                      onClick={chat.dismissError}
                    >
                      {copy.dismiss}
                    </button>
                  </p>
                ) : (
                  <p className="assistant-msg__status">
                    {chat.pending.toolRunning
                      ? copy.consulting
                      : chat.pending.text
                        ? ""
                        : copy.thinking}
                  </p>
                )}
              </div>
            </article>
          ) : null}
          {chat.sendError ? (
            <p className="assistant-msg__status is-error" role="alert">
              {errorText(chat.sendError)}
            </p>
          ) : null}
        </div>
        {showComposer ? (
          <form
            aria-busy={composerLocked || undefined}
            className={composerLocked ? "assistant-composer is-busy" : "assistant-composer"}
            onSubmit={sendDraft}
          >
            <label className="visually-hidden" htmlFor="assistant-next">
              {chat.busy
                ? copy.thinking
                : chat.capabilityLoading
                  ? copy.preparing
                  : chat.thread
                    ? copy.placeholder
                    : copy.placeholderNew}
            </label>
            <textarea
              aria-disabled={composerLocked || undefined}
              className="assistant-composer__input"
              disabled={composerLocked}
              id="assistant-next"
              maxLength={chat.capability?.maximum_message_runes ?? 20000}
              placeholder={
                chat.busy
                  ? copy.thinking
                  : chat.capabilityLoading
                    ? copy.preparing
                    : chat.thread
                      ? copy.placeholder
                      : copy.placeholderNew
              }
              ref={inputRef}
              rows={1}
              value={composerLocked ? "" : draft}
              onChange={(event) => setDraft(event.target.value)}
              onKeyDown={onComposerKey}
            />
            {chat.pending && !chat.pending.errorCode && !chat.pending.cancelled ? (
              <button
                aria-label={copy.stop}
                className="assistant-composer__send"
                title={copy.stop}
                type="button"
                onClick={chat.cancel}
              >
                <Square aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
              </button>
            ) : (
              <button
                aria-label={copy.send}
                className="assistant-composer__send"
                disabled={composerLocked || !draft.trim()}
                type="submit"
              >
                <ArrowUp aria-hidden size={ICON.sm} strokeWidth={ICON_STROKE} />
              </button>
            )}
          </form>
        ) : showUnavailable ? (
          <p className="assistant-float__notice" role="status">
            {copy.unavailable}
          </p>
        ) : null}
      </div>
    </div>
  );
}
