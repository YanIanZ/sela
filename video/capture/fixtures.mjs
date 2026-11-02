// Synthetic "Nadia" fixture for filming the real SELA web UI.
//
// Everything here is fictional (docs/demo/SYNTHETIC_NADIA_FIXTURE.md). No real
// person, record, partner or credential. The router answers /api/v3 with the
// response shapes the web client reads (web/src/v3/api.ts and the screens), so
// what the film shows is exactly what the shipped components render for these
// states. Backend behaviour (stale/revoke/cancel) is not simulated here as a
// claim: it is asserted by the Go tests listed in docs/QA_REPORT.md.

export const N1 = "Sepulang kuliah aku sering capek. Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.";
export const N2 = "Hari ini aku agak kewalahan dengan tugas. Aku belum tahu bagian mana yang paling membuatku lelah.";
export const N3 = "Aku ingin tahu bagaimana menjelaskan minggu yang melelahkan ini. Aku juga ingin menyiapkan beberapa pertanyaan kalau suatu saat ngobrol dengan konselor.";
export const TALK_TURN = "Kadang aku butuh waktu sendiri.";
export const QUOTE_N1 = "Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman-temanku.";
// F0: SCRIPTED WRONG INTERPRETATION. Not live Meta output. Labelled as a test fixture in the film.
// Sela addresses the user as "kamu"; CompareMeaning treats that as the user.
export const F0 = "Kamu selalu butuh waktu sendiri dan menghindari teman.";
// Nadia's correction, in her own words.
export const CORRECTED = "Kadang aku butuh waktu sendiri, tapi aku tetap senang bertemu teman.";
export const Q1 = "Bagaimana aku bisa menjelaskan apa yang kurasakan tanpa harus langsung menyimpulkan penyebabnya?";
export const Q2 = "Apa informasi dari catatan pribadiku yang paling berguna untuk kubawa saat konsultasi?";
export const STEP = "Tuliskan dua pertanyaan untuk konsultasi.";

const T = (d) => `${d}`; // ISO strings are fixtures, not claimed activity

export const profile = {
  id: "user-nadia-synthetic",
  kind: "guest",
  username: null,
  locale: "id",
  ageBand: "18_plus",
  version: 3,
  createdAt: T("2026-10-27T01:00:00Z"),
  lastActiveAt: T("2026-11-02T03:00:00Z"),
  guest: {
    inactivePurgeAt: "2026-11-09T03:00:00Z",
    maxAgePurgeAt: "2026-11-26T01:00:00Z",
    inactiveTtlHours: 168,
    maxAgeDays: 30,
    storage: { location: "server_database", region: null, encrypted: true, boundTo: "this_browser_cookie" },
  },
  recovery: null,
  guardianConsent: null,
  actionsRequired: [],
  environment: "test",
  synthetic: true,
};

const policy = (title) => ({ version: "v1", title: { id: title, en: title }, body: [{ id: "Dokumen kebijakan sintetis untuk demo.", en: "Synthetic policy document for the demo." }] });
const policies = { terms_privacy: policy("Ketentuan & privasi"), guest_storage: policy("Penyimpanan tamu"), ai_processing: policy("Pemrosesan AI"), sharing: policy("Berbagi") };

const options = {
  username: { minLength: 3, maxLength: 32 },
  password: { minLength: 15, maxLength: 256 },
  recoveryCodes: { count: 10, groups: 4, groupLength: 4 },
  ageBands: { allowed: ["18_plus"] },
  guest: { available: true, inactiveTtlHours: 168, maxAgeDays: 30 },
  sessions: { idleHours: 2, absoluteHours: 12, trustedIdleDays: 7, trustedAbsoluteDays: 30 },
  guardianConsentRequired: false,
  environment: "test",
  synthetic: true,
};

const consent = (purpose, granted, required = false) => ({ purpose, required, granted, outdated: false, revision: granted ? 1 : 0, policyVersion: granted ? "v1" : null, currentPolicyVersion: "v1", decidedAt: granted ? "2026-10-27T01:00:00Z" : null });
const consents = [consent("terms_privacy", true, true), consent("guest_storage", true, true), consent("personal_memory", true), consent("ai_processing", true), consent("sharing", true), consent("condition_map", false)];

