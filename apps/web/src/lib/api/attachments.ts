import { APIRequestError, apiURL } from "./client";

export type AttachmentOwnerKind = "DOCUMENT" | "BILL" | "CUSTOM_FIELD";
export type AttachmentCustomTargetKind = "PROFILE" | "DOCUMENT" | "BILL" | "CUSTOM_ENTITY";
export type AttachmentLifecycleState = "ACTIVE" | "TRASHED";

export type AttachmentOwner = {
  owner_kind: AttachmentOwnerKind;
  owner_id: string;
  custom_target_kind?: AttachmentCustomTargetKind;
  field_definition_id?: string;
};

export type AttachmentRecord = AttachmentOwner & {
  id: string;
  original_filename: string;
  declared_mime: string;
  detected_mime: string;
  byte_size: number;
  sha256: string;
  lifecycle_state: AttachmentLifecycleState;
  deleted_at?: string;
  purge_after?: string;
  version: number;
  created_at: string;
  updated_at: string;
};

type SignedRequest = {
  url: string;
  method: "GET" | "PUT";
  headers: Record<string, string>;
  expires_at: string;
};

type UploadGrant = {
  intent_id: string;
  upload: SignedRequest;
  expires_at: string;
};

type ErrorPayload = {
  error?: { code?: string; message?: string };
  request_id?: string;
  field_errors?: Array<{ field: string; code: string; message: string }>;
};

async function readJSON<T>(response: Response): Promise<T> {
  const contentType = response.headers.get("content-type") ?? "";
  if (!contentType.toLowerCase().includes("application/json")) {
    throw new APIRequestError("API response is not JSON", {
      status: response.status,
      code: undefined,
      requestId: response.headers.get("x-request-id") ?? undefined,
    });
  }
  return (await response.json()) as T;
}

async function requestJSON<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(apiURL(path), {
    credentials: "include",
    ...init,
    headers: { Accept: "application/json", ...init.headers },
  });
  if (!response.ok) {
    let payload: ErrorPayload = {};
    try {
      payload = await readJSON<ErrorPayload>(response);
    } catch (error) {
      if (error instanceof APIRequestError) throw error;
    }
    throw new APIRequestError(payload.error?.message ?? "API request failed", {
      status: response.status,
      code: payload.error?.code,
      requestId: payload.request_id ?? response.headers.get("x-request-id") ?? undefined,
      fieldErrors: payload.field_errors,
    });
  }
  return readJSON<T>(response);
}

function ownerQuery(owner: AttachmentOwner): string {
  const query = new URLSearchParams({
    owner_kind: owner.owner_kind,
    owner_id: owner.owner_id,
  });
  if (owner.custom_target_kind) query.set("custom_target_kind", owner.custom_target_kind);
  if (owner.field_definition_id) query.set("field_definition_id", owner.field_definition_id);
  return query.toString();
}

export async function listAttachments(
  owner: AttachmentOwner,
  includeTrashed: boolean,
  signal?: AbortSignal,
): Promise<AttachmentRecord[]> {
  const query = new URLSearchParams(ownerQuery(owner));
  query.set("include_trashed", String(includeTrashed));
  const response = await requestJSON<{ attachments: AttachmentRecord[] }>(
    `/api/v1/attachments?${query.toString()}`,
    signal ? { signal } : {},
  );
  return response.attachments;
}

export async function uploadAttachment(
  owner: AttachmentOwner,
  file: File,
  onProgress: (percent: number) => void,
): Promise<AttachmentRecord> {
  const grant = await requestJSON<UploadGrant>("/api/v1/attachment-upload-intents", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      ...owner,
      original_filename: file.name,
      declared_mime: file.type || "application/octet-stream",
      expected_size: file.size,
    }),
  });
  await uploadSignedObject(grant.upload, file, onProgress);
  return requestJSON<AttachmentRecord>(
    `/api/v1/attachment-upload-intents/${encodeURIComponent(grant.intent_id)}/confirm`,
    { method: "POST" },
  );
}

function uploadSignedObject(
  signed: SignedRequest,
  file: File,
  onProgress: (percent: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open(signed.method, signed.url, true);
    for (const [name, value] of Object.entries(signed.headers))
      request.setRequestHeader(name, value);
    request.upload.onprogress = (event) => {
      if (event.lengthComputable && event.total > 0) {
        onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)));
      }
    };
    request.onerror = () =>
      reject(new Error("Não foi possível enviar o arquivo ao armazenamento privado."));
    request.onabort = () => reject(new Error("Envio cancelado."));
    request.onload = () => {
      if (request.status >= 200 && request.status < 300) {
        onProgress(100);
        resolve();
        return;
      }
      reject(
        new Error(
          "O armazenamento privado recusou o envio. Gere um novo upload e tente novamente.",
        ),
      );
    };
    request.send(file);
  });
}

export async function downloadAttachment(value: AttachmentRecord): Promise<void> {
  const signed = await requestJSON<SignedRequest>(
    `/api/v1/attachments/${encodeURIComponent(value.id)}/download`,
    { method: "POST" },
  );
  const anchor = document.createElement("a");
  anchor.href = signed.url;
  anchor.rel = "noreferrer";
  anchor.download = value.original_filename;
  anchor.click();
  anchor.remove();
}

export async function trashAttachment(value: AttachmentRecord): Promise<AttachmentRecord> {
  return requestJSON<AttachmentRecord>(`/api/v1/attachments/${encodeURIComponent(value.id)}`, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ version: value.version, confirmation: "Confirmar" }),
  });
}

export async function restoreAttachment(value: AttachmentRecord): Promise<AttachmentRecord> {
  return requestJSON<AttachmentRecord>(
    `/api/v1/attachments/${encodeURIComponent(value.id)}/restore`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ version: value.version }),
    },
  );
}
