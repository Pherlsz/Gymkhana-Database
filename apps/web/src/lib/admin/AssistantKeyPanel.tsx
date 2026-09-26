import { Button, Input, Popconfirm, Tag } from "antd";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { QueryView } from "../../components/QueryView";
import { StatusBanner } from "../../components/StatusBanner";
import { useI18n } from "../../i18n";
import { clearModelKey, getModelKeyStatus, setModelKey, type ModelProvider } from "../api/client";
import { formatDateTime } from "../formatters";

const PROVIDER: ModelProvider = "google";
const DEFAULT_MODEL = "gemini-2.5-flash";
const QUERY_KEY = ["admin", "model-key", PROVIDER] as const;

export function AssistantKeyPanel() {
  const { messages } = useI18n();
  const copy = messages.admin.modelKey;
  const queryClient = useQueryClient();
  const [secret, setSecret] = useState("");
  const [model, setModel] = useState(DEFAULT_MODEL);
  const [rotating, setRotating] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const status = useQuery({
    queryKey: QUERY_KEY,
    queryFn: ({ signal }) => getModelKeyStatus(PROVIDER, signal),
  });

  const configured = status.data?.configured ?? false;
  const storedModel = status.data?.model || DEFAULT_MODEL;
  const showForm = !configured || rotating;
  const draftModel = model.trim() || storedModel;

  const beginRotate = () => {
    setModel(storedModel);
    setSecret("");
    setNotice(null);
    setRotating(true);
  };

  const cancelRotate = () => {
    setSecret("");
    setModel(storedModel);
    setRotating(false);
  };

  const save = useMutation({
    mutationFn: () => setModelKey(PROVIDER, { secret: secret.trim(), model: draftModel }),
    onSuccess: (next) => {
      queryClient.setQueryData(QUERY_KEY, next);
      setSecret("");
      setModel(next.model || DEFAULT_MODEL);
      setRotating(false);
      setNotice(copy.saved);
    },
  });
  const remove = useMutation({
    mutationFn: () => clearModelKey(PROVIDER),
    onSuccess: () => {
      queryClient.setQueryData(QUERY_KEY, { provider: PROVIDER, configured: false });
      setSecret("");
      setModel(DEFAULT_MODEL);
      setRotating(false);
      setNotice(copy.removed);
    },
  });

  const canSave = secret.trim().length >= 20 && draftModel.length > 0;
  const error = save.error ?? remove.error;

  return (
    <section className="admin-form">
      <header className="admin-form__header">
        <h2 className="admin-form__title">{copy.title}</h2>
        {status.isSuccess ? (
          <Tag color={configured ? "success" : "default"}>
            {configured ? copy.statusOn : copy.statusOff}
          </Tag>
        ) : null}
      </header>
      {status.isPending || status.isError ? (
        <QueryView
          compact
          error={status.error}
          errorTitle={copy.loadError}
          isError={status.isError}
          isPending={status.isPending}
          onRetry={() => void status.refetch()}
        />
      ) : null}
      {status.isSuccess ? (
        <>
          <p className="admin-form__status">
            {copy.providerGoogle}
            {configured && status.data?.updated_at
              ? ` · ${formatDateTime(status.data.updated_at)}`
              : ` · ${copy.notConfigured}`}
          </p>
          {configured && !rotating ? (
            <dl className="admin-form__facts">
              <dt>{copy.modelLabel}</dt>
              <dd className="admin-form__value">{storedModel}</dd>
            </dl>
          ) : (
            <div className="admin-form__grid">
              <label className="admin-field" htmlFor="admin-model-name">
                <span>{copy.modelLabel}</span>
                <Input
                  autoComplete="off"
                  id="admin-model-name"
                  maxLength={120}
                  name="model"
                  value={model}
                  onChange={(event) => setModel(event.target.value)}
                />
              </label>
              <label className="admin-field" htmlFor="admin-model-secret">
                <span>{copy.secretLabel}</span>
                <Input.Password
                  autoComplete="new-password"
                  id="admin-model-secret"
                  maxLength={512}
                  name="secret"
                  placeholder={copy.secretPlaceholder}
                  value={secret}
                  onChange={(event) => setSecret(event.target.value)}
                />
              </label>
            </div>
          )}
          {showForm ? <p className="admin-form__status">{copy.secretHint}</p> : null}
          {error ? <StatusBanner error={error} title={copy.saveError} /> : null}
          {notice && !error ? <StatusBanner title={notice} tone="success" /> : null}
          <div className="admin-form__actions">
            {showForm ? (
              <Button
                disabled={!canSave}
                loading={save.isPending}
                type="primary"
                onClick={() => save.mutate()}
              >
                {copy.save}
              </Button>
            ) : (
              <Button type="primary" onClick={beginRotate}>
                {copy.rotate}
              </Button>
            )}
            {rotating ? <Button onClick={cancelRotate}>{copy.cancel}</Button> : null}
            <Popconfirm
              cancelText={copy.cancel}
              description={copy.removeConfirm}
              disabled={!configured}
              okButtonProps={{ danger: true }}
              okText={copy.remove}
              title={copy.remove}
              onConfirm={() => remove.mutate()}
            >
              <Button danger disabled={!configured} loading={remove.isPending}>
                {copy.remove}
              </Button>
            </Popconfirm>
          </div>
        </>
      ) : null}
    </section>
  );
}