const rec = (o) => ({ provenance: "user_original", reviewStatus: "confirmed", safety: "ok", ruleset: "r1", version: 1, ...o });
const n1 = rec({ id: "rec-n1", kind: "note", title: "Sepulang kuliah", text: N1, day: "2026-10-27", createdAt: "2026-10-27T11:20:00Z", updatedAt: "2026-10-27T11:20:00Z" });
const n2 = rec({ id: "rec-n2", kind: "checkin", title: "Cek perasaan", text: N2, mood: "heavy", energy: "low", day: "2026-10-30", createdAt: "2026-10-30T12:05:00Z", updatedAt: "2026-10-30T12:05:00Z" });
const n3 = rec({
  id: "rec-n3",
  kind: "conversation",
  title: "Minggu yang melelahkan",
  text: N3,
  day: "2026-11-01",
  createdAt: "2026-11-01T13:40:00Z",
  updatedAt: "2026-11-01T13:40:00Z",
  turns: [{ role: "user", text: N3, provenance: "user_original", createdAt: "2026-11-01T13:40:00Z" }],
});

// Exact output of internal/ledger.CompareMeaning(QUOTE_N1, F0) at dfdffd0
// (see video/evidence/meaning-diff/). Not hand-written.
const diffF0 = {
  version: "sela-meaning-2026.11.01-1",
  critical: true,
  differences: [
    { code: "frequency", severity: "critical", source: "kadang", wording: "selalu" },
    { code: "added_content", severity: "review", wording: "menghindari" },
  ],
};

// The memory that rests on the (fixture) claim. Corrected => stale/invalid.
function memory(state) {
  return rec({
    id: "mem-f0",
    kind: "memory",
    title: "Tentang waktu sendiri",
    text: state.corrected ? CORRECTED : F0,
    provenance: state.corrected ? "user_correction" : "unconfirmed_interpretation",
    reviewStatus: state.corrected ? "corrected" : state.memoryReview ?? "confirmed",
    version: state.corrected ? 3 : state.memoryReview === "unreviewed" ? 1 : 2,
    meaning: state.corrected ? undefined : diffF0,
    sourceRefs: [{ recordId: "rec-n1", version: 1, quote: QUOTE_N1 }],
    createdAt: "2026-11-01T13:45:00Z",
    updatedAt: state.corrected ? "2026-11-02T09:12:00Z" : "2026-11-01T13:50:00Z",
  });
}

function summaryOld(state) {
  return rec({
    id: "sum-old",
    kind: "summary",
    title: "Persiapan konsultasi (versi awal)",
    text: "",
    version: 1,
    summaryState: "approved",
    approvedVersion: 1,
    sourceRefs: [{ recordId: "rec-n1", version: 1 }, { recordId: "mem-f0", version: 2 }],
    prepared: { goal: "Aku ingin bisa menjelaskan rasa lelahku.", questions: "", bring: "" },
    items: [],
    summaryState: state.corrected ? "draft" : "approved",
    stale: state.corrected ? { reason: "claim_corrected", eventId: "mev-1", at: "2026-11-02T09:12:00Z" } : undefined,
    createdAt: "2026-11-01T14:00:00Z",
    updatedAt: "2026-11-01T14:10:00Z",
  });
}

function summaryNew(state) {
  return rec({
    id: "sum-new",
    kind: "summary",
    title: "Hal yang ingin kuceritakan",
    text: "",
    version: 2,
    summaryState: state.approved ? "approved" : "draft",
    approvedVersion: state.approved ? 2 : undefined,
    sourceRefs: [{ recordId: "rec-n1", version: 1 }, { recordId: "rec-n2", version: 1 }],
    prepared: { goal: "Aku ingin menyiapkan kalimat dan pertanyaan untuk membantu menjelaskan pengalaman pribadiku.", questions: `${Q1}\n${Q2}`, bring: "" },
    items: [
      { id: "it-1", text: "Sesekali aku perlu waktu sendiri setelah kuliah, tetapi aku masih senang bersama teman.", quote: QUOTE_N1, sourceRef: { recordId: "rec-n1", version: 1 }, provenance: "user_original", reviewStatus: "confirmed", position: 0 },
      { id: "it-2", text: "Belakangan aku juga merasa kewalahan dengan tugas, dan aku belum tahu penyebabnya.", quote: N2, sourceRef: { recordId: "rec-n2", version: 1 }, provenance: "user_original", reviewStatus: "confirmed", position: 1 },
    ],
    createdAt: "2026-11-02T09:30:00Z",
    updatedAt: "2026-11-02T09:41:00Z",
  });
}

