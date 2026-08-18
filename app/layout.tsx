import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";

import { ThemeProvider } from "@/components/theme-provider";

import "./globals.css";

const themeBootstrapScript = `(() => {
  try {
    const saved = localStorage.getItem("coreloop-theme");
    if (saved === "light" || saved === "dark") {
      document.documentElement.setAttribute("data-theme", saved);
      document.documentElement.style.colorScheme = saved;
    }
  } catch {}
})();`;

export const metadata: Metadata = {
  title: {
    default: "Coreloop",
    template: "%s · Coreloop",
  },
  description:
    "A self-hosted, Telegram-first system for scheduled learning and source-backed news.",
};

export const viewport: Viewport = {
  colorScheme: "light dark",
  themeColor: [
    { color: "#ffffff", media: "(prefers-color-scheme: light)" },
    { color: "#0f172a", media: "(prefers-color-scheme: dark)" },
  ],
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeBootstrapScript }} />
      </head>
      <body>
        <ThemeProvider>{children}</ThemeProvider>
      </body>
    </html>
  );
}
