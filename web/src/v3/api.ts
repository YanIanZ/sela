import type { MeaningCheck, StaleInfo } from "./meaning";
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

export type Reflection = {
  id: string;
  turnId?: string;
  quote: string;
  text: string;
  reviewStatus: "unreviewed" | "confirmed" | "corrected" | "rejected";
  provenance: string;
  /** Meaning Diff of Sela's wording against the quote (D-078). */
  meaning?: MeaningCheck;
  createdAt: string;
  updatedAt: string;
};

export type Room = {
  id: string;
  save: boolean;
  version: number;
  turns: Turn[];
  reflections?: Reflection[];
  paused: boolean;
  busy: boolean;
  aiState: string;
  recordId?: string;
  /** Seconds until the temporary server hold is discarded. */
  holdExpiresIn?: number;
};

/** Events the turn stream may deliver before its final room. */
export type TurnStreamEvent =
  | { type: "status"; state: string }
  | { type: "chunk"; text: string };

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
  summaryState?: "draft" | "approved";
  sections?: {
    feeling: string;
    remember: string;
    agreed: string;
    askNext: string;
  };
  parentId?: string;
  createdAt: string;
  updatedAt: string;
  turns?: Turn[];
  paused?: boolean;
  /** Set when an invalidation reached this version (D-078). */
  stale?: StaleInfo;
  /** Meaning Diff of an AI claim against its quote (D-078). */
  meaning?: MeaningCheck;
};

/** One owned summary that pins a source record (content-free). */
export type DerivedSummaryImpact = {
  id: string;
  title: string;
  version: number;
  approved: boolean;
  shareCount: number;
};

/** GET /derived: what deleting one source record would affect. */
export type DerivedImpact = {
  sourceId: string;
  summaries: DerivedSummaryImpact[];
  totals: { summaries: number; shares: number };
};

export type ReadingSource = {
  id: string;
  title: string;
  publisher: string;
  url: string;
  licence: string;
};

export type ReadingListItem = {
  id: string;
  version: number;
  kind: string;
  topics: string[];
  audience: string;
  title: Localized;
  summary: Localized;
  readingTimeMinutes: { id: number; en: number };
  sources: ReadingSource[];
  reviewedOn: string;
};

export type ReadingSection = {
  heading: Localized;
  paragraphs: Localized[];
};

export type ReadingItem = ReadingListItem & {
  steps?: { id: string[]; en: string[] };
  why?: Localized;
  seekHelpWhen?: Localized;
  cautions?: Localized;
  sections?: ReadingSection[];
  source: {
    relation: string;
    locator: string;
    sources: ReadingSource[];
  };
  licence: {
    itemTextLicence: string;
    sourceLicence: string;
    commercialUse: string;
    note?: string;
  };
  attribution: Localized;
  adaptationNotice?: Localized;
  adaptationNote?: string;
  evidenceNote?: string;
  review: {
    status: string;
    reviewedOn: string;
    verifier: string;
  };
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

/** One name the owner marked private. The list is redacted before any AI
 * request; it is not a guarantee against re-identification by other details. */
export type PrivateName = {
  id: string;
  name: string;
  createdAt: string;
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

type RawStreamFrame = { event: string; data: string };

/** Parse one SSE frame; comments and keep-alive blocks yield null. */
function parseStreamFrame(block: string): RawStreamFrame | null {
  let event = "";
  let data: string[] | null = null;
  for (const line of block.replace(/\r/g, "").split("\n")) {
    if (line === "" || line.startsWith(":")) continue;
    if (line.startsWith("event:")) {
      event = line.slice(6).trim();
    } else if (line.startsWith("data:")) {
      const value = line.slice(5);
      (data ??= []).push(value.startsWith(" ") ? value.slice(1) : value);
    }
  }
  if (event === "" && data === null) return null;
  return { event: event || "message", data: data?.join("\n") ?? "" };
}

/**
 * POST a turn and read validated reply chunks as they arrive. The server
 * only sends text it has already released and stored, so a chunk is never a
 * provider token. The returned room is the same object the JSON route
 * returns and remains the single source of persisted turns.
 */
export async function streamTurn(
  path: string,
  body: unknown,
  onEvent: (event: TurnStreamEvent) => void,
  signal?: AbortSignal,
  timeoutMs = 110_000,
): Promise<Room> {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), timeoutMs);
  const forward = () => controller.abort();
  signal?.addEventListener("abort", forward, { once: true });
  try {
    const res = await fetch("/api/v3" + path, {
      method: "POST",
      credentials: "same-origin",
      signal: controller.signal,
      headers: {
        Accept: "text/event-stream",
        "X-Sela-Request": "1",
        "X-Sela-Language": document.documentElement.lang === "en" ? "en" : "id",
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      let data: { error?: { code?: string; message?: string; field?: string } } = {};
      try {
        data = await res.json();
      } catch {
        // a proxy can answer HTML; the status still decides
      }
      const code = data.error?.code ?? "stream_unavailable";
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
    if (!res.body || !(res.headers.get("Content-Type") ?? "").startsWith("text/event-stream")) {
      throw new V3Error("stream_unavailable", "Streaming is unavailable", res.status);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let room: Room | null = null;
    const handle = (frame: RawStreamFrame) => {
      let payload: unknown = {};
      try {
        payload = frame.data === "" ? {} : JSON.parse(frame.data);
      } catch {
        throw new V3Error("stream_invalid", "The reply stream was malformed");
      }
      if (frame.event === "status") {
        const state = (payload as { state?: unknown }).state;
        onEvent({ type: "status", state: typeof state === "string" ? state : "" });
      } else if (frame.event === "chunk") {
        const text = (payload as { text?: unknown }).text;
        if (typeof text === "string" && text !== "") {
          onEvent({ type: "chunk", text });
        }
      } else if (frame.event === "done") {
        room = payload as Room;
      } else if (frame.event === "error") {
        const code = (payload as { code?: unknown }).code;
        throw new V3Error(
          typeof code === "string" && code !== "" ? code : "internal",
          "The reply stream failed",
        );
      }
    };
    try {
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        let at = buffer.indexOf("\n\n");
        while (at >= 0) {
          const block = buffer.slice(0, at);
          buffer = buffer.slice(at + 2);
          const frame = parseStreamFrame(block);
          if (frame) handle(frame);
          at = buffer.indexOf("\n\n");
        }
      }
      buffer += decoder.decode();
      if (buffer.trim() !== "") {
        const frame = parseStreamFrame(buffer);
        if (frame) handle(frame);
      }
    } finally {
      reader.releaseLock();
    }
    if (room === null) {
      throw new V3Error("stream_incomplete", "The stream ended before the reply was ready");
    }
    return room;
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
