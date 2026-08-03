import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";

import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "Coreloop",
    template: "%s · Coreloop",
  },
  description:
    "A private, Telegram-first system for coherent technical learning and current engineering signals.",
};

export const viewport: Viewport = {
  colorScheme: "light",
  themeColor: "oklch(1 0 0)",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
