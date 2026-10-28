import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

/*
 * Display preferences are the only thing Sela keeps in localStorage. They are
 * non-sensitive by design (text size, theme, contrast, motion, preview
 * visibility). Drafts and personal content stay in tab memory.
 */
export type TextScale = "100" | "115" | "130" | "150";
export type Theme = "system" | "light" | "dark";
export type Contrast = "system" | "more" | "less";
export type Motion = "system" | "reduced" | "full";

export type Prefs = {
  textScale: TextScale;
  spacing: "normal" | "relaxed";
  theme: Theme;
  contrast: Contrast;
  motion: Motion;
  showRepliesAtOnce: boolean;
  enterToSend: boolean;
  announceReplies: boolean;
  readAloud: boolean;
  dictation: boolean;
  quickExitShortcut: boolean;
  quickExitSignOut: "trusted" | "always" | "never";
  hidePreviews: boolean;
  neutralTitles: boolean;
  defaultStorage: "save" | "dont_save" | null;
};

export const defaultPrefs: Prefs = {
  textScale: "100",
  spacing: "normal",
  theme: "system",
  contrast: "system",
  motion: "system",
  showRepliesAtOnce: false,
  enterToSend: false,
  announceReplies: true,
  readAloud: false,
  dictation: false,
  quickExitShortcut: true,
  quickExitSignOut: "trusted",
  hidePreviews: false,
  neutralTitles: false,
  defaultStorage: null,
};

const STORAGE_KEY = "sela.prefs";

function load(): Prefs {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return defaultPrefs;
    const parsed = JSON.parse(raw) as Partial<Prefs>;
    const next = { ...defaultPrefs, ...parsed };
    // Retired colour-mood choice from earlier builds: Sampul is the only look.
    delete (next as Partial<Prefs> & { palette?: unknown }).palette;
    return next;
  } catch {
    return defaultPrefs;
  }
}

export function savePrefs(prefs: Prefs): boolean {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
    return true;
  } catch {
    return false;
  }
}

function systemDark(): boolean {
  try {
    return window.matchMedia("(prefers-color-scheme: dark)").matches;
  } catch {
    return false;
  }
}

function systemContrast(): boolean {
  try {
    return window.matchMedia("(prefers-contrast: more)").matches;
  } catch {
    return false;
  }
}

export function effectiveScheme(prefs: Prefs): string {
  const dark = prefs.theme === "dark" || (prefs.theme === "system" && systemDark());
  const contrast =
    prefs.contrast === "more" || (prefs.contrast === "system" && systemContrast());
  if (contrast) return dark ? "contrast-dark" : "contrast-light";
  return dark ? "dark" : "light";
}

function apply(prefs: Prefs) {
  const root = document.documentElement;
  root.setAttribute("data-scheme", effectiveScheme(prefs));
  if (prefs.textScale === "100") root.removeAttribute("data-text-scale");
  else root.setAttribute("data-text-scale", prefs.textScale);
  if (prefs.spacing === "relaxed") root.setAttribute("data-spacing", "relaxed");
  else root.removeAttribute("data-spacing");
  if (prefs.motion === "system") root.removeAttribute("data-motion");
  else root.setAttribute("data-motion", prefs.motion);
}

type PrefsValue = {
  prefs: Prefs;
  set: <K extends keyof Prefs>(key: K, value: Prefs[K]) => void;
  reset: () => void;
  persisted: boolean;
  scheme: string;
};

const PrefsContext = createContext<PrefsValue | null>(null);

export function PrefsProvider({ children }: { children: ReactNode }) {
  const [prefs, setPrefs] = useState<Prefs>(load);
  const [persisted, setPersisted] = useState(true);
  const [systemTick, setSystemTick] = useState(0);

  const set = useCallback(<K extends keyof Prefs>(key: K, value: Prefs[K]) => {
    setPrefs((p) => {
      const next = { ...p, [key]: value };
      setPersisted(savePrefs(next));
      return next;
    });
  }, []);

  const reset = useCallback(() => {
    setPrefs(defaultPrefs);
    setPersisted(savePrefs(defaultPrefs));
  }, []);

  useEffect(() => {
    apply(prefs);
  }, [prefs, systemTick]);

  useEffect(() => {
    let dark: MediaQueryList | null = null;
    let contrast: MediaQueryList | null = null;
    const bump = () => setSystemTick((n) => n + 1);
    try {
      dark = window.matchMedia("(prefers-color-scheme: dark)");
      contrast = window.matchMedia("(prefers-contrast: more)");
      dark.addEventListener("change", bump);
      contrast.addEventListener("change", bump);
    } catch {
      // media queries unavailable: the light scheme stands
    }
    return () => {
      dark?.removeEventListener("change", bump);
      contrast?.removeEventListener("change", bump);
    };
  }, []);

  const value = useMemo<PrefsValue>(
    () => ({ prefs, set, reset, persisted, scheme: effectiveScheme(prefs) }),
    [prefs, set, reset, persisted, systemTick],
  );
  return <PrefsContext.Provider value={value}>{children}</PrefsContext.Provider>;
}

export function usePrefs(): PrefsValue {
  const v = useContext(PrefsContext);
  if (!v) throw new Error("usePrefs outside PrefsProvider");
  return v;
}

/** useMediaQuery is a tiny live media-query hook for reduced motion and
 * viewport decisions that CSS cannot express. */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => {
    try {
      return window.matchMedia(query).matches;
    } catch {
      return false;
    }
  });
  useEffect(() => {
    let mql: MediaQueryList;
    try {
      mql = window.matchMedia(query);
    } catch {
      return;
    }
    const onChange = () => setMatches(mql.matches);
    onChange();
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, [query]);
  return matches;
}

export function useReducedMotion(): boolean {
  const { prefs } = usePrefs();
  const system = useMediaQuery("(prefers-reduced-motion: reduce)");
  if (prefs.motion === "reduced") return true;
  if (prefs.motion === "full") return false;
  return system;
}
