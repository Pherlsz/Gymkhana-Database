import "@pherlsz/gymkhana-ui/tokens.css";
import "@pherlsz/gymkhana-ui/styles.css";
import { ThemeProvider } from "@pherlsz/gymkhana-ui";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import "./styles.css";
import "./attachments.css";
import "./search.css";
import "./operations.css";
import "./google-forms.css";
import "./query.css";

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
