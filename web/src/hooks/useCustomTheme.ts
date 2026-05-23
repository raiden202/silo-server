import { useCallback, useEffect, useRef, useState } from "react";
import { useSettings, useSetSetting } from "@/hooks/queries/settings";
import { useOptionalAuth } from "@/hooks/useAuth";
import { shouldLoadApiTheme } from "@/hooks/themePreferences";
import { storage } from "@/utils/storage";
import { parseVarsJson } from "@/lib/themeExport";
import { sanitizeCss } from "@/lib/cssSanitizer";
import type { ThemeToken } from "@/lib/themeTokens";

export type ThemeVarOverrides = Partial<Record<ThemeToken, string>>;

interface UseCustomThemeResult {
  vars: ThemeVarOverrides;
  customCss: string;
  /** Update a single token (instant, debounced persist). */
  setVar: (token: ThemeToken, value: string) => void;
  /** Remove a single token override. */
  resetVar: (token: ThemeToken) => void;
  /** Replace all variable overrides at once. */
  setAllVars: (vars: ThemeVarOverrides) => void;
  /** Update the raw CSS. */
  setCustomCss: (css: string) => void;
  /** Reset all custom overrides. */
  resetAll: () => void;
  /** Import a full set of overrides (from file or catalog). */
  importOverrides: (vars: ThemeVarOverrides, css: string) => void;
  /** Whether local state differs from last-persisted state. */
  isDirty: boolean;
}

export function useCustomTheme(): UseCustomThemeResult {
  const auth = useOptionalAuth();
  const loadApi = shouldLoadApiTheme({
    loading: auth?.loading ?? false,
    user: auth?.user ? { id: auth.user.id } : null,
  });

  // API values
  const { data: apiSettings } = useSettings({ enabled: loadApi });
  const apiVars = apiSettings?.ui_custom_theme_vars;
  const apiCss = apiSettings?.ui_custom_css;
  const settingMutation = useSetSetting();

  // Local draft state (for instant updates without waiting for API)
  const [localVars, setLocalVars] = useState<ThemeVarOverrides>(() =>
    parseVarsJson(storage.get(storage.KEYS.UI_CUSTOM_THEME_VARS)),
  );
  const [localCss, setLocalCss] = useState<string>(
    () => storage.get(storage.KEYS.UI_CUSTOM_CSS) ?? "",
  );
  const [isDirty, setIsDirty] = useState(false);

  // Debounce timers
  const varsTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const cssTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  // Sync API values into local state when they arrive
  useEffect(() => {
    if (loadApi && apiVars !== undefined) {
      const parsed = parseVarsJson(apiVars);
      setLocalVars(parsed);
      storage.set(storage.KEYS.UI_CUSTOM_THEME_VARS, JSON.stringify(parsed));
    }
  }, [loadApi, apiVars]);

  useEffect(() => {
    if (loadApi && apiCss !== undefined && apiCss !== null) {
      setLocalCss(apiCss);
      storage.set(storage.KEYS.UI_CUSTOM_CSS, apiCss);
    }
  }, [loadApi, apiCss]);

  const persistVars = useCallback(
    (vars: ThemeVarOverrides) => {
      const json = JSON.stringify(vars);
      storage.set(storage.KEYS.UI_CUSTOM_THEME_VARS, json);
      settingMutation.mutate({ key: "ui_custom_theme_vars", value: json });
    },
    [settingMutation],
  );

  const persistCss = useCallback(
    (css: string) => {
      const safe = sanitizeCss(css);
      storage.set(storage.KEYS.UI_CUSTOM_CSS, safe);
      settingMutation.mutate({ key: "ui_custom_css", value: safe });
    },
    [settingMutation],
  );

  const setVar = useCallback(
    (token: ThemeToken, value: string) => {
      const next = { ...localVars, [token]: value };
      setLocalVars(next);
      setIsDirty(true);
      clearTimeout(varsTimerRef.current);
      varsTimerRef.current = setTimeout(() => {
        persistVars(next);
        setIsDirty(false);
      }, 500);
    },
    [localVars, persistVars],
  );

  const resetVar = useCallback(
    (token: ThemeToken) => {
      const next = { ...localVars };
      delete next[token];
      setLocalVars(next);
      persistVars(next);
      setIsDirty(false);
    },
    [localVars, persistVars],
  );

  const setAllVars = useCallback(
    (vars: ThemeVarOverrides) => {
      setLocalVars(vars);
      persistVars(vars);
      setIsDirty(false);
    },
    [persistVars],
  );

  const setCustomCss = useCallback(
    (css: string) => {
      setLocalCss(css);
      setIsDirty(true);
      clearTimeout(cssTimerRef.current);
      cssTimerRef.current = setTimeout(() => {
        persistCss(css);
        setIsDirty(false);
      }, 1000);
    },
    [persistCss],
  );

  const resetAll = useCallback(() => {
    setLocalVars({});
    setLocalCss("");
    persistVars({});
    persistCss("");
    setIsDirty(false);
  }, [persistVars, persistCss]);

  const importOverrides = useCallback(
    (vars: ThemeVarOverrides, css: string) => {
      setLocalVars(vars);
      setLocalCss(css);
      persistVars(vars);
      persistCss(css);
      setIsDirty(false);
    },
    [persistVars, persistCss],
  );

  return {
    vars: localVars,
    customCss: localCss,
    setVar,
    resetVar,
    setAllVars,
    setCustomCss,
    resetAll,
    importOverrides,
    isDirty,
  };
}
