import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode, type MouseEvent } from 'react';

// A minimal history-API router: the app has a fixed set of top-level pages,
// so a dependency is not needed. Query strings are kept; fragments are left
// to the page (the recipient link keeps its token in the fragment).
type Route = { path: string; query: URLSearchParams; hash: string };
type RouterValue = Route & { navigate: (to: string, opts?: { replace?: boolean }) => void };

const RouterContext = createContext<RouterValue | null>(null);

function current(): Route {
  return { path: window.location.pathname, query: new URLSearchParams(window.location.search), hash: window.location.hash };
}

export function RouterProvider({ children }: { children: ReactNode }) {
  const [route, setRoute] = useState<Route>(current);
  useEffect(() => {
    const onPop = () => setRoute(current());
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);
  const navigate = useCallback((to: string, opts?: { replace?: boolean }) => {
    if (opts?.replace) window.history.replaceState(null, '', to);
    else window.history.pushState(null, '', to);
    setRoute(current());
    window.scrollTo({ top: 0 });
    // Move focus to the main landmark so screen readers announce the new page.
    requestAnimationFrame(() => document.getElementById('main')?.focus());
  }, []);
  const value = useMemo(() => ({ ...route, navigate }), [route, navigate]);
  return <RouterContext.Provider value={value}>{children}</RouterContext.Provider>;
}

export function useRouter(): RouterValue {
  const v = useContext(RouterContext);
  if (!v) throw new Error('useRouter outside RouterProvider');
  return v;
}

export function Link({ to, children, className, ...rest }: { to: string; children: ReactNode; className?: string; 'aria-current'?: 'page' | undefined }) {
  const { navigate } = useRouter();
  const onClick = (e: MouseEvent<HTMLAnchorElement>) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    navigate(to);
  };
  return (
    <a href={to} onClick={onClick} className={className} {...rest}>
      {children}
    </a>
  );
}
