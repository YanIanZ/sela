import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
} from "react";
import { useWords } from "../v3/shared";
import type { Admin } from "./api";
import { LanguageToggle, roleLabel } from "./chrome";
import "./shell.css";

/** Operator routes live under /operator; the app reads location.pathname. */
export function normalizePath(path: string): string {
  const p = path.replace(/\/+$/, "");
  return p === "" ? "/operator" : p;
}

type Router = { path: string; navigate: (to: string) => void };

const RouterContext = createContext<Router>({ path: "/operator", navigate: () => {} });

/** RouterProvider keeps the current operator path in sync with history. */
export function RouterProvider({ children }: { children: ReactNode }) {
  const [path, setPath] = useState(() => normalizePath(window.location.pathname));
  useEffect(() => {
    const onPop = () => setPath(normalizePath(window.location.pathname));
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  const navigate = useCallback((to: string) => {
    const next = normalizePath(to);
    if (next !== normalizePath(window.location.pathname)) {
      window.history.pushState(null, "", next);
    }
    setPath(next);
    window.scrollTo?.(0, 0);
  }, []);
  const value = useMemo(() => ({ path, navigate }), [path, navigate]);
  return <RouterContext.Provider value={value}>{children}</RouterContext.Provider>;
}

export function useRouter(): Router {
  return useContext(RouterContext);
}

/** OpLink is an in-app link: a real href, client-side navigation on click. */
export function OpLink({
  to,
  children,
  className,
  current,
}: {
  to: string;
  children: ReactNode;
  className?: string;
  current?: boolean;
}) {
  const { navigate } = useRouter();
  const onClick = (e: MouseEvent<HTMLAnchorElement>) => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    navigate(to);
  };
  return (
    <a href={to} className={className} aria-current={current ? "page" : undefined} onClick={onClick}>
      {children}
    </a>
  );
}

export type NavItem = { path: string; label: string; count?: number };

/**
 * Shell is the operator frame: a sidebar on wide screens and a compact top
 * bar with a wrapping nav on phones. Only permitted items are passed in.
 */
export function Shell({
  admin,
  items,
  active,
  signingOut,
  onSignOut,
  context,
  children,
}: {
  admin: Admin;
  items: NavItem[];
  active: string;
  signingOut?: boolean;
  onSignOut: () => void;
  context?: string;
  children: ReactNode;
}) {
  const w = useWords();
  return (
    <div className="op op-app">
      <a className="op-skip" href="#op-main">
        {w("Lewati ke konten", "Skip to content")}
      </a>
      <aside className="op-side">
        <div className="op-side-top">
          <div className="op-brand">
            <span className="op-brand-word">{w("Sela", "Sela")}</span>
            <span className="op-badge">{w("Operator", "Operator")}</span>
          </div>
          {context && <p className="op-side-context">{context}</p>}
        </div>
        <nav className="op-side-nav" aria-label={w("Navigasi operator", "Operator navigation")}>
          <ul>
            {items.map((item) => (
              <li key={item.path}>
                <OpLink to={item.path} current={item.path === active}>
                  <span>{item.label}</span>
                  {item.count !== undefined && item.count > 0 && (
                    <span className="op-side-count" aria-label={w(`${item.count} menunggu`, `${item.count} waiting`)}>
                      {item.count}
                    </span>
                  )}
                </OpLink>
              </li>
            ))}
          </ul>
        </nav>
        <div className="op-side-foot">
          <span className="op-user">
            <strong>{admin.displayName || admin.username}</strong>
            <span className="op-user-role">
              {roleLabel(admin.role, w)}
              {admin.displayName ? ` · @${admin.username}` : ""}
            </span>
          </span>
          <div className="op-side-actions">
            <LanguageToggle />
            <button type="button" className="op-btn op-btn--quiet" onClick={onSignOut} disabled={signingOut}>
              {signingOut ? w("Keluar…", "Signing out…") : w("Keluar", "Sign out")}
            </button>
          </div>
        </div>
      </aside>
      <main className="op-main op-page" id="op-main">
        {children}
      </main>
    </div>
  );
}

/** Page renders a page heading that takes focus when the page appears. */
export function Page({
  title,
  lead,
  actions,
  children,
}: {
  title: string;
  lead?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const ref = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    ref.current?.focus({ preventScroll: true });
  }, [title]);
  return (
    <>
      <div className="op-intro">
        <div>
          <h1 ref={ref} tabIndex={-1} className="op-page-title">
            {title}
          </h1>
          {lead && <p>{lead}</p>}
        </div>
        {actions}
      </div>
      {children}
    </>
  );
}
