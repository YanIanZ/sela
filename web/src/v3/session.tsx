import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useI18n } from "../i18n";
import { useRouter } from "../lib/router";
import {
  api,
  AUTH_EXPIRED_EVENT,
  setAuthActive,
  type Options,
  type Policy,
  type Profile,
} from "./api";

type SessionValue = {
  user: Profile | null;
  setUser: (p: Profile | null) => void;
  policies: Record<string, Policy>;
  options: Options | null;
  loading: boolean;
  error: unknown;
  refresh: () => Promise<Profile | null>;
  reload: () => Promise<void>;
  signOut: () => Promise<void>;
  expired: boolean;
  dismissExpired: () => void;
};

const SessionContext = createContext<SessionValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const { setLang } = useI18n();
  const { navigate } = useRouter();
  const [user, setUser] = useState<Profile | null>(null);
  const [policies, setPolicies] = useState<Record<string, Policy>>({});
  const [options, setOptions] = useState<Options | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const [expired, setExpired] = useState(false);

  const reload = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [p, o] = await Promise.all([
        api<{ policies: Record<string, Policy> }>("/policies"),
        api<Options>("/auth/options"),
      ]);
      setPolicies(p.policies);
      setOptions(o);
      try {
        const profile = await api<Profile>("/me");
        setAuthActive(true);
        setUser(profile);
        setLang(profile.locale);
        try {
          window.sessionStorage.setItem("sela.userKind", profile.kind);
        } catch {
          // non-sensitive convenience only
        }
      } catch (e) {
        const code = (e as { code?: string }).code;
        if (code === "not_signed_in") {
          setAuthActive(false);
          setUser(null);
        } else throw e;
      }
    } catch (e) {
      setError(e);
    } finally {
      setLoading(false);
    }
  }, [setLang]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const refresh = useCallback(async () => {
    try {
      const profile = await api<Profile>("/me");
      setAuthActive(true);
      setUser(profile);
      try {
        window.sessionStorage.setItem("sela.userKind", profile.kind);
      } catch {
        // session storage may be blocked; the screen is still usable
      }
      setExpired(false);
      return profile;
    } catch (e) {
      if ((e as { code?: string }).code === "not_signed_in") {
        setAuthActive(false);
        setUser(null);
        return null;
      }
      throw e;
    }
  }, []);

  useEffect(() => {
    const onExpired = () => {
      setAuthActive(false);
      // Keep the profile and the mounted screen so in-tab drafts survive.
      // The shell shows the session-ended banner and every write fails closed.
      setExpired(true);
    };
    window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
    return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
  }, []);

  const signOut = useCallback(async () => {
    try {
      await api("/auth/logout", {});
    } catch {
      // signing out is idempotent; the cookie is cleared by the server too
    }
    setAuthActive(false);
    setUser(null);
    setExpired(false);
    navigate("/");
  }, [navigate]);

  const value = useMemo<SessionValue>(
    () => ({
      user,
      setUser,
      policies,
      options,
      loading,
      error,
      refresh,
      reload,
      signOut,
      expired,
      dismissExpired: () => setExpired(false),
    }),
    [user, policies, options, loading, error, refresh, reload, signOut, expired],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

// A static provider for the separate development preview entry. It never
// initializes authentication, touches account storage, or calls the API.
export function DesignPreviewSession({ children, profile }: { children: ReactNode; profile: Profile | null }) {
  const value: SessionValue = {
    user: profile, setUser: () => {}, policies: {}, options: null,
    loading: false, error: null, expired: false, dismissExpired: () => {},
    refresh: async () => profile, reload: async () => {}, signOut: async () => {},
  };
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionValue {
  const v = useContext(SessionContext);
  if (!v) throw new Error("useSession outside SessionProvider");
  return v;
}
