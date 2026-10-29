import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
  type MouseEvent,
  type MouseEventHandler,
  type AnchorHTMLAttributes,
} from "react";

// A minimal history-API router: the app has a fixed set of top-level pages,
// so a dependency is not needed. Query strings are kept; fragments are left
// to the page (the recipient link keeps its token in the fragment).
type Route = { path: string; query: URLSearchParams; hash: string };
type RouterValue = Route & {
  navigate: (to: string, opts?: { replace?: boolean }) => void;
};

const RouterContext = createContext<RouterValue | null>(null);

function current(): Route {
  return {
    path: window.location.pathname || "/",
    query: new URLSearchParams(window.location.search),
    hash: window.location.hash,
  };
}

export function RouterProvider({ children }: { children: ReactNode }) {
  const [route, setRoute] = useState<Route>(current);
  useEffect(() => {
    const onPop = () => setRoute(current());
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  const navigate = useCallback((to: string, opts?: { replace?: boolean }) => {
    // Navigating to the page already shown (e.g. two code paths sending the
    // visitor to Immediate help) does not add a history entry.
    const target = to;
    const same = target === window.location.pathname + window.location.search;
    if (opts?.replace || same) window.history.replaceState(null, "", target);
    else window.history.pushState(null, "", target);
    setRoute(current());
    window.scrollTo({ top: 0 });
    // Move focus to the main landmark so screen readers announce the new
    // page. preventScroll keeps the freshly reset route at the top instead
    // of letting focus scroll it down by the header's scroll padding.
    requestAnimationFrame(() =>
      document.getElementById("main")?.focus({ preventScroll: true }),
    );
  }, []);
  const value = useMemo(() => ({ ...route, navigate }), [route, navigate]);
  return (
    <RouterContext.Provider value={value}>{children}</RouterContext.Provider>
  );
}

export function useRouter(): RouterValue {
  const v = useContext(RouterContext);
  if (!v) throw new Error("useRouter outside RouterProvider");
  return v;
}

export function Link({
  to,
  children,
  className,
  onClick,
  ...rest
}: {
  to: string;
  children: ReactNode;
  className?: string;
} & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href" | "onClick"> & {
    onClick?: MouseEventHandler<HTMLAnchorElement>;
  }) {
  const { navigate } = useRouter();
  const handle = (e: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(e);
    if (
      e.defaultPrevented ||
      e.button !== 0 ||
      e.metaKey ||
      e.ctrlKey ||
      e.shiftKey ||
      e.altKey
    )
      return;
    e.preventDefault();
    navigate(to);
  };
  return (
    <a href={to} onClick={handle} className={className} {...rest}>
      {children}
    </a>
  );
}
