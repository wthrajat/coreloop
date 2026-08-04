"use client";

import { useTheme, type ThemeMode } from "@/components/theme-provider";

const labels: Record<ThemeMode, string> = {
  system: "System",
  light: "Light",
  dark: "Dark",
};

export function ThemeToggle({ compact = false }: { compact?: boolean }) {
  const { mode, cycleMode } = useTheme();
  const label = labels[mode];

  return (
    <button
      aria-label={`Theme: ${label}. Activate to switch theme.`}
      className={`theme-toggle${compact ? " theme-toggle-compact" : ""}`}
      onClick={cycleMode}
      title={`Theme: ${label}`}
      type="button"
    >
      <ThemeIcon mode={mode} />
      {compact ? (
        <span className="sr-only">{label}</span>
      ) : (
        <span className="theme-toggle-copy">
          <span>Theme</span>
          <strong>{label}</strong>
        </span>
      )}
    </button>
  );
}

function ThemeIcon({ mode }: { mode: ThemeMode }) {
  if (mode === "dark") {
    return (
      <svg
        aria-hidden="true"
        className="theme-toggle-icon"
        fill="none"
        viewBox="0 0 24 24"
      >
        <path
          d="M20.4 15.2A8.5 8.5 0 0 1 8.8 3.6 8.5 8.5 0 1 0 20.4 15.2Z"
          stroke="currentColor"
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="1.8"
        />
      </svg>
    );
  }

  if (mode === "light") {
    return (
      <svg
        aria-hidden="true"
        className="theme-toggle-icon"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle
          cx="12"
          cy="12"
          r="3.5"
          stroke="currentColor"
          strokeWidth="1.8"
        />
        <path
          d="M12 2.5v2M12 19.5v2M4.6 4.6l1.4 1.4M18 18l1.4 1.4M2.5 12h2M19.5 12h2M4.6 19.4 6 18M18 6l1.4-1.4"
          stroke="currentColor"
          strokeLinecap="round"
          strokeWidth="1.8"
        />
      </svg>
    );
  }

  return (
    <svg
      aria-hidden="true"
      className="theme-toggle-icon"
      fill="none"
      viewBox="0 0 24 24"
    >
      <rect
        height="13"
        rx="1.8"
        stroke="currentColor"
        strokeWidth="1.8"
        width="18"
        x="3"
        y="4"
      />
      <path
        d="M8 20h8M12 17v3"
        stroke="currentColor"
        strokeLinecap="round"
        strokeWidth="1.8"
      />
    </svg>
  );
}
