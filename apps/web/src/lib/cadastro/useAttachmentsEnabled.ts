import { useQuery } from "@tanstack/react-query";
import { APIRequestError, jsonRequest, requestJSON } from "../api/client";

/** Probes private storage by attempting an upload intent; validation errors imply storage is up. */
export function useAttachmentsEnabled() {
  return useQuery({
    queryKey: ["attachments-enabled"],
    queryFn: async ({ signal }) => {
      try {
        await requestJSON("/api/v1/attachment-upload-intents", {
          ...jsonRequest("POST", {
            owner_kind: "DOCUMENT",
            owner_id: "00000000-0000-0000-0000-000000000001",
            original_filename: "probe.txt",
            declared_mime: "text/plain",
            expected_size: 1,
          }),
          signal,
        });
        return true;
      } catch (error) {
        if (error instanceof APIRequestError && error.status === 503) return false;
        return true;
      }
    },
    staleTime: 5 * 60_000,
  });
}
