import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { ApiError, request } from './api';
import type { AIStatus, Catalog, SessionView } from './types';
import { useI18n } from '../i18n';

type Status = 'loading' | 'none' | 'ready' | 'ended';
/** Why the session ended: deleted by the visitor, or gone on the server (expiry, restart). */
export type EndedReason = 'deleted' | 'gone';

type SessionValue = {
  status: Status;
  session: SessionView | null;
  catalog: Catalog | null;
  catalogError: string | null;
  online: boolean;
  /** refresh reloads the session view. */
  refresh: () => Promise<SessionView | null>;
  /** setSession replaces the view after a successful mutation. */
  setSession: (s: SessionView | null) => void;
  /** run executes a mutation; stale versions trigger a refresh, missing sessions end the session. */
  run: <T>(fn: (version: number) => Promise<T>) => Promise<T>;
  markEnded: (reason?: EndedReason) => void;
  endedReason: EndedReason;
  /** ai is the provider status from the session, or from /ai/status before a session exists. */
  ai: AIStatus | null;
};

const SessionContext = createContext<SessionValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const { setLang } = useI18n();
  const [status, setStatus] = useState<Status>('loading');
  const [session, setSessionState] = useState<SessionView | null>(null);
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const [catalogError, setCatalogError] = useState<string | null>(null);
  const [endedReason, setEndedReason] = useState<EndedReason>('gone');
  const [aiStatus, setAIStatus] = useState<AIStatus | null>(null);
  const [online, setOnline] = useState<boolean>(typeof navigator === 'undefined' ? true : navigator.onLine);
  const sessionRef = useRef<SessionView | null>(null);

  const setSession = useCallback((s: SessionView | null) => {
    sessionRef.current = s;
    setSessionState(s);
    if (s) {
      setStatus('ready');
      setLang(s.language);
    }
  }, [setLang]);

  const refresh = useCallback(async () => {
    try {
      const s = await request<SessionView>('/session');
      setSession(s);
      return s;
    } catch (err) {
      if (err instanceof ApiError && err.code === 'session_missing') {
        const had = sessionRef.current !== null;
        sessionRef.current = null;
        setSessionState(null);
        setStatus(had ? 'ended' : 'none');
        return null;
      }
      throw err;
    }
  }, [setSession]);

  useEffect(() => {
    request<Catalog>('/catalog').then(setCatalog).catch((e: unknown) => setCatalogError(e instanceof ApiError ? e.code : 'network'));
    request<AIStatus>('/ai/status').then(setAIStatus).catch(() => undefined);
    refresh().catch(() => setStatus('none'));
    const on = () => setOnline(true);
    const off = () => setOnline(false);
    window.addEventListener('online', on);
    window.addEventListener('offline', off);
    return () => {
      window.removeEventListener('online', on);
      window.removeEventListener('offline', off);
    };
  }, [refresh]);

  const run = useCallback(async <T,>(fn: (version: number) => Promise<T>): Promise<T> => {
    const version = sessionRef.current?.version ?? 0;
    try {
      return await fn(version);
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === 'session_missing') {
          sessionRef.current = null;
          setSessionState(null);
          setStatus('ended');
        } else if (err.code === 'stale_version') {
          await refresh().catch(() => undefined);
        }
      }
      throw err;
    }
  }, [refresh]);

  const markEnded = useCallback((reason: EndedReason = 'gone') => {
    sessionRef.current = null;
    setSessionState(null);
    setEndedReason(reason);
    setStatus('ended');
  }, []);

  const ai = session?.ai ?? aiStatus;
  const value = useMemo(() => ({ status, session, catalog, catalogError, online, refresh, setSession, run, markEnded, endedReason, ai }),
    [status, session, catalog, catalogError, online, refresh, setSession, run, markEnded, endedReason, ai]);
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionValue {
  const v = useContext(SessionContext);
  if (!v) throw new Error('useSession outside SessionProvider');
  return v;
}
