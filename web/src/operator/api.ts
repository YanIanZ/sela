// Operator API client. The session lives only in the __Host-sela_admin
// cookie; this module never stores or reads a token. Every mutation sends
// the same-origin marker the server requires.

const BASE = "/api/admin/v1";

export type Admin = {
  username: string;
  role: string;
  orgId: string;
  permissions: string[];
  /** Set for partner accounts (D-076): the one care organisation they act in. */
  careOrgId?: string;
  displayName?: string;
  partner?: boolean;
};

export type CareOrg = {
  id: string;
  kind: string;
  name: string;
  city?: string;
  description?: string;
  modes: string[];
  stages: string[];
  synthetic: boolean;
  status: "pending" | "verified" | "suspended";
  acceptsRequests: boolean;
  createdAt: string;
  verifiedAt?: string;
  members: number;
};

export type CareOrgMember = {
  username: string;
  role: string;
  displayName?: string;
  professionalType?: string;
  licenseNo?: string;
  licenseVerified: boolean;
  licenseVerifiedAt?: string;
  status: string;
};

export type CareProposal = {
  slots: string[];
  mode: "online" | "in_person";
  contact: string;
  note?: string;
};

export type SharedSummary = {
  label: string;
  purpose: string;
  summaryVersion: number;
  title: string;
  text: string;
  prepared?: { goal?: string; questions?: string[]; bring?: string[] };
  items?: { text: string; quote?: string; sourceLabel: string; position: number }[];
  sources: string[];
  createdAt: string;
  expiresAt: string;
};

