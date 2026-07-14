import { getLiveHealth } from "./client";

export async function checkLiveHealth(signal?: AbortSignal): Promise<boolean> {
  const response = await getLiveHealth(signal);
  return response.status === "ok";
}
