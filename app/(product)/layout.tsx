import type { Metadata } from "next";
import type { ReactNode } from "react";

import { AppShell } from "@/components/app-shell";
import { SessionProvider } from "@/components/session-provider";

export const metadata: Metadata = {
  robots: { index: false, follow: false, noarchive: true, nocache: true },
};

export default function ProductLayout({ children }: { children: ReactNode }) {
  return (
    <SessionProvider>
      <AppShell>{children}</AppShell>
    </SessionProvider>
  );
}
