import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { useI18n } from "../i18n";
import type { Lang } from "./api";

/** useWords returns the active-language string from an ID/EN pair. */
export function useWords() {
  const { lang } = useI18n();
  return useCallback(
    (id: string, en: string) => (lang === "id" ? id : en),
    [lang],
  );
}

export function useLocale() {
  const { lang } = useI18n();
  const locale = lang === "id" ? "id-ID" : "en-GB";
  return useMemo(
    () => ({
      lang,
      locale,
      date: (iso?: string | null) =>
        iso
          ? new Intl.DateTimeFormat(locale, { dateStyle: "long" }).format(new Date(iso))
          : "",
      shortDate: (iso?: string | null) =>
        iso
          ? new Intl.DateTimeFormat(locale, { day: "numeric", month: "short" }).format(
              new Date(iso),
            )
          : "",
      dayHeading: (iso: string) => {
        const d = new Date(iso);
        const today = new Date();
        const start = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
        const diff = Math.round((start(today) - start(d)) / 86_400_000);
        if (diff === 0) return lang === "id" ? "Hari ini" : "Today";
        if (diff === 1) return lang === "id" ? "Kemarin" : "Yesterday";
        return new Intl.DateTimeFormat(locale, {
          weekday: "long",
          day: "numeric",
          month: "long",
          year: "numeric",
        }).format(d);
      },
      time: (iso?: string | null) =>
        iso
          ? new Intl.DateTimeFormat(locale, { hour: "2-digit", minute: "2-digit" }).format(
              new Date(iso),
            )
          : "",
      dateTime: (iso?: string | null) =>
        iso
          ? new Intl.DateTimeFormat(locale, {
              dateStyle: "medium",
              timeStyle: "short",
            }).format(new Date(iso))
          : "",
    }),
    [lang, locale],
  );
}

export type AnnounceSegment = { text: string; lang?: Lang };
export type Region = "status" | "reply" | "alert";

type AnnounceValue = {
  announce: (region: Region, content: string | AnnounceSegment[]) => void;
  clear: () => void;
};

const AnnounceContext = createContext<AnnounceValue | null>(null);

/**
 * The three announcement regions live at the end of <body>, are created once
 * and are cleared on route changes and on the next user input. Inserting a
 * panel with interactive content into the transcript never reads the panel
 * out: the transcript log is silent and only these regions speak.
 */
export function AnnounceProvider({ children }: { children: ReactNode }) {
  const [text, setText] = useState<Record<Region, AnnounceSegment[]>>({
    status: [],
    reply: [],
    alert: [],
  });
  const setAt = useRef(0);

  const announce = useCallback((region: Region, content: string | AnnounceSegment[]) => {
    const segments =
      typeof content === "string" ? [{ text: content }] : content;
    setText((t) => ({ ...t, [region]: [] }));
    window.setTimeout(() => {
      setText((t) => ({ ...t, [region]: segments }));
      setAt.current = Date.now();
    }, 80);
  }, []);

  const clear = useCallback(() => {
    setText({ status: [], reply: [], alert: [] });
  }, []);

  useEffect(() => {
    const onInput = () => {
      if (Date.now() - setAt.current > 1000) clear();
    };
    const onKey = (e: KeyboardEvent) => {
      if (!["Shift", "Control", "Alt", "Meta"].includes(e.key)) onInput();
    };
    document.addEventListener("input", onInput, true);
    document.addEventListener("pointerdown", onInput, true);
    document.addEventListener("keydown", onKey, true);
    return () => {
      document.removeEventListener("input", onInput, true);
      document.removeEventListener("pointerdown", onInput, true);
      document.removeEventListener("keydown", onKey, true);
    };
  }, [clear]);

  const value = useMemo(() => ({ announce, clear }), [announce, clear]);
  return (
    <AnnounceContext.Provider value={value}>
      {children}
      {createPortal(
        <>
          <div id="sr-status" role="status" aria-live="polite" aria-atomic="true" className="sr-only">
            <Segments items={text.status} />
          </div>
          <div id="sr-reply" aria-live="polite" aria-atomic="true" className="sr-only">
            <Segments items={text.reply} />
          </div>
          <div id="sr-alert" role="alert" aria-atomic="true" className="sr-only">
            <Segments items={text.alert} />
          </div>
        </>,
        document.body,
      )}
    </AnnounceContext.Provider>
  );
}

function Segments({ items }: { items: AnnounceSegment[] }) {
  return (
    <>
      {items.map((s, i) =>
        s.lang ? (
          <span key={i} lang={s.lang}>
            {s.text}
          </span>
        ) : (
          <span key={i}>{s.text}</span>
        ),
      )}
    </>
  );
}

export function useAnnounce(): AnnounceValue {
  const v = useContext(AnnounceContext);
  if (!v) throw new Error("useAnnounce outside AnnounceProvider");
  return v;
}

export function useOnline(): boolean {
  const [online, setOnline] = useState(() =>
    typeof navigator === "undefined" ? true : navigator.onLine,
  );
  useEffect(() => {
    const up = () => setOnline(true);
    const down = () => setOnline(false);
    window.addEventListener("online", up);
    window.addEventListener("offline", down);
    return () => {
      window.removeEventListener("online", up);
      window.removeEventListener("offline", down);
    };
  }, []);
  return online;
}

/** Focus a target by id after a state change, without stealing focus on
 * pointer activation (the design asks for a visible cue there too). */
export function focusId(id: string) {
  requestAnimationFrame(() => document.getElementById(id)?.focus());
}
