import { message } from "antd";

/**
 * Cadastro navigates away right after saving, so an in-page notice never
 * renders. Announce through antd's global message instead — it survives the
 * route change. The sink is injectable for tests; in production it is the
 * antd static message API.
 */
export function announceSaved(
  sink: { success: (content: string) => void } | undefined,
  content: string,
): void {
  (sink ?? message).success(content);
}
