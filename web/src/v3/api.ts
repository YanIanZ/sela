/*
 * The /api/v3 client. One place for envelopes, the mutation guard headers,
 * request timeouts and the stable error codes. No component calls fetch
 * directly.
 */
export type Lang = "id" | "en";
export type Localized = { id: string; en: string };

export type GuestStorage = {
  inactivePurgeAt: string;
  maxAgePurgeAt: string;
  inactiveTtlHours: number;
  maxAgeDays: number;
  storage: {
    location: string;
    region: string | null;
    encrypted: boolean;
    boundTo: string;
  };
};

export type GuardianConsent = {
  required: boolean;
  status: "missing" | "active" | "withdrawn";
};

export type Profile = {
  id: string;
  kind: "guest" | "account";
  username: string | null;
  locale: Lang;
  ageBand: string;
  ageBandConfirmedAt?: string | null;
  version: number;
  createdAt: string;
  lastActiveAt: string;
  guest: GuestStorage | null;
  recovery: { remaining: number; generatedAt: string; acknowledged: boolean } | null;
  guardianConsent: GuardianConsent | null;
  actionsRequired: string[];
  environment: string;
  synthetic: boolean;
};

export type Policy = {
  version: string;
  title: Localized;
  body: Localized[];
};

export type Consent = {
  purpose: string;
  required?: boolean;
  granted: boolean;
  outdated: boolean;
  revision: number;
  policyVersion?: string | null;
  currentPolicyVersion: string;
  decidedAt?: string | null;
};

export type ConsentEvent = {
  id: string;
  purpose: string;
  decision: "granted" | "withdrawn";
  policyVersion: string;
  source: string;
  decidedAt: string;
};

export type Turn = {
  role: "user" | "assistant";
  text: string;
  provenance: string;
  reviewStatus?: string;
  model?: string;
  createdAt: string;
  aiProvenance?: {
    draftProvenance?: string;
    route?: string;
    fallbackReason?: string;
    generator?: {
      provider?: string;
      modelId?: string;
      servedModel?: string;
      servedRevision?: string;
    };
    verification?: {
      provider?: string;
      modelId?: string;
      servedModel?: string;
      status?: string;
      provenance?: string;
      label?: string;
      released?: number;
      withheldAll?: boolean;
    };
  };
};

export type Room = {
  id: string;
  save: boolean;
  version: number;
  turns: Turn[];
  paused: boolean;
  busy: boolean;
  aiState: string;
  recordId?: string;
};

export type Entry = {
  id: string;
  kind: "note" | "checkin" | "conversation" | "memory" | "summary" | "followup";
  title: string;
  text: string;
  mood?: string;
  energy?: string;
  tags?: string[];
  day?: string;
  version: number;
  provenance: string;
  reviewStatus: string;
  safety: string;
  createdAt: string;
  updatedAt: string;
  turns?: Turn[];
  paused?: boolean;
};

export type Options = {
  username: { minLength: number; maxLength: number };
  password: { minLength: number; maxLength: number };
  recoveryCodes: { count: number; groups: number; groupLength: number };
  ageBands: { allowed: string[] };
  guest: { available: boolean; inactiveTtlHours: number; maxAgeDays: number };
  sessions: {
    idleHours: number;
    absoluteHours: number;
    trustedIdleDays: number;
    trustedAbsoluteDays: number;
  };
  guardianConsentRequired: boolean;
  environment: string;
  synthetic: boolean;
};

export type SessionView = {
  id: string;
  current: boolean;
  kind: string;
  trustedDevice: boolean;
  device: { browser: string; os: string };
  createdAt: string;
  lastSeenAt: string;
  idleExpiresAt: string;
  absoluteExpiresAt: string;
};

export type AuditEvent = {
  id: string;
  at: string;
  type: string;
  actor: string;
  currentSession?: boolean | null;
  detail?: Record<string, unknown>;
};

export type AuthResult = {
  user: Profile;
  session?: SessionView;
  recoveryCodes?: string[];
};

export class V3Error extends Error {
  constructor(
    public code: string,
    message: string,
    public status = 0,
    public field?: string,
  ) {
    super(message);
    this.name = "V3Error";
  }
}

export function isAbort(error: unknown): boolean {
  return (
    (error instanceof DOMException && error.name === "AbortError") ||
    (error instanceof Error && error.name === "AbortError")
  );
}

/** Notified on a 401 only when a session was active, so a first visit never
 * claims that a session ended. */
export const AUTH_EXPIRED_EVENT = "sela:auth-expired";
let authActive = false;
export function setAuthActive(active: boolean) {
  authActive = active;
}

function requestHeaders(body: unknown): HeadersInit {
  return {
    Accept: "application/json",
    "X-Sela-Request": "1",
    "X-Sela-Language": document.documentElement.lang === "en" ? "en" : "id",
    ...(body === undefined ? {} : { "Content-Type": "application/json" }),
  };
}

export async function api<T>(
  path: string,
  body?: unknown,
  method = body === undefined ? "GET" : "POST",
  signal?: AbortSignal,
  timeoutMs = 45_000,
): Promise<T> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), timeoutMs);
  const forward = () => controller.abort();
  signal?.addEventListener("abort", forward, { once: true });
  try {
    const res = await fetch("/api/v3" + path, {
      method,
      credentials: "same-origin",
      signal: controller.signal,
      headers: requestHeaders(body),
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    let data: { data?: unknown; error?: { code?: string; message?: string; field?: string } } = {};
    try {
      data = await res.json();
    } catch {
      // A proxy or an outage can answer HTML; the status still decides.
    }
    if (!res.ok) {
      const code = data.error?.code ?? "internal";
      if (res.status === 401 && code === "not_signed_in" && authActive) {
        window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
      }
      throw new V3Error(
        code,
        data.error?.message ?? `HTTP ${res.status}`,
        res.status,
        data.error?.field,
      );
    }
    return data.data as T;
  } catch (error) {
    if (isAbort(error) && signal?.aborted) throw error;
    if (isAbort(error)) throw new V3Error("timeout", "Request timed out");
    throw error;
  } finally {
    window.clearTimeout(timeout);
    signal?.removeEventListener("abort", forward);
  }
}

/** Download an export as a Blob. The response is not an envelope. */
export async function download(
  path: string,
  filename: string,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch("/api/v3" + path, {
    credentials: "same-origin",
    signal,
    headers: {
      Accept: "*/*",
      "X-Sela-Language": document.documentElement.lang === "en" ? "en" : "id",
    },
  });
  if (!res.ok) {
    let code = "internal";
    let message = `HTTP ${res.status}`;
    try {
      const data = await res.json();
      code = data.error?.code ?? code;
      message = data.error?.message ?? message;
    } catch {
      // keep the status text
    }
    if (res.status === 401 && authActive) {
      window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
    }
    throw new V3Error(code, message, res.status);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 2000);
}

export function localDay(d = new Date()): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(
    d.getDate(),
  ).padStart(2, "0")}`;
}