function passport(state) {
  const s = summaryNew(state);
  return {
    summaryId: s.id,
    summaryVersion: 2,
    state: state.approved ? "approved" : "draft",
    shareable: Boolean(state.approved),
    sources: [
      { recordId: "rec-n1", version: 1, kind: "note", label: "Sepulang kuliah", current: true },
      { recordId: "rec-n2", version: 1, kind: "checkin", label: "Cek perasaan", current: true },
    ],
    recipient: {
      summaryVersion: 2,
      title: s.title,
      text: "",
      prepared: s.prepared,
      items: s.items.map((i) => ({ text: i.text, quote: i.quote, sourceLabel: i.sourceRef.recordId === "rec-n1" ? "Sepulang kuliah · v1" : "Cek perasaan · v1", position: i.position })),
      sources: ["Sepulang kuliah · v1", "Cek perasaan · v1"],
    },
    updatedAt: s.updatedAt,
  };
}

const assistantTurn = (text, at) => ({ role: "assistant", text, provenance: "test_double", reviewStatus: "unreviewed", createdAt: at });

function room(state) {
  const turns = [];
  if (state.sent) {
    turns.push({ role: "user", text: TALK_TURN, provenance: "user_original", createdAt: "2026-11-02T08:58:00Z" });
    turns.push(assistantTurn("Terima kasih sudah cerita. Butuh waktu sendiri itu wajar. Waktu sendiri seperti apa yang biasanya paling membantumu?", "2026-11-02T08:58:10Z"));
  }
  // The talk card shows that an interpretation is tentative and reviewable.
  // Its Meaning Diff is the real CompareMeaning output (no differences).
  const reflections = state.reflection
    ? [
        {
          id: "rf-1",
          quote: TALK_TURN,
          text: "Kamu kadang butuh waktu sendiri.",
          reviewStatus: state.reflectionReview ?? "unreviewed",
          provenance: "test_double",
          meaning: { version: "sela-meaning-2026.11.01-1", critical: false, differences: [] },
          createdAt: "2026-11-02T09:00:00Z",
          updatedAt: "2026-11-02T09:00:00Z",
        },
      ]
    : [];
  return { id: "room-1", save: true, version: 4, turns, reflections, paused: false, busy: false, aiState: "available", recordId: "rec-n3", holdExpiresIn: 3000 };
}

// Same shape and counts as the preview asserted in TestMeaningCorrectionPropagates:
// one summary, its share plus the care request's hidden share, one care request.
const impact = { origin: { kind: "memory", id: "mem-f0", version: 2 }, memories: 0, summaries: 1, shares: 2, careRequests: 1, actions: 0 };
const event = () => ({ eventId: "mev-1", reason: "claim_corrected", ...impact, at: new Date(Date.now() - 20_000).toISOString() });

function timeline(state) {
  const it = (r, kind, over = {}) => ({
    source: { recordId: r.id, version: r.version, kind },
    occurredAt: r.createdAt,
    updatedAt: r.updatedAt,
    title: r.title,
    excerpt: r.text,
    truncated: false,
    provenance: "user_original",
    meaningStatus: "current",
    ...over,
  });
  return { items: [it(n3, "conversation"), it(n2, "checkin", { mood: "heavy" }), it(n1, "note")], nextCursor: null };
}

function steps(state) {
  if (!state.stepSaved) return [];
  return [
    {
      id: "step-1",
      version: 1,
      text: STEP,
      status: "chosen",
      sourceRefs: [{ recordId: "rec-n3", version: 1 }],
      origin: { kind: "user_written" },
      review: { userApproved: true, stale: false },
      scope: "private",
      safety: "ok",
      createdAt: "2026-11-02T09:20:00Z",
      updatedAt: "2026-11-02T09:20:00Z",
    },
  ];
}

