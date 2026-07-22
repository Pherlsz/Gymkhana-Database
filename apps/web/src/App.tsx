import type { AuthSessionResponse } from "./lib/api/client";

export function useApplicationSession(): AuthSessionResponse {
  return {
    authenticated: true,
    user: {
      login: "admin",
      display_name: "Admin",
      role: "ADMIN",
    },
  };
}

export const profilesRoute = { id: "/profiles", useSearch: () => ({}) } as any;
export const searchRoute = { id: "/search", useSearch: () => ({}) } as any;
export const operationsRoute = { id: "/operations", useSearch: () => ({}) } as any;
export const googleFormsRoute = { id: "/google-forms", useSearch: () => ({}) } as any;
export const queryRoute = { id: "/query", useSearch: () => ({}) } as any;
export const taskRoute = { id: "/tasks", useSearch: () => ({}) } as any;
export const matchingRoute = { id: "/matching", useSearch: () => ({}) } as any;
export const chatRoute = { id: "/chat", useSearch: () => ({}) } as any;
export const ocrRoute = { id: "/ocr", useSearch: () => ({}) } as any;

export function App() {
  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        backgroundColor: "#090a0f",
        color: "#ffffff",
        fontFamily: "system-ui, -apple-system, sans-serif",
      }}
    >
      <div
        style={{
          padding: "2.5rem 2rem",
          borderRadius: "1.25rem",
          border: "1px solid rgba(255, 255, 255, 0.1)",
          backgroundColor: "rgba(18, 20, 29, 0.85)",
          backdropFilter: "blur(16px)",
          textAlign: "center",
          maxWidth: "420px",
          boxShadow: "0 25px 50px -12px rgba(0, 0, 0, 0.5)",
        }}
      >
        <h1 style={{ fontSize: "1.375rem", fontWeight: 700, marginBottom: "0.75rem", color: "#ffffff" }}>
          Gymkhana Database
        </h1>
        <p style={{ fontSize: "0.9375rem", color: "#a0aec0", margin: 0, lineHeight: 1.5 }}>
          Interface antiga removida. O workspace está limpo e pronto para a migração da nova UI/UX.
        </p>
      </div>
    </div>
  );
}
