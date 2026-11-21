/**
 * Pure Grex JS v1.0.0 Light & Dark Theme Controller for Groxy Desk v1.1.0.
 * Zero-FOUC SSR + persistent server sync + WCAG AA (>= 4.5:1) verified palettes.
 */
import { applyTokens, computeContrastRatio, createTheme } from "../grex/grex.js";

export const DESK_THEME_PALETTES = {
  light: {
    "--grex-bg": "#ffffff",
    "--grex-surface": "#f8fafc",
    "--grex-surface-alt": "#f1f5f9",
    "--grex-text": "#0f172a",
    "--grex-muted": "#475569",
    "--grex-accent": "#1d4ed8",
    "--grex-accent-text": "#ffffff",
    "--grex-border": "#cbd5e1",
    "--grex-success": "#047857",
    "--grex-warning": "#b45309",
    "--grex-danger": "#b91c1c",
    "--grex-code-bg": "#f1f5f9",
    "--grex-focus-ring": "#1d4ed8",
  },
  dark: {
    "--grex-bg": "#090d16",
    "--grex-surface": "#111827",
    "--grex-surface-alt": "#1e293b",
    "--grex-text": "#f8fafc",
    "--grex-muted": "#cbd5e1",
    "--grex-accent": "#60a5fa",
    "--grex-accent-text": "#090d16",
    "--grex-border": "#334155",
    "--grex-success": "#34d399",
    "--grex-warning": "#fbbf24",
    "--grex-danger": "#f87171",
    "--grex-code-bg": "#0f172a",
    "--grex-focus-ring": "#93c5fd",
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
      ["text_on_bg", p["--grex-text"], p["--grex-bg"]],
      ["text_on_surface", p["--grex-text"], p["--grex-surface"]],
      ["text_on_surface_alt", p["--grex-text"], p["--grex-surface-alt"]],
      ["muted_on_surface", p["--grex-muted"], p["--grex-surface"]],
      ["muted_on_surface_alt", p["--grex-muted"], p["--grex-surface-alt"]],
      ["accent_text_on_accent", p["--grex-accent-text"], p["--grex-accent"]],
      ["accent_on_surface", p["--grex-accent"], p["--grex-surface"]],
      ["success_on_surface", p["--grex-success"], p["--grex-surface"]],
      ["warning_on_surface", p["--grex-warning"], p["--grex-surface"]],
      ["danger_on_surface", p["--grex-danger"], p["--grex-surface"]],
      ["text_on_code_bg", p["--grex-text"], p["--grex-code-bg"]],
    ];
    for (const [name, fg, bg] of pairs) {
      const ratio = computeContrastRatio(fg, bg);
      results.push({
        mode,
        pair: name,
        fg,
        bg,
        ratio,
        passAA: ratio >= 4.5,
      });
    }
  }
  return {
    ok: results.every((r) => r.passAA),
    results,
  };
}