const org = {
  id: "org-demo",
  kind: "clinic",
  name: "Klinik Cerita Demo",
  city: "Kota Demo",
  description: "Layanan fiktif untuk demo. Bukan klinik sungguhan.",
  modes: ["online"],
  stages: ["everyday", "attention", "support"],
  synthetic: true,
  practitioners: [{ id: "pr-demo", displayName: "Konselor Demo B", professionalType: "counsellor", licenseVerified: false }],
};

function careRequests(state) {
  const base = { orgId: org.id, orgName: org.name, orgSynthetic: true, practitionerId: "pr-demo", practitionerName: "Konselor Demo B", mode: "online", expiresAt: "2026-11-09T09:50:00Z" };
  const out = [];
  if (state.request) {
    out.push({
      ...base,
      id: "cr-new",
      reference: "DEMO-2B7K",
      shareId: "sh-care-new",
      summaryId: "sum-new",
      summaryVersion: 2,
      state: state.request,
      proposal: state.request === "proposed" || state.request === "confirmed" ? { slots: ["2026-11-05T03:00:00Z", "2026-11-06T07:00:00Z"], mode: "online", contact: "Tautan video dari layanan simulasi", note: "Ini layanan simulasi untuk demo." } : undefined,
      confirmedSlot: state.request === "confirmed" ? "2026-11-05T03:00:00Z" : undefined,
      version: 2,
      createdAt: "2026-11-02T09:50:00Z",
      updatedAt: "2026-11-02T10:05:00Z",
    });
  }
  if (state.oldRequest) {
    out.push({
      ...base,
      id: "cr-old",
      reference: "DEMO-1A9C",
      shareId: "sh-care-old",
      summaryId: "sum-old",
      summaryVersion: 1,
      state: state.corrected ? "cancelled" : "sent",
      version: state.corrected ? 2 : 1,
      createdAt: "2026-11-01T14:20:00Z",
      updatedAt: state.corrected ? "2026-11-02T09:12:00Z" : "2026-11-01T14:20:00Z",
    });
  }
  return out;
}

function shares(state) {
  const out = [
    {
      id: "sh-old",
      summaryId: "sum-old",
      summaryVersion: 1,
      label: "Konselor (simulasi)",
      purpose: "consultation",
      version: 2,
      state: state.corrected ? "revoked" : "active",
      endReason: state.corrected ? "meaning_invalidated" : undefined,
      createdAt: "2026-11-01T14:12:00Z",
      expiresAt: "2026-11-15T14:12:00Z",
      revokedAt: state.corrected ? "2026-11-02T09:12:00Z" : undefined,
      openedCount: 1,
      acknowledgedCount: 0,
      lastOpenedAt: "2026-11-01T15:00:00Z",
      acknowledged: false,
    },
  ];
  if (state.newShare) {
    out.unshift({
      id: "sh-new",
      summaryId: "sum-new",
      summaryVersion: 2,
      label: "Konselor (simulasi)",
      purpose: "consultation",
      version: 2,
      state: state.newShare,
      endReason: state.newShare === "revoked" ? "owner_revoked" : undefined,
      createdAt: "2026-11-02T09:45:00Z",
      expiresAt: "2026-11-16T09:45:00Z",
      revokedAt: state.newShare === "revoked" ? "2026-11-02T10:20:00Z" : undefined,
      openedCount: 1,
      acknowledgedCount: 1,
      lastOpenedAt: "2026-11-02T10:00:00Z",
      acknowledged: true,
    });
  }
  return out;
}

