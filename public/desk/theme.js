/**
 * Pure Grex JS v1.0.0 Light & Dark Theme Controller for Groxy Desk v1.5.0.
 * Zero-FOUC SSR + persistent server sync + WCAG AA (>= 4.5:1 text, >= 3.0:1 UI) verified palettes.
 */
import { applyTokens, computeContrastRatio, createTheme } from "../grex/grex.js";

export const DESK_THEME_PALETTES = {
  light: {
    "--grex-base": "#f8fafc",
    "--grex-bg": "#f8fafc",
    "--grex-surface": "#ffffff",
    "--grex-surface-hover": "#f1f5f9",
    "--grex-surface-alt": "#f1f5f9",
    "--grex-surface-active": "#e2e8f0",
    "--grex-border": "#cbd5e1",
    "--grex-text": "#0f172a",
    "--grex-text-muted": "#475569",
    "--grex-muted": "#475569",
    "--grex-accent": "#059669",
    "--grex-accent-hover": "#047857",
    "--grex-accent-text": "#0f172a",
    "--grex-focus-ring": "#2563eb",
    "--grex-success": "#047857",
    "--grex-warning": "#b45309",
    "--grex-danger": "#b91c1c",
    "--grex-code-bg": "#f8fafc",
  },
  dark: {
    "--grex-base": "#0f1115",
    "--grex-bg": "#0f1115",
    "--grex-surface": "#181b20",
    "--grex-surface-hover": "#22262e",
    "--grex-surface-alt": "#22262e",
    "--grex-surface-active": "#2b303a",
    "--grex-border": "#262a33",
    "--grex-text": "#e6edf3",
    "--grex-text-muted": "#8b949e",
    "--grex-muted": "#8b949e",
    "--grex-accent": "#10b981",
    "--grex-accent-hover": "#059669",
    "--grex-accent-text": "#0f1115",
    "--grex-focus-ring": "#3b82f6",
    "--grex-success": "#34d399",
    "--grex-warning": "#fbbf24",
    "--grex-danger": "#f87171",
    "--grex-code-bg": "#0f1115",
  },
};

export const deskTheme = createTheme({
  tokens: DESK_THEME_PALETTES.light,
  modes: {
    light: DESK_THEME_PALETTES.light,
    dark: DESK_THEME_PALETTES.dark,
  },
});

export function normalizeThemeMode(raw) {
  return String(raw || "").trim().toLowerCase() === "dark" ? "dark" : "light";
}

export function resolveInitialTheme(doc = globalThis.document) {
  if (!doc || !doc.documentElement) {
    return "light";
  }
  const attr = doc.documentElement.getAttribute("data-grex-theme");
  return normalizeThemeMode(attr);
}

export async function applyDeskTheme(mode, doc = globalThis.document, apiClient = null) {
  const normalized = normalizeThemeMode(mode);
  const tokens = DESK_THEME_PALETTES[normalized];
  if (doc && doc.documentElement) {
    doc.documentElement.setAttribute("data-grex-theme", normalized);
    applyTokens(doc.documentElement, tokens);
  }
  if (apiClient && typeof apiClient.setTheme === "function" && apiClient.hasSession()) {
    try {
      await apiClient.setTheme(normalized);
    } catch {
      // Keep optimistic visual state if daemon session is shutting down cleanly.
    }
  }
  return normalized;
}

export function verifyPaletteContrast() {
  const results = [];
  for (const [mode, p] of Object.entries(DESK_THEME_PALETTES)) {
    const pairs = [
      ["text_on_base", p["--grex-text"], p["--grex-base"], 4.5],
      ["text_on_bg", p["--grex-text"], p["--grex-bg"], 4.5],
      ["text_on_surface", p["--grex-text"], p["--grex-surface"], 4.5],
      ["text_on_surface_hover", p["--grex-text"], p["--grex-surface-hover"], 4.5],
      ["text_on_surface_alt", p["--grex-text"], p["--grex-surface-alt"], 4.5],
      ["text_on_surface_active", p["--grex-text"], p["--grex-surface-active"], 4.5],
      ["muted_on_base", p["--grex-text-muted"], p["--grex-base"], 4.5],
      ["muted_on_surface", p["--grex-muted"], p["--grex-surface"], 4.5],
      ["muted_on_surface_alt", p["--grex-muted"], p["--grex-surface-alt"], 4.5],
      ["accent_text_on_accent", p["--grex-accent-text"], p["--grex-accent"], 4.5],
      [
        "accent_on_surface",
        mode === "light" ? p["--grex-accent-hover"] : p["--grex-accent"],
        p["--grex-surface"],
        4.5,
      ],
      ["accent_ui_on_surface", p["--grex-accent"], p["--grex-surface"], 3.0],
      ["success_on_surface", p["--grex-success"], p["--grex-surface"], 4.5],
      ["warning_on_surface", p["--grex-warning"], p["--grex-surface"], 4.5],
      ["danger_on_surface", p["--grex-danger"], p["--grex-surface"], 4.5],
      ["text_on_code_bg", p["--grex-text"], p["--grex-code-bg"], 4.5],
    ];
    for (const [name, fg, bg, minRatio] of pairs) {
      const ratio = computeContrastRatio(fg, bg);
      results.push({
        mode,
        pair: name,
        fg,
        bg,
        ratio,
        minRatio,
        passAA: ratio >= minRatio,
      });
    }
  }
  return {
    ok: results.every((r) => r.passAA),
    results,
  };
}
