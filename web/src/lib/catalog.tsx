import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { ApiError, request } from './api';
import type { Catalog } from './types';

type CatalogValue = {
  catalog: Catalog | null;
  /** error is the stable code of the last failed attempt while no catalog is loaded. */
  error: string | null;
  /** retry fetches the catalog again now. */
  retry: () => void;
};

const CatalogContext = createContext<CatalogValue | null>(null);

/** Retry delays after a failed catalog request; the last one repeats. */
export const CATALOG_RETRY_MS = [2000, 5000, 10000, 30000];

/** CatalogProvider loads the fixture catalog for owner and recipient pages
 * alike. A failed request (503, 429, network) is shown and retried with
 * backoff, so no page waits on it forever. */
export function CatalogProvider({ children, initial }: { children: ReactNode; initial?: Catalog }) {
  const [catalog, setCatalog] = useState<Catalog | null>(initial ?? null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const failures = useRef(0);

  useEffect(() => {
    if (catalog) return;
    let active = true;
    let timer: number | undefined;
    request<Catalog>('/catalog')
      .then((c) => {
        if (!active) return;
        failures.current = 0;
        setCatalog(c);
        setError(null);
      })
      .catch((e: unknown) => {
        if (!active) return;
        setError(e instanceof ApiError ? e.code : 'network');
        const delay = CATALOG_RETRY_MS[Math.min(failures.current, CATALOG_RETRY_MS.length - 1)];
        failures.current += 1;
        timer = window.setTimeout(() => setAttempt((n) => n + 1), delay);
      });
    return () => {
      active = false;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [attempt, catalog]);

  const retry = useCallback(() => setAttempt((n) => n + 1), []);
  const value = useMemo(() => ({ catalog, error: catalog ? null : error, retry }), [catalog, error, retry]);
  return <CatalogContext.Provider value={value}>{children}</CatalogContext.Provider>;
}

export function useCatalog(): CatalogValue {
  const v = useContext(CatalogContext);
  if (!v) throw new Error('useCatalog outside CatalogProvider');
  return v;
}
