import { apiURL, throwAPIError } from "../api/client";

export const RECORTE_XLSX_FALLBACK = "recorte.xlsx";

export function filenameFromContentDisposition(
  header: string | null,
  fallback = RECORTE_XLSX_FALLBACK,
): string {
  if (!header) return fallback;
  const encoded = /filename\*\s*=\s*UTF-8''([^;]+)/i.exec(header);
  if (encoded) {
    try {
      const name = decodeURIComponent(encoded[1].trim());
      return name || fallback;
    } catch {
      return fallback;
    }
  }
  const quoted = /filename\s*=\s*"([^"]+)"/i.exec(header);
  if (quoted?.[1]) return quoted[1];
  const plain = /filename\s*=\s*([^;]+)/i.exec(header);
  if (plain?.[1]) {
    const name = plain[1].trim().replace(/^["']|["']$/g, "");
    return name || fallback;
  }
  return fallback;
}

export async function downloadChatResultXlsx(referenceId: string): Promise<void> {
  const id = referenceId.trim();
  if (!id) throw new Error("missing recorte");
  const response = await fetch(apiURL(`/api/v1/chat/result-references/${id}/xlsx`), {
    credentials: "include",
    headers: {
      Accept: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    },
  });
  if (!response.ok) await throwAPIError(response);
  const blob = await response.blob();
  const filename = filenameFromContentDisposition(
    response.headers.get("content-disposition"),
    RECORTE_XLSX_FALLBACK,
  );
  const href = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = href;
  anchor.rel = "noreferrer";
  anchor.download = filename;
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(href);
}
