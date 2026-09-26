import { Button, Modal } from "antd";
import { ArrowLeft } from "lucide-react";
import type { ReactNode } from "react";
import { QueryView, type QueryViewProps } from "../../components/QueryView";
import { StateCard, type StateCardProps } from "../../components/StateCard";

export function CadastroWorkShell({
  backLabel,
  current,
  onBack,
  children,
}: {
  backLabel: string;
  current?: string | undefined;
  onBack: () => void;
  children: ReactNode;
}) {
  return (
    <div className="cadastro-work">
      <nav aria-label={backLabel} className="cadastro-crumb">
        <Button
          className="cadastro-crumb__btn"
          type="text"
          size="small"
          icon={<ArrowLeft size={13} />}
          onClick={onBack}
        >
          {backLabel}
        </Button>
        {current ? (
          <>
            <span className="cadastro-crumb__sep">/</span>
            <span className="cadastro-crumb__current">{current}</span>
          </>
        ) : null}
      </nav>
      {children}
    </div>
  );
}

export function CadastroWorkState(props: StateCardProps) {
  return (
    <div className="cadastro-work__state">
      <StateCard compact {...props} />
    </div>
  );
}

export function CadastroWorkQuery(props: Omit<QueryViewProps, "compact">) {
  return (
    <div className="cadastro-work__state">
      <QueryView compact {...props} />
    </div>
  );
}

export function confirmCadastroAction({
  title,
  content,
  okText,
  cancelText,
}: {
  title: string;
  content: string;
  okText?: string;
  cancelText?: string;
}): Promise<boolean> {
  return new Promise((resolve) => {
    Modal.confirm({
      title,
      content,
      okText,
      cancelText,
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    });
  });
}
