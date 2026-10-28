import type { ReactNode } from "react";

/*
 * Inline outline icons on a 24px grid, 1.75px stroke, round joins, drawn in
 * currentColor. They follow the Lucide shapes named in the design direction
 * (ISC licence). No icon package is installed; this keeps the CSP and the
 * bundle small. Every icon is decorative: the visible label carries meaning.
 */
export type IconName =
  | "home"
  | "talk"
  | "notes"
  | "support"
  | "help"
  | "quick-exit"
  | "menu"
  | "send"
  | "stop"
  | "mic"
  | "quote"
  | "pencil"
  | "ai"
  | "interpretation"
  | "pause"
  | "eye-off"
  | "bookmark"
  | "bookmark-x"
  | "share"
  | "eye"
  | "check"
  | "calendar"
  | "ban"
  | "clock"
  | "square-check"
  | "trash"
  | "download"
  | "copy"
  | "arrow-left"
  | "arrow-right"
  | "close"
  | "settings"
  | "key"
  | "info"
  | "alert"
  | "wifi-off"
  | "phone"
  | "whatsapp"
  | "globe"
  | "chevron-down"
  | "lock"
  | "user"
  | "plus"
  | "external"
  | "print"
  | "spark"
  | "shield"
  | "search";

const paths: Record<IconName, ReactNode> = {
  home: <path d="M3 10.5 12 3l9 7.5V20a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1Z" />,
  talk: <path d="M21 11.5a8.4 8.4 0 0 1-8.5 8.3H8l-5 2.2 1.6-4.6A8.2 8.2 0 0 1 3 11.5 8.4 8.4 0 0 1 11.5 3 8.4 8.4 0 0 1 21 11.5Z" />,
  notes: (
    <>
      <path d="M6 3h12a2 2 0 0 1 2 2v16H6a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3Z" />
      <path d="M8 8h8M8 12h8M8 16h5" />
    </>
  ),
  support: (
    <>
      <path d="M11 4.5 9.8 3.3a3.4 3.4 0 0 0-4.8 4.8L12 15l7-6.9a3.4 3.4 0 0 0-4.8-4.8L13 4.5a1.4 1.4 0 0 1-2 0Z" />
      <path d="M12 15v6M8 18h8" />
    </>
  ),
  help: (
    <>
      <circle cx="12" cy="12" r="9" />
      <circle cx="12" cy="12" r="4" />
      <path d="m5.6 5.6 3.8 3.8m5.2 5.2 3.8 3.8m0-12.8-3.8 3.8m-5.2 5.2-3.8 3.8" />
    </>
  ),
  "quick-exit": (
    <>
      <path d="M9 4H5a1 1 0 0 0-1 1v14a1 1 0 0 0 1 1h4" />
      <path d="M14 8l4 4-4 4M8 12h10" />
    </>
  ),
  menu: <path d="M4 7h16M4 12h16M4 17h16" />,
  send: <path d="m3 11 18-8-8 18-2-8Z" />,
  stop: <rect x="6" y="6" width="12" height="12" rx="1" />,
  mic: (
    <>
      <rect x="9" y="3" width="6" height="11" rx="3" />
      <path d="M5 11a7 7 0 0 0 14 0M12 18v3M9 21h6" />
    </>
  ),
  quote: <path d="M9 7H5v5h4c0 3-1.6 4.4-4 5 4.6 0 7-2.7 7-7V7Zm10 0h-4v5h4c0 3-1.6 4.4-4 5 4.6 0 7-2.7 7-7V7Z" />,
  pencil: <path d="M4 20h4L20 8l-4-4L4 16v4Zm9-13 4 4" />,
  ai: (
    <>
      <rect x="4" y="4" width="16" height="16" rx="3" />
      <path d="M8 15l2.2-6h1.6L14 15M8.7 13h4.6M16.5 9v6" />
    </>
  ),
  interpretation: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M9.6 9a2.6 2.6 0 0 1 5 1c0 1.7-2.6 2-2.6 3.4M12 17h.01" />
    </>
  ),
  pause: (
    <>
      <rect x="7" y="5" width="3.5" height="14" rx="1" />
      <rect x="13.5" y="5" width="3.5" height="14" rx="1" />
    </>
  ),
  "eye-off": (
    <>
      <path d="M3 3l18 18" />
      <path d="M10.6 6.2A9.9 9.9 0 0 1 12 6c5 0 9 6 9 6a17 17 0 0 1-2.4 3.1M6.6 6.6A17.7 17.7 0 0 0 3 12s4 6 9 6a9.7 9.7 0 0 0 4.1-.9" />
      <path d="M10 10a2.8 2.8 0 0 0 4 4" />
    </>
  ),
  bookmark: <path d="M6 3h12v18l-6-4-6 4Z" />,
  "bookmark-x": (
    <>
      <path d="M6 3h12v18l-6-4-6 4Z" />
      <path d="m9.5 8.5 5 5m0-5-5 5" />
    </>
  ),
  share: (
    <>
      <circle cx="18" cy="5" r="2.5" />
      <circle cx="6" cy="12" r="2.5" />
      <circle cx="18" cy="19" r="2.5" />
      <path d="m8.3 10.8 7.4-4.4M8.3 13.2l7.4 4.4" />
    </>
  ),
  eye: (
    <>
      <path d="M3 12s4-6.5 9-6.5S21 12 21 12s-4 6.5-9 6.5S3 12 3 12Z" />
      <circle cx="12" cy="12" r="2.6" />
    </>
  ),
  check: <path d="m4.5 12.5 4.5 4.5L19.5 6" />,
  calendar: (
    <>
      <rect x="3" y="5" width="18" height="16" rx="2" />
      <path d="M8 3v4M16 3v4M3 10h18M8.5 15l2 2 4-4" />
    </>
  ),
  ban: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="m5.6 5.6 12.8 12.8" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3.5 2" />
    </>
  ),
  "square-check": (
    <>
      <rect x="3" y="3" width="18" height="18" rx="3" />
      <path d="m7.5 12 3 3 6-6" />
    </>
  ),
  trash: <path d="M4 7h16M9 7V4h6v3m-8 0 1 14h8l1-14M10 11v6M14 11v6" />,
  download: <path d="M12 3v12m-4.5-4.5L12 15l4.5-4.5M4 20h16" />,
  copy: (
    <>
      <rect x="9" y="9" width="12" height="12" rx="2" />
      <path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1" />
    </>
  ),
  "arrow-left": <path d="M20 12H4m6-6-6 6 6 6" />,
  "arrow-right": <path d="M4 12h16m-6-6 6 6-6 6" />,
  close: <path d="M5 5l14 14M19 5 5 19" />,
  settings: (
    <>
      <path d="M4 7h10M18 7h2M4 17h2M10 17h10" />
      <circle cx="16" cy="7" r="2" />
      <circle cx="8" cy="17" r="2" />
    </>
  ),
  key: (
    <>
      <circle cx="8" cy="8" r="4" />
      <path d="m11 11 9 9m-4-4 2-2m-5-1 2-2" />
    </>
  ),
  info: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v6M12 7.5h.01" />
    </>
  ),
  alert: (
    <>
      <path d="M12 3 2.7 20h18.6Z" />
      <path d="M12 9v5M12 17h.01" />
    </>
  ),
  "wifi-off": (
    <>
      <path d="M3 3l18 18M8.5 8.8a13 13 0 0 1 11 2.2M5 11a13 13 0 0 1 1.8-1.1M9.5 15.5a5 5 0 0 1 5 0M12 19h.01" />
    </>
  ),
  phone: <path d="M7 3H4a1 1 0 0 0-1 1c0 9.4 7.6 17 17 17a1 1 0 0 0 1-1v-3l-4-2-2 2a14 14 0 0 1-6-6l2-2Z" />,
  whatsapp: (
    <>
      <path d="M20 11.5A8 8 0 0 1 7.4 18.4L3 20l1.6-4.3A8 8 0 1 1 20 11.5Z" />
      <path d="M9 8.5c0 3 2.5 5.5 5.5 5.5M9 8.5l1.5-1L12 9l-1 1.2M14.5 14l-1 1 1.5 1.5 1-1.2" />
    </>
  ),
  globe: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3c2.5 2.5 4 5.5 4 9s-1.5 6.5-4 9c-2.5-2.5-4-5.5-4-9s1.5-6.5 4-9Z" />
    </>
  ),
  "chevron-down": <path d="m6 9 6 6 6-6" />,
  lock: (
    <>
      <rect x="4" y="10" width="16" height="11" rx="2" />
      <path d="M8 10V7a4 4 0 0 1 8 0v3" />
    </>
  ),
  user: (
    <>
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21a8 8 0 0 1 16 0" />
    </>
  ),
  plus: <path d="M12 5v14M5 12h14" />,
  external: <path d="M14 4h6v6M20 4l-9 9M19 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h5" />,
  print: (
    <>
      <path d="M6 9V3h12v6M6 18H4a1 1 0 0 1-1-1v-6a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v6a1 1 0 0 1-1 1h-2" />
      <rect x="6" y="14" width="12" height="7" />
    </>
  ),
  spark: <path d="M12 3v4M12 17v4M3 12h4M17 12h4M5.6 5.6l2.8 2.8M15.6 15.6l2.8 2.8M18.4 5.6l-2.8 2.8M8.4 15.6l-2.8 2.8" />,
  shield: <path d="M12 3 4 6v5c0 5 3.4 8.5 8 10 4.6-1.5 8-5 8-10V6Z" />,
  search: (
    <>
      <circle cx="11" cy="11" r="7" />
      <path d="m16 16 5 5" />
    </>
  ),
};

export function Icon({
  name,
  size = 20,
  className,
}: {
  name: IconName;
  size?: number;
  className?: string;
}) {
  return (
    <svg
      className={className ? `s-icon ${className}` : "s-icon"}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {paths[name]}
    </svg>
  );
}

export function Mark({ size = 32 }: { size?: number }) {
  // Four spines on a shelf, one per place: Beranda, Cerita, Catatanku, Dukungan.
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      aria-hidden="true"
      focusable="false"
      className="s-mark"
    >
      <rect x="2" y="7" width="5" height="22" rx="1" style={{ fill: "var(--mark-1, currentColor)" }} />
      <rect x="8.5" y="3" width="5" height="26" rx="1" style={{ fill: "var(--mark-2, currentColor)" }} />
      <rect x="15" y="9" width="5" height="20" rx="1" style={{ fill: "var(--mark-3, currentColor)" }} />
      <rect x="22" y="5" width="5" height="24" rx="1" transform="rotate(8 24.5 29)" style={{ fill: "var(--mark-4, currentColor)" }} />
    </svg>
  );
}
