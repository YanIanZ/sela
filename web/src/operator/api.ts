// Operator API client. The session lives only in the __Host-sela_admin
// cookie; this module never stores or reads a token. Every mutation sends
// the same-origin marker the server requires.

const BASE = "/api/admin/v1";

export type Admin = {
  username: string;
  role: string;
  orgId: string;
  permissions: string[];
};

export type AISpend = {
  monthlyLimitUsd: number;
  reserveUsd: number;
  monthStart: string;
  committedUsd: number;
  actualUsd: number;
  openReservations: number;
  operations: number;
  generatedAt: string;
};

export type TaskSpend = {
  task: string;
  committedUsd: number;
  actualUsd: number;
  operations: number;
};

export type Overview = {
  activeStaff: number;
  liveSessions: number;
  auditLast24h: number;
  generatedAt: string;
  cacheEnabled: boolean;
  databaseReady: boolean;
  version: string;
  aiSpend?: AISpend;
  aiSpendByTask?: TaskSpend[];
};

export type AuditEvent = {
  id: string;
  actor?: string;
  action: string;
  objectKind?: string;
  objectId?: string;
  oldStatus?: string;
  newStatus?: string;
  reason?: string;
  createdAt: string;
};

export type CacheHealth = {
  enabled: boolean;
  ok: boolean;
  latencyMs?: number;
  error?: string;
};

export class AdminError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string) {
    super(code);
    this.name = "AdminError";
    this.status = status;
    this.code = code;
  }
}

type ErrorBody = { error?: { code?: string } };

async function request<T>(
  path: string,
  init: RequestInit = {},
  mutation = false,
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body !== undefined) headers.set("Content-Type", "application/json");
  if (mutation) headers.set("X-Sela-Request", "1");
  let res: Response;
  try {
    res = await fetch(BASE + path, {
      ...init,
      credentials: "same-origin",
      headers,
    });
  } catch {
    throw new AdminError(0, "network");
  }
  let body: unknown = null;
  try {
    body = await res.json();
  } catch {
    body = null;
  }
  if (!res.ok) {
    const code = (body as ErrorBody | null)?.error?.code ?? "internal";
    throw new AdminError(res.status, code);
  }
  return (body as { data: T }).data;
}

export const adminApi = {
  me: () => request<{ admin: Admin }>("/me").then((r) => r.admin),
  login: (username: string, password: string, totp: string) =>
    request<{ admin: Admin }>(
      "/sessions",
      { method: "POST", body: JSON.stringify({ username, password, totp }) },
      true,
    ).then((r) => r.admin),
  logout: () =>
    request<{ ok: boolean }>("/sessions/current", { method: "DELETE" }, true),
  overview: () => request<Overview>("/overview"),
  audit: (limit = 50) =>
    request<{ events: AuditEvent[] }>(`/audit?limit=${limit}`).then(
      (r) => r.events,
    ),
  cacheHealth: () =>
    request<{ cache: CacheHealth }>("/cache/health").then((r) => r.cache),
  invalidateCache: () =>
    request<{ invalidated: string }>(
      "/cache/invalidate",
      { method: "POST" },
      true,
    ).then((r) => r.invalidated),
};
