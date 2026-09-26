import type { ReactNode } from "react";
import { useI18n } from "../i18n";
import { errorMessage } from "../lib/formatters";
import { StateCard } from "./StateCard";

export interface QueryViewProps {
  isPending?: boolean | undefined;
  isError?: boolean | undefined;
  error?: unknown;
  errorTitle?: ReactNode | undefined;
  errorDescription?: ReactNode | undefined;
  onRetry?: (() => void) | undefined;
  retryLabel?: ReactNode | undefined;
  empty?: boolean | undefined;
  emptyTitle?: ReactNode | undefined;
  emptyDescription?: ReactNode | undefined;
  compact?: boolean | undefined;
  loadingTitle?: ReactNode | undefined;
  loadingDescription?: ReactNode | undefined;
  children?: ReactNode | undefined;
}

export function QueryView({
  isPending = false,
  isError = false,
  error,
  errorTitle,
  errorDescription,
  onRetry,
  retryLabel,
  empty = false,
  emptyTitle,
  emptyDescription,
  compact = false,
  loadingTitle,
  loadingDescription,
  children,
}: QueryViewProps) {
  const { messages } = useI18n();

  if (isPending) {
    return (
      <StateCard
        compact={compact}
        description={loadingDescription}
        kind="loading"
        title={loadingTitle ?? messages.common.status.loading}
      />
    );
  }

  if (isError) {
    return (
      <StateCard
        compact={compact}
        description={errorDescription ?? (error !== undefined ? errorMessage(error) : undefined)}
        kind="error"
        retryLabel={retryLabel}
        title={errorTitle ?? messages.common.status.error}
        onRetry={onRetry}
      />
    );
  }

  if (empty) {
    return (
      <StateCard
        compact={compact}
        description={emptyDescription}
        kind="empty"
        title={emptyTitle ?? messages.common.status.empty}
      />
    );
  }

  return <>{children}</>;
}
