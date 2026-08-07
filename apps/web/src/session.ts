import { createContext, useContext } from "react";
import type { AuthSessionResponse } from "./lib/api/client";

export type ApplicationContextValue = {
  session: AuthSessionResponse;
  signingOut: boolean;
  signOut: () => void;
};

export const SessionContext = createContext<ApplicationContextValue | null>(null);

export function useApplicationSession(): AuthSessionResponse {
  const value = useContext(SessionContext);
  if (!value) throw new Error("Application session is unavailable");
  return value.session;
}

export function useApplicationContext(): ApplicationContextValue {
  const value = useContext(SessionContext);
  if (!value) throw new Error("Application session is unavailable");
  return value;
}
