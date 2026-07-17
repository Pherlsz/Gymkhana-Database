import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { AttachmentsPanel } from "./AttachmentsPanel";
import type { AttachmentOwner } from "./lib/api/attachments";

const attachmentQueryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 15_000, retry: 1 } },
});

export function ProfileAttachmentsBridge() {
  return (
    <QueryClientProvider client={attachmentQueryClient}>
      <ProfileAttachmentsPortal />
    </QueryClientProvider>
  );
}

function ProfileAttachmentsPortal() {
  const location = useBrowserLocation();
  const [target, setTarget] = useState<HTMLElement | null>(null);
  const selection = useMemo(() => attachmentSelection(location), [location]);

  useEffect(() => {
    if (!selection) {
      setTarget(null);
      return;
    }
    const findTarget = () => {
      const panel = document.querySelector<HTMLElement>(".profile-panel");
      setTarget(panel);
    };
    findTarget();
    const observer = new MutationObserver(findTarget);
    observer.observe(document.body, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, [selection]);

  if (!selection || !target) return null;
  return createPortal(
    <AttachmentsPanel
      description={selection.description}
      owner={selection.owner}
      title={selection.title}
    />,
    target,
  );
}

function useBrowserLocation() {
  const [value, setValue] = useState(() => window.location.href);
  useEffect(() => {
    const refresh = () => setValue(window.location.href);
    const originalPush = window.history.pushState;
    const originalReplace = window.history.replaceState;
    window.history.pushState = function (...arguments_) {
      originalPush.apply(this, arguments_);
      window.dispatchEvent(new Event("gymkhana-location"));
    };
    window.history.replaceState = function (...arguments_) {
      originalReplace.apply(this, arguments_);
      window.dispatchEvent(new Event("gymkhana-location"));
    };
    window.addEventListener("popstate", refresh);
    window.addEventListener("gymkhana-location", refresh);
    return () => {
      window.history.pushState = originalPush;
      window.history.replaceState = originalReplace;
      window.removeEventListener("popstate", refresh);
      window.removeEventListener("gymkhana-location", refresh);
    };
  }, []);
  return value;
}

function attachmentSelection(
  location: string,
): { owner: AttachmentOwner; title: string; description: string } | undefined {
  const url = new URL(location);
  if (url.pathname !== "/profiles") return undefined;
  const section = url.searchParams.get("section");
  if (section === "documents") {
    const ownerID = url.searchParams.get("document_selected");
    const mode = url.searchParams.get("document_mode");
    if (ownerID && (mode === "view" || mode === "edit")) {
      return {
        owner: { owner_kind: "DOCUMENT", owner_id: ownerID },
        title: "Anexos do documento",
        description: "Arquivos privados vinculados exclusivamente a este documento.",
      };
    }
  }
  if (section === "bills") {
    const ownerID = url.searchParams.get("bill_selected");
    const mode = url.searchParams.get("bill_mode");
    if (ownerID && (mode === "view" || mode === "edit")) {
      return {
        owner: { owner_kind: "BILL", owner_id: ownerID },
        title: "Anexos da conta ou comprovante",
        description: "Arquivos privados vinculados exclusivamente a este registro.",
      };
    }
  }
  return undefined;
}
