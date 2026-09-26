import { message } from "antd";

export function announceSaved(
  sink: { success: (content: string) => void } | undefined,
  content: string,
): void {
  (sink ?? message).success(content);
}
