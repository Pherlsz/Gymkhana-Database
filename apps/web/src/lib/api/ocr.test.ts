import { afterEach, describe, expect, it, vi } from "vitest";
import { consumeOCRJobEvents } from "./ocr";

const jobID = "019c0000-0000-7000-8000-000000000001";

function eventFrame(sequence: number, kind: string): string {
  return `id: ${sequence}\nevent: ${kind}\ndata: ${JSON.stringify({
    sequence,
    kind,
    created_at: "2026-07-18T12:00:00Z",
  })}\n\n`;
}

describe("OCR event stream", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("resumes from the durable cursor, ignores duplicates, and stops on a terminal event", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(eventFrame(7, "SOURCE_VALIDATED") + eventFrame(8, "JOB_COMPLETED"), {
        headers: { "content-type": "text/event-stream; charset=utf-8" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const events: number[] = [];

    const cursor = await consumeOCRJobEvents(
      jobID,
      7,
      (event) => events.push(event.sequence),
      new AbortController().signal,
    );

    expect(cursor).toBe(8);
    expect(events).toEqual([8]);
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining(`/api/v1/ocr/jobs/${jobID}/events?after=7`),
      expect.objectContaining({
        credentials: "include",
        headers: expect.objectContaining({ "Last-Event-ID": "7" }),
      }),
    );
  });

  it("reconnects with the last persisted sequence when a non-terminal stream closes", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(eventFrame(3, "SOURCE_VALIDATED"), {
          headers: { "content-type": "text/event-stream" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(eventFrame(4, "JOB_COMPLETED"), {
          headers: { "content-type": "text/event-stream" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);
    const events: number[] = [];

    const cursor = await consumeOCRJobEvents(
      jobID,
      2,
      (event) => events.push(event.sequence),
      new AbortController().signal,
    );

    expect(cursor).toBe(4);
    expect(events).toEqual([3, 4]);
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      expect.stringContaining(`after=3`),
      expect.objectContaining({ headers: expect.objectContaining({ "Last-Event-ID": "3" }) }),
    );
  });
});
