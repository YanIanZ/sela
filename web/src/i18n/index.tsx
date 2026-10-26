import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { id, type MessageKey } from './id';
import { en } from './en';
import type { Lang, Text } from '../lib/types';
import { setApiLanguage } from '../lib/api';

export const dictionaries: Record<Lang, Record<MessageKey, string>> = { id, en };

type Vars = Record<string, string | number>;

export function translate(lang: Lang, key: MessageKey, vars?: Vars): string {
  let s: string = dictionaries[lang][key] ?? dictionaries.id[key] ?? key;
  if (vars) for (const [k, v] of Object.entries(vars)) s = s.split(`{${k}}`).join(String(v));
  return s;
}

type I18nValue = {
  lang: Lang;
  setLang: (l: Lang) => void;
  t: (key: MessageKey, vars?: Vars) => string;
  /** tx picks the active language from a fixture Text. */
  tx: (text: Text | null | undefined) => string;
  /** has reports whether a dynamic key exists (e.g. error codes). */
  has: (key: string) => key is MessageKey;
  dateTime: (iso: string) => string;
  date: (iso: string) => string;
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
    setApiLanguage(lang);
    try {
      window.localStorage.setItem(STORAGE_KEY, lang);
    } catch {
      // per-viewer convenience only
    }
  }, [lang]);
  const setLang = useCallback((l: Lang) => setLangState(l), []);
  const value = useMemo<I18nValue>(() => {
    const locale = lang === 'en' ? 'en-GB' : 'id-ID';
    const dtf = new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' });
    const df = new Intl.DateTimeFormat(locale, { dateStyle: 'long' });
    return {
      lang,
      setLang,
      t: (key, vars) => translate(lang, key, vars),
      tx: (text) => (text ? (lang === 'en' ? text.en : text.id) : ''),
      has: (key: string): key is MessageKey => key in dictionaries[lang],
      dateTime: (iso) => (iso ? dtf.format(new Date(iso)) : ''),
      date: (iso) => (iso ? df.format(new Date(iso)) : ''),
    };
  }, [lang, setLang]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const v = useContext(I18nContext);
  if (!v) throw new Error('useI18n outside I18nProvider');
  return v;
}

export type { MessageKey };
