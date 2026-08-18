import { Typography } from "antd";
import { useI18n } from "./i18n";

export function EmptyTab() {
  const { messages } = useI18n();
  return (
    <section className="coming-soon">
      <Typography.Title level={1}>{messages.shell.comingSoon}</Typography.Title>
    </section>
  );
}
