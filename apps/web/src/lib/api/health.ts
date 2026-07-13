export async function checkLiveHealth(signal?: AbortSignal): Promise<boolean> {
  const baseUrl = import.meta.env.VITE_API_BASE_URL ?? "";
  const response = await fetch(`${baseUrl}/health/live`, signal ? { signal } : undefined);

  return response.ok;
}
