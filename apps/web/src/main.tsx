import "@ant-design/v5-patch-for-react-19";
import "./ui/tokens.css";
import "./ui/styles.css";
import { ConfigProvider } from "antd";
import ptBR from "antd/locale/pt_BR";
import { ThemeProvider } from "./ui";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
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

createRoot(root).render(
  <StrictMode>
    <ConfigProvider
      locale={ptBR}
      theme={{
        token: {
          colorPrimary: "#7c3aed",
          borderRadius: 8,
          fontSize: 14,
        },
      }}
    >
      <ThemeProvider theme="light">
        <App />
      </ThemeProvider>
    </ConfigProvider>
  </StrictMode>
);
