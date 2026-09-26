import { Button, Spin } from "antd";
import type { ReactNode } from "react";

export interface CadastroStickyBarProps {
  summaryText?: ReactNode;
  summaryBlocked?: boolean | undefined;
  saving: boolean;
  saveDisabled?: boolean | undefined;
  saveHint?: string | undefined;
  onCancel: () => void;
  onSave: () => void;
  saveButtonText: string;
  cancelButtonText: string;
}

export function CadastroStickyBar({
  summaryText,
  summaryBlocked,
  saving,
  saveDisabled,
  saveHint,
  onCancel,
  onSave,
  saveButtonText,
  cancelButtonText,
}: CadastroStickyBarProps) {
  const blocked = Boolean(saveDisabled);
  return (
    <footer className="sticky-actions">
      <div
        className={`sticky-actions__summary${summaryBlocked ? " sticky-actions__summary--blocked" : ""}`}
        id="cadastro-sticky-hint"
      >
        {summaryText}
      </div>
      <div className="sticky-actions__buttons">
        <Button disabled={saving} onClick={onCancel}>
          {cancelButtonText}
        </Button>
        <span title={blocked ? saveHint : undefined}>
          <Button
            aria-describedby="cadastro-sticky-hint"
            disabled={saving || blocked}
            icon={saving ? <Spin size="small" /> : undefined}
            type="primary"
            onClick={onSave}
          >
            {saveButtonText}
          </Button>
        </span>
      </div>
    </footer>
  );
}
