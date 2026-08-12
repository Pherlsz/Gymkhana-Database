import { ConfigProvider } from "antd";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { getAntdLocale, I18nProvider, resolveAppLocale } from "./i18n";
import "./styles.css";
import "./attachments.css";
import "./search.css";
import "./operations.css";
import "./google-forms.css";
import "./query.css";
import "./task.css";
import "./matching.css";
import "./chat.css";
import "./ocr.css";

const root = document.getElementById("root");

if (!root) {
  throw new Error("Root element was not found");
}

const locale = resolveAppLocale(navigator.language);
document.documentElement.lang = locale;

createRoot(root).render(
  <StrictMode>
    <I18nProvider locale={locale}>
      <ConfigProvider
        locale={getAntdLocale(locale)}
        theme={{
          token: {
            colorPrimary: "#7c3aed",
            borderRadius: 8,
            fontSize: 14,
          },
        }}
      >
        <App />
      </ConfigProvider>
    </I18nProvider>
  </StrictMode>,
);
