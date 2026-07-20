import { createContext, useContext } from "react";
import type { AuthSessionResponse } from "../lib/api/auth";

export const SessionContext = createContext<AuthSessionResponse | null>(null);

export function useApplicationSession(): AuthSessionResponse & {
  user: NonNullable<AuthSessionResponse["user"]>;
} {
  const session = useContext(SessionContext);
  if (!session?.authenticated || !session.user) {
    throw new Error("Authenticated session is unavailable");
  }
  return session as AuthSessionResponse & { user: NonNullable<AuthSessionResponse["user"]> };
}
