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

export type Staff = {
  username: string;
  role: string;
  orgId?: string;
  status: string;
  createdAt: string;
  lastLoginAt?: string;
  liveSessions: number;
};

export type DirectoryEntry = {
  id: string;
  kind: string;
  name: string;
  city?: string;
  description?: string;
  contactUrl?: string;
  sourceUrl: string;
  sourcePublisher: string;
  sourceAccessedAt: string;
  status: string;
  synthetic: boolean;
  version: number;
  verifiedAt?: string;
  reverifyAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type SandboxRequest = {
  id: string;
  organization: string;
  professional?: string;
  service?: string;
  status: string;
  note?: string;
  synthetic: boolean;
  version: number;
  createdAt: string;
  updatedAt: string;
};

export type CareAudit = {
  id: string;
  actor?: string;
  action: string;
  objectKind: string;
  objectId: string;
  oldStatus?: string;
  newStatus?: string;
  reason?: string;
  createdAt: string;
};

export type ReadingLocalized = { id: string; en: string };

export type ReadingSourceRef = {
  id: string;
  title: string;
  publisher: string;
  url: string;
  licence: string;
};

export type ReadingDetail = {
  id: string;
  version: number;
  kind: string;
  topics: string[];
  audience: string;
  title: ReadingLocalized;
  summary: ReadingLocalized;
  steps?: { id: string[]; en: string[] };
  why?: ReadingLocalized;
  seekHelpWhen?: ReadingLocalized;
  cautions?: ReadingLocalized;
  sections?: {
    heading: ReadingLocalized;
    paragraphs: ReadingLocalized[];
  }[];
  readingTimeMinutes: { id: number; en: number };
  source: {
    relation: string;
    locator: string;
    sources: ReadingSourceRef[];
  };
  licence: {
    itemTextLicence: string;
    sourceLicence: string;
    commercialUse: string;
    note?: string;
  };
  attribution: ReadingLocalized;
  adaptationNotice?: ReadingLocalized;
  adaptationNote?: string;
  evidenceNote?: string;
  review: { status: string; reviewedOn: string; verifier: string };
};

export type ReadingEmbeddedReview = {
  status: string;
  humanEditor?: string;
  reviewedOn?: string;
  approvedTextSha256?: string;
  teenReview?: string;
  verifierStatus: string;
  verifierNote?: string;
  publishable: boolean;
};

export type ReadingApproval = {
  slug: string;
  reviewerName: string;
  reviewedOn: string;
  note?: string;
  verifierStatus: string;
  contentHash: string;
  approvedBy: string;
  approvedAt: string;
  revokedAt?: string;
  version: number;
};

export type ReadingAdminItem = {
  slug: string;
  detail: ReadingDetail;
  contentHash: string;
  embeddedReview: ReadingEmbeddedReview;
  approval?: ReadingApproval;
  hashMatches: boolean;
  published: boolean;
};

export type ReadingAudit = {
  id: string;
  actor?: string;
  action: string;
  objectKind: string;
  objectId: string;
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
  staff: () => request<{ staff: Staff[] }>("/staff").then((r) => r.staff),
  revokeStaff: (username: string, reason: string) =>
    request<{ username: string; status: string }>(
      `/staff/${encodeURIComponent(username)}/revoke`,
      { method: "POST", body: JSON.stringify({ reason }) },
      true,
    ),
  directory: (status = "") =>
    request<{ entries: DirectoryEntry[] }>(
      "/directory" + (status ? `?status=${encodeURIComponent(status)}` : ""),
    ).then((r) => r.entries),
  createDirectory: (input: Record<string, unknown>) =>
    request<DirectoryEntry>("/directory", { method: "POST", body: JSON.stringify(input) }, true),
  directoryAction: (
    id: string,
    action: "submit" | "verify" | "reject" | "unpublish",
    expectedVersion: number,
    extra: Record<string, unknown> = {},
  ) =>
    request<DirectoryEntry>(
      `/directory/${encodeURIComponent(id)}/${action}`,
      { method: "POST", body: JSON.stringify({ expectedVersion, ...extra }) },
      true,
    ),
  careAudit: () =>
    request<{ events: CareAudit[] }>("/directory/audit").then((r) => r.events),
  sandbox: () =>
    request<{ requests: SandboxRequest[] }>("/sandbox/requests").then((r) => r.requests),
  createSandbox: (input: Record<string, unknown>) =>
    request<SandboxRequest>(
      "/sandbox/requests",
      { method: "POST", body: JSON.stringify(input) },
      true,
    ),
  transitionSandbox: (id: string, expectedVersion: number, status: string, note = "") =>
    request<SandboxRequest>(
      `/sandbox/requests/${encodeURIComponent(id)}/transition`,
      { method: "POST", body: JSON.stringify({ expectedVersion, status, note }) },
      true,
    ),
  cacheHealth: () =>
    request<{ cache: CacheHealth }>("/cache/health").then((r) => r.cache),
  invalidateCache: () =>
    request<{ invalidated: string }>(
      "/cache/invalidate",
      { method: "POST" },
      true,
    ).then((r) => r.invalidated),
  reading: () =>
    request<{ items: ReadingAdminItem[] }>("/reading").then((r) => r.items),
  readingAudit: () =>
    request<{ events: ReadingAudit[] }>("/reading/audit").then((r) => r.events),
  approveReading: (
    slug: string,
    input: {
      reviewerName: string;
      reviewedOn: string;
      note: string;
      verifierStatus: "supported";
      contentHash: string;
      confirm: boolean;
    },
  ) =>
    request<{ slug: string; approval: ReadingApproval }>(
      `/reading/${encodeURIComponent(slug)}/approve`,
      { method: "POST", body: JSON.stringify(input) },
      true,
    ),
  revokeReading: (slug: string, reason: string) =>
    request<{ slug: string; approval: ReadingApproval }>(
      `/reading/${encodeURIComponent(slug)}/revoke`,
      { method: "POST", body: JSON.stringify({ reason }) },
      true,
    ),
};
