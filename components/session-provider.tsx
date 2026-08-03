"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";

import { api, APIError } from "@/lib/api-client";
import type { SessionPayload } from "@/lib/api-types";

type SessionState = {
  status: "loading" | "authenticated" | "unauthenticated" | "error";
  session: SessionPayload | null;
  refresh: () => Promise<void>;
};

const SessionContext = createContext<SessionState | null>(null);

export function SessionProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<SessionState["status"]>("loading");
  const [session, setSession] = useState<SessionPayload | null>(null);

  async function refresh() {
    try {
      const value = await api<SessionPayload>("/session");
      setSession(value);
      setStatus("authenticated");
    } catch (error) {
      setSession(null);
      setStatus(
        error instanceof APIError && error.status === 401
          ? "unauthenticated"
          : "error",
      );
    }
  }

  useEffect(() => {
    let active = true;
    api<SessionPayload>("/session")
      .then((value) => {
        if (!active) return;
        setSession(value);
        setStatus("authenticated");
      })
      .catch((error: unknown) => {
        if (!active) return;
        setSession(null);
        setStatus(
          error instanceof APIError && error.status === 401
            ? "unauthenticated"
            : "error",
        );
      });
    return () => {
      active = false;
    };
  }, []);

  const value = useMemo(
    () => ({ status, session, refresh }),
    [status, session],
  );
  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
}

export function useSession(): SessionState {
  const value = useContext(SessionContext);
  if (!value) throw new Error("useSession must be used inside SessionProvider");
  return value;
}