export function createRouter(initial = {}) {
  const state = { sent: false, reflection: false, corrected: false, approved: false, stepSaved: false, oldRequest: true, request: null, newShare: null, ...initial };
  const router = (method, path, body) => {
    const p = path.split("?")[0];
    const key = `${method} ${p}`;
    switch (key) {
      case "GET /policies": return { data: { policies } };
      case "GET /auth/options": return { data: options };
      case "GET /me": return { data: profile };
      case "GET /me/consents": return { data: { consents } };
      case "GET /me/consents/history": return { data: { events: [] } };
      case "GET /records": {
        const records = [n3, n2, n1];
        if (path.includes("kind=memory")) return { data: { records: [memory(state)] } };
        if (path.includes("kind=summary")) return { data: { records: [summaryNew(state), summaryOld(state)] } };
        return { data: { records } };
      }
      case "GET /conversations": return { data: { conversations: state.sent ? [room(state)] : [] } };
      case "GET /memory": return { data: { enabled: true, items: [memory(state)] } };
      case "GET /meaning/lineage": return { data: { lineage: { node: { kind: "memory", id: "mem-f0", version: memory(state).version }, parents: [{ kind: "record", id: "rec-n1", version: 1, current: true }], descendants: state.corrected ? [] : [{ kind: "summary", id: "sum-old", version: 1 }, { kind: "share", id: "sh-old", version: 1 }, { kind: "share", id: "sh-care-old", version: 1 }, { kind: "care_request", id: "cr-old", version: 1 }] } } };
      case "GET /meaning/impact": return { data: { impact: state.corrected ? { ...impact, memories: 0, summaries: 0, shares: 0, careRequests: 0 } : impact } };
      case "GET /meaning/events": return { data: { events: state.corrected ? [event()] : [] } };
      case "GET /context-timeline": return { data: timeline(state) };
      case "GET /action-steps": return { data: { actionSteps: steps(state) } };
      case "GET /consultation-summaries": return { data: { summaries: [summaryNew(state), summaryOld(state)] } };
      case "GET /shares": return { data: { shares: shares(state) } };
      case "GET /care-requests": return { data: { requests: careRequests(state) } };
      case "GET /care-network": return { data: { orgs: [org] } };
      case "GET /condition-map": return { status: 404, error: { code: "not_found" } };
    }
    if (method === "GET" && p.startsWith("/conversations/")) return { data: room(state) };
    if (method === "GET" && p === "/memory/mem-f0") return { data: memory(state) };
    // Demo recording: sending a turn and asking for reflections. The stream
    // route answers "unavailable" so the client uses its JSON compatibility path.
    if (method === "POST" && p === "/conversations/room-1/turns/stream") return { status: 404, error: { code: "stream_unavailable" } };
    if (method === "POST" && p === "/conversations/room-1/turns") { state.sent = true; return { data: room(state), delay: 1600 }; }
    if (method === "POST" && p === "/conversations/room-1/reflections") { state.reflection = true; return { data: room(state), delay: 1100 }; }
    if (method === "POST" && p === "/consultation-summaries/sum-new/approve") { state.approved = true; return { data: summaryNew(state) }; }
    if (method === "POST" && p === "/action-steps") { state.stepSaved = true; return { data: steps(state)[0] }; }
    if (method === "POST" && p === "/care-requests/cr-new/confirm") { state.request = "confirmed"; return { data: { request: careRequests(state)[0] } }; }
    if (method === "POST" && p === "/care-requests/cr-new/cancel") { state.request = "cancelled"; return { data: { request: careRequests(state)[0] } }; }
    if (method === "POST" && /^\/shares\/sh-new\/revoke$/.test(p)) { state.newShare = "revoked"; return { data: { share: shares(state)[0] } }; }
    if (method === "PATCH" && /^\/conversations\/room-1\/reflections\//.test(p)) { state.reflectionReview = body?.action === "confirm" ? "confirmed" : body?.action === "reject" ? "rejected" : "corrected"; return { data: room(state) }; }
    if (method === "PATCH" && p === "/memory/mem-f0") { state.corrected = true; return { data: memory(state) }; }
    if (method === "PATCH" && p === "/memory/mem-f0/review") { state.corrected = true; return { data: memory(state) }; }
    if (method === "GET" && p.startsWith("/records/rec-")) return { data: [n1, n2, n3].find((r) => p.endsWith(r.id)) };
    if (method === "GET" && /\/consultation-summaries\/sum-new\/passport$/.test(p)) return { data: { passport: passport(state) } };
    if (method === "GET" && p === "/consultation-summaries/sum-new") return { data: summaryNew(state) };
    if (method === "GET" && p === "/consultation-summaries/sum-old") return { data: summaryOld(state) };
    if (method === "GET" && /\/shares\/[^/]+\/events$/.test(p)) return { data: { events: [{ kind: "created", at: "2026-11-01T14:12:00Z" }, { kind: "opened", at: "2026-11-01T15:00:00Z" }, ...(state.corrected ? [{ kind: "revoked", at: "2026-11-02T09:12:00Z" }] : [])] } };
    return undefined;
  };
  return { state, router };
}
