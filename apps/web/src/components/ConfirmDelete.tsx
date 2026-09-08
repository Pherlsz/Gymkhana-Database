import { Alert, Button, Flex, Input, Typography } from "antd";
import { useId, useState } from "react";

export function ConfirmDelete({
  title,
  description,
  confirmationLabel,
  confirmationWord,
  confirmLabel,
  cancelLabel,
  pending = false,
  error,
  onConfirm,
  onCancel,
}: {
  title: string;
  description: string;
  confirmationLabel: string;
  confirmationWord: string;
  confirmLabel: string;
  cancelLabel: string;
  pending?: boolean;
  error?: string | undefined;
  onConfirm: (confirmation: string) => void;
  onCancel: () => void;
}) {
  const [confirmation, setConfirmation] = useState("");
  const inputId = useId();
  const matches = confirmation === confirmationWord;

  return (
    <section className="confirm-delete">
      <Flex gap="0.75rem" vertical>
        <Typography.Text strong>{title}</Typography.Text>
        <Typography.Text type="secondary">{description}</Typography.Text>
        <label className="confirm-delete__field" htmlFor={inputId}>
          <span>{confirmationLabel}</span>
          <Input
            aria-required="true"
            autoComplete="off"
            id={inputId}
            onChange={(event) => setConfirmation(event.target.value)}
            placeholder={confirmationWord}
            value={confirmation}
          />
        </label>
        {error ? <Alert message={error} showIcon type="error" /> : null}
        <Flex gap="0.5rem" wrap>
          <Button
            danger
            disabled={!matches || pending}
            loading={pending}
            onClick={() => onConfirm(confirmation)}
            type="primary"
          >
            {confirmLabel}
          </Button>
          <Button disabled={pending} onClick={onCancel}>
            {cancelLabel}
          </Button>
        </Flex>
      </Flex>
    </section>
  );
}
