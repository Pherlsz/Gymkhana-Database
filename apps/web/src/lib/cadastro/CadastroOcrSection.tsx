import { useQuery } from "@tanstack/react-query";
import { StateCard } from "../../components/StateCard";
import { useI18n } from "../../i18n";
import { getOCRCapability } from "../api/ocr";
import { queryKeys } from "../api/queryKeys";
import type { AttachmentOwner } from "../api/attachments";
import { OcrReviewPanel } from "./OcrReviewPanel";
import { useAttachmentsEnabled } from "./useAttachmentsEnabled";

export function CadastroOcrSection({ owner }: { owner: AttachmentOwner }) {
  const { messages } = useI18n();
  const copy = messages.tables.cadastro;
  const attachmentsEnabled = useAttachmentsEnabled();
  const ocrCapability = useQuery({
    queryKey: queryKeys.ocr.capability,
    queryFn: ({ signal }) => getOCRCapability(signal),
  });
  if (attachmentsEnabled.isPending || ocrCapability.isPending) {
    return <StateCard compact kind="loading" title={messages.common.status.loading} />;
  }
  if (attachmentsEnabled.data === false) {
    return (
      <StateCard
        compact
        description={copy.ocrUnavailableR2}
        kind="warning"
        title={copy.ocrUnavailable}
      />
    );
  }
  if (!ocrCapability.data?.enabled) {
    return (
      <StateCard
        compact
        description={copy.ocrUnavailableProvider}
        kind="warning"
        title={copy.ocrUnavailable}
      />
    );
  }
  return <OcrReviewPanel owner={owner} />;
}