export type PartnerCareRequest = {
  id: string;
  reference: string;
  practitionerId?: string;
  practitionerName?: string;
  mode: "online" | "in_person" | "either";
  state: "sent" | "proposed" | "declined" | "confirmed" | "completed" | "cancelled" | "expired";
  synthetic: boolean;
  proposal?: CareProposal;
  confirmedSlot?: string;
  declineReason?: string;
  version: number;
  contentAvailable: boolean;
  expiresAt: string;
  createdAt: string;
  updatedAt: string;
  summary?: SharedSummary;
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
  careOrgId?: string;
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

export type CommunityQueueItem = {
  kind: "post" | "reply";
  id: string;
  postId?: string;
  alias: string;
  body: string;
  status: "published" | "held" | "hidden" | "removed";
  labels: string[];
  reports: number;
  updatedAt: string;
};

export type Invite = {
  id: string;
  username: string;
  role: string;
  careOrgId: string;
  displayName?: string;
  professionalType?: string;
  licenseNo?: string;
  createdAt: string;
  expiresAt: string;
  acceptedAt?: string;
  revokedAt?: string;
  state: "open" | "accepted" | "revoked" | "expired";
};

export type Activation = {
  username: string;
  role: string;
  orgName: string;
  displayName?: string;
  secret: string;
  otpauthUrl: string;
};

export type Tasks = {
  network?: { pendingOrgs: number; pendingLicenses: number; openInvites: number };
  moderation?: number;
  directoryPending?: number;
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
  tasks: () => request<{ tasks: Tasks }>("/tasks").then((r) => r.tasks ?? {}),
  createInvite: (input: Record<string, unknown>) =>
    request<{ invite: Invite; activationUrl: string }>(
      "/network/invites",
      { method: "POST", body: JSON.stringify(input) },
      true,
    ),
  invites: (orgId: string) =>
    request<{ invites: Invite[] }>(`/network/orgs/${encodeURIComponent(orgId)}/invites`).then(
      (r) => r.invites ?? [],
    ),
  revokeInvite: (id: string) =>
    request<{ revoked: boolean }>(
      `/network/invites/${encodeURIComponent(id)}/revoke`,
      { method: "POST" },
      true,
    ),
  activationStart: (token: string) =>
    request<{ activation: Activation }>(
      "/activation",
      { method: "POST", body: JSON.stringify({ token }) },
      true,
    ).then((r) => r.activation),
  activationComplete: (token: string, password: string, totp: string) =>
    request<{ admin: Admin }>(
      "/activation/complete",
      { method: "POST", body: JSON.stringify({ token, password, totp }) },
      true,
    ).then((r) => r.admin),
  partnerMembers: () =>
    request<{ members: CareOrgMember[] }>("/partner/members").then((r) => r.members ?? []),
  networkOrgs: () =>
    request<{ orgs: CareOrg[]; realPartners: boolean }>("/network/orgs").then((r) => ({
      orgs: r.orgs ?? [],
      realPartners: r.realPartners,
    })),
  createNetworkOrg: (input: Record<string, unknown>) =>
    request<{ org: CareOrg }>(
      "/network/orgs",
      { method: "POST", body: JSON.stringify(input) },
      true,
    ).then((r) => r.org),
  setNetworkOrgStatus: (id: string, status: "verified" | "suspended") =>
    request<{ org: CareOrg }>(
      `/network/orgs/${encodeURIComponent(id)}/status`,
      { method: "POST", body: JSON.stringify({ status }) },
      true,
    ).then((r) => r.org),
  networkMembers: (id: string) =>
    request<{ members: CareOrgMember[] }>(
      `/network/orgs/${encodeURIComponent(id)}/members`,
    ).then((r) => r.members ?? []),
  verifyLicense: (username: string) =>
    request<{ username: string; licenseVerified: boolean }>(
      `/network/members/${encodeURIComponent(username)}/verify-license`,
      { method: "POST" },
      true,
    ),
  partnerOrg: () =>
    request<{ org: CareOrg; displayName: string }>("/partner/org"),
  partnerRequests: () =>
    request<{ requests: PartnerCareRequest[] }>("/partner/requests").then((r) => r.requests ?? []),
  partnerRequest: (id: string) =>
    request<{ request: PartnerCareRequest; canReadContent: boolean }>(
      `/partner/requests/${encodeURIComponent(id)}`,
    ),
  partnerRespond: (id: string, input: Record<string, unknown>) =>
    request<{ request: PartnerCareRequest }>(
      `/partner/requests/${encodeURIComponent(id)}/respond`,
      { method: "POST", body: JSON.stringify(input) },
      true,
    ).then((r) => r.request),
  partnerComplete: (id: string, expectedVersion: number) =>
    request<{ request: PartnerCareRequest }>(
      `/partner/requests/${encodeURIComponent(id)}/complete`,
      { method: "POST", body: JSON.stringify({ expectedVersion }) },
      true,
    ).then((r) => r.request),
  communityQueue: () =>
    request<{ items: CommunityQueueItem[] }>("/community/queue").then((r) => r.items ?? []),
  communityDecide: (kind: string, id: string, action: "restore" | "remove", reason: string) =>
    request<{ status: string }>(
      `/community/${encodeURIComponent(kind)}/${encodeURIComponent(id)}/decision`,
      { method: "POST", body: JSON.stringify({ action, reason }) },
      true,
    ),
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
      (r) => r.events ?? [],
    ),
  staff: () => request<{ staff: Staff[] }>("/staff").then((r) => r.staff ?? []),
  revokeStaff: (username: string, reason: string) =>
    request<{ username: string; status: string }>(
      `/staff/${encodeURIComponent(username)}/revoke`,
      { method: "POST", body: JSON.stringify({ reason }) },
      true,
    ),
  directory: (status = "") =>
    request<{ entries: DirectoryEntry[] }>(
      "/directory" + (status ? `?status=${encodeURIComponent(status)}` : ""),
    ).then((r) => r.entries ?? []),
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
    request<{ events: CareAudit[] }>("/directory/audit").then((r) => r.events ?? []),
  sandbox: () =>
    request<{ requests: SandboxRequest[] }>("/sandbox/requests").then((r) => r.requests ?? []),
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
