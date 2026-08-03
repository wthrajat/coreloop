import type { ReactNode } from "react";

type StatusTone = "neutral" | "attention" | "ready" | "error";

type StatusPillProps = {
  children: ReactNode;
  tone?: StatusTone;
};

export function StatusPill({ children, tone = "neutral" }: StatusPillProps) {
  return (
    <span className="status-pill" data-tone={tone}>
      {children}
    </span>
  );
}
