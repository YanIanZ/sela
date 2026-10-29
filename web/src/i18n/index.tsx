import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

/** Sela's two interface languages. Copy lives beside each screen (useWords). */
export type Lang = 'id' | 'en';

type I18nValue = {
  lang: Lang;
  setLang: (l: Lang) => void;
};

const I18nContext = createContext<I18nValue | null>(null);
const STORAGE_KEY = 'sela.lang';

function initialLang(): Lang {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored === 'id' || stored === 'en') return stored;
  } catch {
    // storage unavailable (private mode): fall through
  }
  return navigator.language?.toLowerCase().startsWith('en') ? 'en' : 'id';
}

export function I18nProvider({ children, initial }: { children: ReactNode; initial?: Lang }) {
  const [lang, setLangState] = useState<Lang>(initial ?? initialLang);
  useEffect(() => {
    document.documentElement.lang = lang;
    try {
      window.localStorage.setItem(STORAGE_KEY, lang);
    } catch {
      // per-viewer convenience only
    }
  }, [lang]);
  const setLang = useCallback((l: Lang) => setLangState(l), []);
  const value = useMemo<I18nValue>(() => ({ lang, setLang }), [lang, setLang]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const v = useContext(I18nContext);
  if (!v) throw new Error('useI18n outside I18nProvider');
  return v;
}
