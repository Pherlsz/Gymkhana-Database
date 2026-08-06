// CSS tokens removidos - usando shims locais
// CSS styles removidos - usando shims locais
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
    <ThemeProvider density="comfortable" theme="system">
      <App />
    </ThemeProvider>
  </StrictMode>,
);
