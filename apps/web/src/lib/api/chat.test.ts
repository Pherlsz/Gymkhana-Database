import { afterEach, describe, expect, it, vi } from "vitest";
import { APIRequestError } from "./client";
import { consumeChatRunEvents } from "./chat";

const timestamp = "2026-07-18T12:00:00Z";

function streamResponse(frames: string): Response {
  return new Response(frames, { headers: { "content-type": "text/event-stream" } });
}

function frame(sequence: number, kind: string, extra: Record<string, unknown> = {}): string {
  return `id: ${sequence}\nevent: ${kind}\ndata: ${JSON.stringify({ sequence, kind, created_at: timestamp, ...extra })}\n\n`;
}

describe("AI Chat event client", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("reconnects from the last persisted sequence and ignores replayed events", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(streamResponse(frame(1, "TEXT_DELTA", { text_delta: "Olá" })))
      .mockResolvedValueOnce(
        streamResponse(frame(1, "TEXT_DELTA", { text_delta: "Olá" }) + frame(2, "RUN_COMPLETED")),
      );
    vi.stubGlobal("fetch", fetchMock);
    const events: number[] = [];

    const cursor = await consumeChatRunEvents(
      "019bf789-4400-7f12-9abc-123456789abc",
      0,
      (event) => events.push(event.sequence),
      new AbortController().signal,
    );

    expect(cursor).toBe(2);
    expect(events).toEqual([1, 2]);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      expect.stringContaining("after=1"),
      expect.objectContaining({
        credentials: "include",
        headers: expect.objectContaining({ "Last-Event-ID": "1" }),
      }),
    );
  });

  it("rejects malformed stream data without exposing its raw body", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(streamResponse("id: 1\nevent: TEXT_DELTA\ndata: not-json\n\n")),
    );

    await expect(
      consumeChatRunEvents(
        "019bf789-4400-7f12-9abc-123456789abc",
        0,
        vi.fn(),
        new AbortController().signal,
      ),
    ).rejects.toEqual(
      expect.objectContaining<Partial<APIRequestError>>({ code: "chat_malformed_provider" }),
    );
  });
});
