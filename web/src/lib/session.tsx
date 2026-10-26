import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { ApiError, request } from './api';
import type { AIStatus, Catalog, Lang, SessionView } from './types';
import { useI18n } from '../i18n';
import { useCatalog } from './catalog';
import { useRouter } from './router';
import { useOnline } from './online';

type Status = 'loading' | 'none' | 'ready' | 'ended';
/** Why the session ended: deleted by the visitor, or gone on the server (expiry, restart). */
export type EndedReason = 'deleted' | 'gone';

/** Owner pages that show (and depend on) the session; navigating to one reloads the view. */
export const SESSION_PATHS = ['/consent', '/story', '/ledger', '/next', '/practice', '/share', '/journey', '/settings'];

type SessionValue = {
  status: Status;
  session: SessionView | null;
  catalog: Catalog | null;
  catalogError: string | null;
  online: boolean;
  /** refresh reloads the session view. */
  refresh: () => Promise<SessionView | null>;
  /** setSession replaces the view after a successful mutation. A response
   * that interrupts the session (crisis route) navigates to Immediate help. */
  setSession: (s: SessionView | null) => void;
  /** run executes a mutation; stale versions trigger a refresh, missing sessions end the session. */
  run: <T>(fn: (version: number) => Promise<T>) => Promise<T>;
  /** changeLanguage switches the interface now and saves it to the session
   * (retrying once on a stale version). It resolves false when it could not be saved. */
  changeLanguage: (l: Lang) => Promise<boolean>;
  markEnded: (reason?: EndedReason) => void;
  endedReason: EndedReason;
  /** ai is the provider status from the session, or from /ai/status before a session exists. */
  ai: AIStatus | null;
};

const SessionContext = createContext<SessionValue | null>(null);

/** sameSession reports whether two views belong to one session: the view
 * carries no id, and createdAt (nanosecond precision) is fixed per session. */
export function sameSession(a: SessionView, b: SessionView): boolean {
  return a.createdAt === b.createdAt;
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const { setLang } = useI18n();
  const { path, navigate } = useRouter();
  const { catalog, error: catalogError } = useCatalog();
  const [status, setStatus] = useState<Status>('loading');
  const [session, setSessionState] = useState<SessionView | null>(null);
  const [endedReason, setEndedReason] = useState<EndedReason>('gone');
  const [aiStatus, setAIStatus] = useState<AIStatus | null>(null);
  const online = useOnline();
  const sessionRef = useRef<SessionView | null>(null);
  // The server language last applied to the interface. A refreshed view only
  // changes the interface language when the server's value itself changed,
  // so a background refresh never reverts the visitor's own choice.
  const serverLang = useRef<Lang | null>(null);

  const apply = useCallback(
    (s: SessionView, toHelpOnCrisis: boolean) => {
      const prev = sessionRef.current;
      // createdAt identifies a session (the view has no id). A different
      // session (the old one deleted or expired and a new one started, e.g.
      // in another tab) always replaces the view, whatever its version.
      const same = prev && sameSession(prev, s) ? prev : null;
      // Within one session versions only grow: a view older than the one
      // shown (a background refresh answered after a mutation) is dropped.
      if (same && s.version < same.version) return;
      sessionRef.current = s;
      setSessionState(s);
      setStatus('ready');
      if (s.language !== serverLang.current) {
        serverLang.current = s.language;
        setLang(s.language);
      }
      if (toHelpOnCrisis && s.safety.interrupted && !same?.safety.interrupted && window.location.pathname !== '/help') navigate('/help');
    },
    [setLang, navigate],
  );

  const setSession = useCallback(
    (s: SessionView | null) => {
      if (s) apply(s, true);
      else {
        sessionRef.current = null;
        setSessionState(null);
      }
    },
    [apply],
  );

  const refresh = useCallback(async () => {
    try {
      const s = await request<SessionView>('/session');
      // A session that became interrupted since the last view (another tab,
      // a rehearsal) routes to help; the first load of the app does not.
      apply(s, sessionRef.current !== null);
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
  }, [apply]);

  useEffect(() => {
    request<AIStatus>('/ai/status').then(setAIStatus).catch(() => undefined);
    refresh().catch(() => setStatus('none'));
    // Recipient actions and expiry change the owner's view on the server:
    // reload it whenever the visitor comes back to the tab.
    let last = 0;
    const back = () => {
      // focus and visibilitychange usually fire together: reload once.
      if (document.visibilityState !== 'visible' || !sessionRef.current || Date.now() - last < 1000) return;
      last = Date.now();
      refresh().catch(() => undefined);
    };
    window.addEventListener('focus', back);
    document.addEventListener('visibilitychange', back);
    return () => {
      window.removeEventListener('focus', back);
      document.removeEventListener('visibilitychange', back);
    };
  }, [refresh]);

  // Every in-app navigation to a session page shows the current server state.
  useEffect(() => {
    if (sessionRef.current && SESSION_PATHS.includes(path)) refresh().catch(() => undefined);
  }, [path, refresh]);

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
        } else if (err.code === 'stale_version' || err.code === 'crisis_interrupted') {
          await refresh().catch(() => undefined);
        }
      }
      throw err;
    }
  }, [refresh]);

  const changeLanguage = useCallback(
    async (l: Lang): Promise<boolean> => {
      setLang(l);
      if (!sessionRef.current || sessionRef.current.language === l) return true;
      const save = (version: number) => request<SessionView>('/session', { method: 'PATCH', body: { expectedVersion: version, language: l } });
      for (let tries = 0; tries < 2; tries++) {
        try {
          setSession(await run(save));
          return true;
        } catch (err) {
          // run() refreshed the view on a stale version: try once more with it.
          if (!(err instanceof ApiError && err.code === 'stale_version')) return false;
        }
      }
      return false;
    },
    [setLang, run, setSession],
  );

  const markEnded = useCallback((reason: EndedReason = 'gone') => {
    sessionRef.current = null;
    setSessionState(null);
    setEndedReason(reason);
    setStatus('ended');
  }, []);

  const ai = session?.ai ?? aiStatus;
  const value = useMemo(
    () => ({ status, session, catalog, catalogError, online, refresh, setSession, run, changeLanguage, markEnded, endedReason, ai }),
    [status, session, catalog, catalogError, online, refresh, setSession, run, changeLanguage, markEnded, endedReason, ai],
  );
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionValue {
  const v = useContext(SessionContext);
  if (!v) throw new Error('useSession outside SessionProvider');
  return v;
}

/** useOptionalSession is null outside the owner's SessionProvider (the recipient perspective). */
export function useOptionalSession(): SessionValue | null {
  return useContext(SessionContext);
}
