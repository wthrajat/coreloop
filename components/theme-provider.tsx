"use client";

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useSyncExternalStore,
  type ReactNode,
} from "react";

export type ThemeMode = "system" | "light" | "dark";

type ThemeContextValue = {
  mode: ThemeMode;
  cycleMode: () => void;
};

const THEME_STORAGE_KEY = "coreloop-theme";
const THEME_CHANGE_EVENT = "coreloop-theme-change";
const themeModes: ThemeMode[] = ["system", "light", "dark"];

const ThemeContext = createContext<ThemeContextValue | null>(null);

function isThemeMode(value: string | null): value is ThemeMode {
  return value === "system" || value === "light" || value === "dark";
}

function applyTheme(mode: ThemeMode) {
  if (mode === "system") {
    document.documentElement.removeAttribute("data-theme");
    return;
  }
  document.documentElement.setAttribute("data-theme", mode);
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const mode = useSyncExternalStore<ThemeMode>(
    (onStoreChange) => {
      window.addEventListener("storage", onStoreChange);
      window.addEventListener(THEME_CHANGE_EVENT, onStoreChange);
      return () => {
        window.removeEventListener("storage", onStoreChange);
        window.removeEventListener(THEME_CHANGE_EVENT, onStoreChange);
      };
    },
    (): ThemeMode => {
      const savedMode = window.localStorage.getItem(THEME_STORAGE_KEY);
      return isThemeMode(savedMode) ? savedMode : "system";
    },
    (): ThemeMode => "system",
  );

  useEffect(() => {
    applyTheme(mode);
  }, [mode]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      mode,
      cycleMode: () => {
        const currentIndex = themeModes.indexOf(mode);
        const nextMode: ThemeMode =
          themeModes[(currentIndex + 1) % themeModes.length];
        window.localStorage.setItem(THEME_STORAGE_KEY, nextMode);
        window.dispatchEvent(new Event(THEME_CHANGE_EVENT));
      },
    }),
    [mode],
  );

  return (
    <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
  );
}

export function useTheme() {
  const value = useContext(ThemeContext);
  if (!value) throw new Error("useTheme must be used inside ThemeProvider");
  return value;
}
