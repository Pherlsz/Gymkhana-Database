import { Button, Spin } from "antd";
import type { ReactNode } from "react";

export interface CadastroStickyBarProps {
  summaryText?: ReactNode;
  saving: boolean;
  onCancel: () => void;
  onSave: () => void;
  saveButtonText: string;
  cancelButtonText: string;
}

export function CadastroStickyBar({
  summaryText,
  saving,
  onCancel,
  onSave,
  saveButtonText,
  cancelButtonText,
}: CadastroStickyBarProps) {
  return (
    <footer className="sticky-actions">
      <div className="sticky-actions__summary">{summaryText}</div>
      <div className="sticky-actions__buttons">
        <Button disabled={saving} onClick={onCancel}>
          {cancelButtonText}
        </Button>
        <Button
          disabled={saving}
          icon={saving ? <Spin size="small" /> : undefined}
          type="primary"
          onClick={onSave}
        >
          {saveButtonText}
        </Button>
      </div>
    </footer>
  );
}
