import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import axe from "axe-core";
import { I18nProvider } from "../i18n";
import { RouterProvider } from "../lib/router";
import { V3App } from "../v3/App";
import type { Profile } from "../v3/api";

const text = (value: string) => ({ id: value, en: value });

const policies = {
  terms_privacy: {
    version: "terms-v1",
    title: { id: "Ketentuan & Privasi", en: "Terms & Privacy" },
    body: [text("Draf ketentuan."), text("Data disimpan di server Sela.")],
  },
  guest_storage: {
    version: "guest-v1",
    title: { id: "Penyimpanan tamu", en: "Guest storage" },
    body: [text("Data tamu dihapus otomatis.")],
  },
  guardian_information: {
    version: "guardian-v1",
    title: { id: "Informasi untuk wali", en: "Information for a guardian" },
    body: [text("Wali tidak mendapat akses ke isi cerita.")],
  },
  ai_processing: {
    version: "ai-v1",
    title: { id: "Balasan AI · Meta", en: "AI replies · Meta" },
    body: [
      text("Hanya tulisan yang diperlukan untuk membalas yang dikirim."),
      text("Tanpa izin, Sela tidak membalas dengan AI."),
    ],
  },
};

const options = {
  username: { minLength: 3, maxLength: 32 },
  password: { minLength: 15, maxLength: 256 },
  recoveryCodes: { count: 10, groups: 4, groupLength: 4 },
  ageBands: { allowed: ["18_plus", "15_17", "unknown"] },
  guest: { available: true, inactiveTtlHours: 168, maxAgeDays: 30 },
  sessions: {
    idleHours: 2,
    absoluteHours: 12,
    trustedIdleDays: 7,
    trustedAbsoluteDays: 30,
  },
  guardianConsentRequired: true,
  environment: "development",
  synthetic: false,
};

function profile(over: Partial<Profile> = {}): Profile {
  return {
    id: "user-test",
    kind: "guest",
    username: null,
    locale: "id",
    ageBand: "18_plus",
    version: 1,
    createdAt: "2026-10-27T00:00:00Z",
    lastActiveAt: "2026-10-27T00:00:00Z",
    guest: {
      inactivePurgeAt: "2026-11-03T00:00:00Z",
      maxAgePurgeAt: "2026-11-26T00:00:00Z",
      inactiveTtlHours: 168,
      maxAgeDays: 30,
      storage: {
        location: "server_database",
        region: null,
        encrypted: true,
        boundTo: "this_browser_cookie",
      },
    },
    recovery: null,
    guardianConsent: null,
    actionsRequired: [],
    environment: "development",
    synthetic: false,
    ...over,
  };
}

const consents = [
  {
    purpose: "terms_privacy",
    required: true,
    granted: true,
    outdated: false,
    revision: 1,
    currentPolicyVersion: "terms-v1",
  },
  {
    purpose: "guest_storage",
    required: true,
    granted: true,
    outdated: false,
    revision: 1,
    currentPolicyVersion: "guest-v1",
  },
  {
    purpose: "ai_processing",
    required: false,
    granted: false,
    outdated: false,
    revision: 0,
    currentPolicyVersion: "ai-v1",
  },
];

type Reply = { status?: number; data?: unknown; error?: string } | "unauth" | Response;
type Handler = (body: unknown, url: URL) => Reply | Promise<Reply>;

function json(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function sse(frames: Array<{ event: string; data: unknown }>) {
  return new Response(
    frames.map((f) => `event: ${f.event}\ndata: ${JSON.stringify(f.data)}\n\n`).join(""),
    {
      status: 200,
      headers: { "Content-Type": "text/event-stream; charset=utf-8" },
    },
  );
}

function mockApi(handlers: Record<string, Handler>, signedIn: Profile | null) {
  const calls: Array<{ method: string; path: string; body: unknown; search: string }> = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
      const url = new URL(String(input), "http://localhost");
      const method = (init.method ?? "GET").toUpperCase();
      const path = url.pathname.replace(/^\/api\/v3/, "");
      const body = typeof init.body === "string" ? JSON.parse(init.body) : undefined;
      calls.push({ method, path, body, search: url.search });
      if (path === "/policies") return json({ data: { policies } });
      if (path === "/auth/options") return json({ data: options });
      const handler = handlers[`${method} ${path}`];
      if (!handler) {
        if (path === "/me" && signedIn) return json({ data: signedIn });
        if (path === "/me") {
          return json(
            { error: { code: "not_signed_in", message: "Not signed in" } },
            401,
          );
        }
        return json({ error: { code: "not_found", message: "Not found" } }, 404);
      }
      const reply = await handler(body, url);
      if (reply instanceof Response) return reply;
      if (reply === "unauth") {
        return json(
          { error: { code: "not_signed_in", message: "Not signed in" } },
          401,
        );
      }
      if (reply.error) {
        return json({ error: { code: reply.error, message: reply.error } }, reply.status ?? 400);
      }
      return json({ data: reply.data ?? null }, reply.status ?? 200);
    }),
  );
  return calls;
}

function renderPath(path: string, lang: "id" | "en" = "id") {
  window.history.replaceState({}, "", path);
  return render(
    <I18nProvider initial={lang}>
      <RouterProvider>
        <V3App />
      </RouterProvider>
    </I18nProvider>,
  );
}

beforeEach(() => {
  try {
    window.localStorage.clear();
    window.sessionStorage.clear();
  } catch {
    // jsdom without storage
  }
  document.documentElement.removeAttribute("data-text-scale");
  document.documentElement.removeAttribute("data-scheme");
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("public entry and onboarding", () => {
  it("opens the information dashboard without login before personal Home", async () => {
    const calls = mockApi({}, null);
    renderPath("/");
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Dashboard Sela",
      }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Butuh bantuan sekarang" }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole("button", { name: /Keluar cepat/ }).length).toBeGreaterThan(0);
    expect(screen.getByRole("link", { name: "Masuk ke Beranda" })).toHaveAttribute("href", "/start/age");
    expect(screen.getByRole("heading", { name: "Jelajahi fitur" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Dari sini ke Beranda" })).toBeInTheDocument();
    expect(calls.some(c => c.path === "/records" || c.path === "/conversations" || c.method !== "GET")).toBe(false);
    expect(screen.queryByText(/data sintetis/i)).not.toBeInTheDocument();
  });

  it("keeps the information portal public in English at its own route", async () => {
    const calls = mockApi({}, null);
    renderPath("/dashboard", "en");
    expect(await screen.findByRole("heading", { name: "Sela dashboard" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Continue to Home" })).toHaveAttribute("href", "/start/age");
    expect(screen.getByRole("heading", { name: "You stay in control" })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/dashboard");
    expect(calls.some(c => c.path === "/records" || c.method !== "GET")).toBe(false);
  });

  it("blocks an under-15 age band before anything is created", async () => {
    const calls = mockApi({}, null);
    renderPath("/start/age");
    fireEvent.click(
      await screen.findByRole("radio", { name: "Di bawah 15 tahun" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Lanjut" }));
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: /Sela belum bisa dipakai di usiamu/,
      }),
    ).toBeInTheDocument();
    expect(calls.some((c) => c.path === "/auth/guest")).toBe(false);
  });

  it("creates a guest only after the storage and consent choices", async () => {
    const user = profile();
    const calls = mockApi(
      {
        "POST /auth/guest": () => ({ status: 201, data: { user } }),
        "GET /me": () => ({ data: user }),
      },
      null,
    );
    renderPath("/start/age");
    fireEvent.click(await screen.findByRole("radio", { name: "18 tahun ke atas" }));
    fireEvent.click(screen.getByRole("button", { name: "Lanjut" }));

    expect(
      await screen.findByRole("heading", { level: 1, name: "Di mana datamu disimpan?" }),
    ).toBeInTheDocument();
    expect(calls.some((c) => c.path === "/auth/guest")).toBe(false);

    fireEvent.click(screen.getByRole("radio", { name: /Mulai sebagai tamu/ }));
    fireEvent.click(screen.getByRole("radio", { name: /Simpan di Catatanku/ }));
    fireEvent.click(screen.getByLabelText(/Aku sudah membaca dan menyetujui/));
    fireEvent.click(screen.getByLabelText(/Aku mengerti cara penyimpanan/));
    fireEvent.click(screen.getByRole("button", { name: "Mulai sebagai tamu" }));

    expect(
      await screen.findByRole("heading", { level: 1, name: "Balasan AI dan pemeriksa" }),
    ).toBeInTheDocument();
    const guest = calls.find((c) => c.path === "/auth/guest");
    expect(guest?.body).toMatchObject({ ageBand: "18_plus", locale: "id" });

    mockApi({}, user);
    fireEvent.click(screen.getByRole("link", { name: "Lihat beranda dulu" }));
    expect(
      await screen.findByRole("heading", { level: 1, name: "Beranda" }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /Mulai bercerita/ }).length).toBeGreaterThan(0);
  });
});

describe("accounts and errors", () => {
  it("shows a uniform sign-in error and keeps both values", async () => {
    mockApi(
      {
        "POST /auth/login": () => ({ status: 401, error: "invalid_credentials" }),
      },
      null,
    );
    renderPath("/signin");
    const username = await screen.findByLabelText("Nama pengguna");
    const password = screen.getByLabelText("Kata sandi");
    fireEvent.change(username, { target: { value: "pelangi.senja" } });
    fireEvent.change(password, { target: { value: "kata sandi panjang sekali" } });
    fireEvent.click(screen.getByRole("button", { name: "Masuk" }));

    expect(
      await screen.findByText("Nama pengguna atau kata sandi tidak cocok."),
    ).toBeInTheDocument();
    expect(username).toHaveValue("pelangi.senja");
    expect(password).toHaveValue("kata sandi panjang sekali");
  });
});

describe("home, notes and conversations", () => {
  it("shows the greeting, three actions and the latest note without streaks or scores", async () => {
    const user = profile();
    const note = {
      id: "note-1",
      kind: "note",
      title: "Catatan sintetis",
      text: "Isi catatan sintetis.",
      version: 1,
      provenance: "user_original",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt: "2026-10-27T10:00:00Z",
      updatedAt: "2026-10-27T10:00:00Z",
    };
    mockApi(
      {
        "GET /records": () => ({ data: { records: [note] } }),
        "GET /conversations": () => ({ data: { conversations: [] } }),
      },
      user,
    );
    renderPath("/home");
    expect(await screen.findByRole("heading", { level: 1, name: "Beranda" })).toBeInTheDocument();
    expect(
      screen.getByText(/Selamat (pagi|siang|sore|malam)\. Apa yang ingin kamu lakukan sekarang\?/),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Mulai bercerita/ })).toHaveAttribute("href", "/talk/new");
    expect(within(screen.getByRole("main")).getByRole("link", { name: /Tulis catatan/ })).toHaveAttribute("href", "/notes/write");
    expect(within(screen.getByRole("main")).getByRole("link", { name: /Cek perasaan/ })).toHaveAttribute("href", "/notes/check-in");
    expect(await screen.findByText("Isi catatan sintetis.")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 2, name: "Terbaru di Catatanku" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Buka" })).toHaveAttribute("href", "/notes/memory");
    expect(screen.queryByText(/Tidak harus rapi/)).not.toBeInTheDocument();
    expect(
      screen.queryByText(/streak|beruntun|poin|skor|level|lencana|badge/i),
    ).not.toBeInTheDocument();
  });

  it("shows one primary action and no record blocks on a first visit", async () => {
    mockApi(
      {
        "GET /records": () => ({ data: { records: [] } }),
        "GET /conversations": () => ({ data: { conversations: [] } }),
      },
      profile(),
    );
    renderPath("/home");
    expect(await screen.findByRole("heading", { level: 1, name: "Beranda" })).toBeInTheDocument();
    expect(
      await screen.findByText(
        "Belum ada yang tersimpan. Apa pun yang kamu simpan akan muncul di Catatanku.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { level: 2, name: "Terbaru di Catatanku" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { level: 2, name: "Yang Sela ingat" }),
    ).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /Mulai bercerita/ })).toHaveLength(1);
  });

  it("saves a note only with the storage choice and opens its detail", async () => {
    const user = profile();
    const note = {
      id: "note-1",
      kind: "note",
      title: "",
      text: "Tulisan sintetis yang aku pilih untuk disimpan.",
      version: 1,
      provenance: "user_original",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt: "2026-10-27T10:00:00Z",
      updatedAt: "2026-10-27T10:00:00Z",
    };
    const calls = mockApi(
      {
        "GET /records": () => ({ data: { records: [] } }),
        "POST /records": () => ({ status: 201, data: note }),
        "GET /records/note-1": () => ({ data: note }),
      },
      user,
    );
    renderPath("/notes/write");
    const field = await screen.findByLabelText("Catatanmu");
    fireEvent.change(field, { target: { value: note.text } });
    fireEvent.click(screen.getByLabelText(/Simpan catatan ini di Catatanku/));
    fireEvent.click(screen.getByRole("button", { name: "Simpan catatan" }));
    await waitFor(() =>
      expect(window.location.pathname).toBe("/notes/note-1"),
    );
    expect(await screen.findByText(note.text)).toBeInTheDocument();
    const create = calls.find((c) => c.method === "POST" && c.path === "/records");
    expect(create?.body).toMatchObject({
      kind: "note",
      storageGranted: true,
    });
  });

  it("checks derived impact and keeps summaries by default", async () => {
    const user = profile();
    const note = {
      id: "note-1",
      kind: "note",
      title: "Catatan sumber sintetis",
      text: "Isi catatan sumber sintetis.",
      version: 2,
      provenance: "user_correction",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt: "2026-10-27T09:00:00Z",
      updatedAt: "2026-10-27T10:00:00Z",
    };
    const impact = {
      sourceId: "note-1",
      summaries: [
        {
          id: "sum-1",
          title: "Ringkasan sintetis",
          version: 3,
          approved: true,
          shareCount: 2,
        },
      ],
      totals: { summaries: 1, shares: 2 },
    };
    const calls = mockApi(
      {
        "GET /records": () => ({ data: { records: [note] } }),
        "GET /records/note-1": () => ({ data: note }),
        "GET /derived": (_body, url) => {
          expect(url.searchParams.get("source")).toBe("note-1");
          return { data: impact };
        },
        "DELETE /records/note-1": () => ({ data: { deleted: true } }),
      },
      user,
    );
    renderPath("/notes/note-1");

    fireEvent.click(await screen.findByRole("button", { name: "Hapus catatan" }));
    const dialog = await screen.findByRole("alertdialog", {
      name: "Hapus catatan ini?",
    });
    expect(
      await within(dialog).findByText("Catatan ini dipakai di 1 ringkasan."),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText("2 tautan dibagikan dari ringkasan itu."),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("radio", { name: /Simpan ringkasan \(jadikan usang\)/ }),
    ).toBeChecked();
    expect(
      within(dialog).getByRole("radio", { name: /Hapus juga ringkasan turunannya/ }),
    ).not.toBeChecked();

    fireEvent.click(within(dialog).getByRole("button", { name: "Ya, hapus" }));
    await waitFor(() => expect(window.location.pathname).toBe("/notes"));
    const remove = calls.find(
      (c) => c.method === "DELETE" && c.path === "/records/note-1",
    );
    expect(remove?.body).toMatchObject({ expectedVersion: 2 });
    expect(remove?.search).toBe("?derived=keep");
  });

  it("shows the derived choice in English", async () => {
    const user = profile({ locale: "en" });
    const note = {
      id: "note-1",
      kind: "note",
      title: "Synthetic source note",
      text: "Synthetic source text.",
      version: 1,
      provenance: "user_original",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt: "2026-10-27T09:00:00Z",
      updatedAt: "2026-10-27T10:00:00Z",
    };
    mockApi(
      {
        "GET /records": () => ({ data: { records: [note] } }),
        "GET /records/note-1": () => ({ data: note }),
        "GET /derived": () => ({
          data: {
            sourceId: "note-1",
            summaries: [
              {
                id: "sum-1",
                title: "Synthetic summary",
                version: 1,
                approved: true,
                shareCount: 1,
              },
            ],
            totals: { summaries: 1, shares: 1 },
          },
        }),
      },
      user,
    );
    renderPath("/notes/note-1", "en");

    fireEvent.click(await screen.findByRole("button", { name: "Delete note" }));
    const dialog = await screen.findByRole("alertdialog", {
      name: "Delete this note?",
    });
    expect(
      await within(dialog).findByText("This note is used in 1 summary."),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByText("1 link is shared from that summary."),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("radio", {
        name: /Keep summaries \(mark them stale\)/,
      }),
    ).toBeChecked();
    expect(
      within(dialog).getByRole("radio", {
        name: /Delete the derived summaries too/,
      }),
    ).toBeInTheDocument();
  });

  it("sends derived=delete when the owner removes derived summaries", async () => {
    const user = profile();
    const note = {
      id: "note-1",
      kind: "note",
      title: "",
      text: "Isi catatan sumber sintetis.",
      version: 1,
      provenance: "user_original",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt: "2026-10-27T09:00:00Z",
      updatedAt: "2026-10-27T10:00:00Z",
    };
    const calls = mockApi(
      {
        "GET /records": () => ({ data: { records: [note] } }),
        "GET /records/note-1": () => ({ data: note }),
        "GET /derived": () => ({
          data: {
            sourceId: "note-1",
            summaries: [
              {
                id: "sum-1",
                title: "Ringkasan sintetis",
                version: 1,
                approved: false,
                shareCount: 0,
              },
            ],
            totals: { summaries: 1, shares: 0 },
          },
        }),
        "DELETE /records/note-1": () => ({ data: { deleted: true } }),
      },
      user,
    );
    renderPath("/notes/note-1");

    fireEvent.click(await screen.findByRole("button", { name: "Hapus catatan" }));
    const dialog = await screen.findByRole("alertdialog", {
      name: "Hapus catatan ini?",
    });
    const removeDerived = await within(dialog).findByRole("radio", {
      name: /Hapus juga ringkasan turunannya/,
    });
    fireEvent.click(removeDerived);
    expect(removeDerived).toBeChecked();
    expect(
      within(dialog).getByText(/Belum ada tautan yang dibagikan/),
    ).toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: "Ya, hapus" }));
    await waitFor(() => expect(window.location.pathname).toBe("/notes"));
    const remove = calls.find(
      (c) => c.method === "DELETE" && c.path === "/records/note-1",
    );
    expect(remove?.search).toBe("?derived=delete");
  });

  it("keeps the plain confirmation when no summary uses the note", async () => {
    const user = profile();
    const note = {
      id: "note-1",
      kind: "note",
      title: "",
      text: "Isi catatan sintetis.",
      version: 1,
      provenance: "user_original",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt: "2026-10-27T09:00:00Z",
      updatedAt: "2026-10-27T10:00:00Z",
    };
    const calls = mockApi(
      {
        "GET /records": () => ({ data: { records: [note] } }),
        "GET /records/note-1": () => ({ data: note }),
        "GET /derived": () => ({
          data: {
            sourceId: "note-1",
            summaries: [],
            totals: { summaries: 0, shares: 0 },
          },
        }),
        "DELETE /records/note-1": () => ({ data: { deleted: true } }),
      },
      user,
    );
    renderPath("/notes/note-1");

    fireEvent.click(await screen.findByRole("button", { name: "Hapus catatan" }));
    const dialog = await screen.findByRole("alertdialog", {
      name: "Hapus catatan ini?",
    });
    expect(
      await within(dialog).findByText(
        "Catatan ini tidak dipakai di ringkasan mana pun. Semua versinya ikut terhapus.",
      ),
    ).toBeInTheDocument();
    expect(within(dialog).queryByRole("radiogroup")).not.toBeInTheDocument();
    expect(screen.getAllByRole("alertdialog")).toHaveLength(1);

    fireEvent.click(within(dialog).getByRole("button", { name: "Ya, hapus" }));
    await waitFor(() => expect(window.location.pathname).toBe("/notes"));
    const remove = calls.find(
      (c) => c.method === "DELETE" && c.path === "/records/note-1",
    );
    expect(remove?.search).toBe("?derived=keep");
  });

  it("sends a message and names the generator on the released reply", async () => {
    const user = profile();
    const empty = {
      id: "room-1",
      save: false,
      version: 1,
      turns: [],
      paused: false,
      busy: false,
      aiState: "not_requested",
    };
    const answered = {
      ...empty,
      version: 3,
      aiState: "ready",
      turns: [
        {
          role: "user",
          text: "Aku merasa berat hari ini.",
          provenance: "user_original",
          createdAt: "2026-10-27T10:00:00Z",
        },
        {
          role: "assistant",
          text: "Kedengarannya hari ini berat. Apa yang paling terasa?",
          provenance: "live_meta",
          reviewStatus: "unreviewed",
          model: "meta-model",
          createdAt: "2026-10-27T10:00:05Z",
          aiProvenance: {
            draftProvenance: "live_meta",
            generator: { provider: "meta", modelId: "meta-model" },
            verification: { provenance: "live_meta_verifier", status: "released" },
          },
        },
      ],
    };
    const calls = mockApi(
      {
        "GET /conversations/room-1": () => ({ data: empty }),
        "POST /conversations/room-1/turns": () => ({ data: answered }),
      },
      user,
    );
    renderPath("/talk/room-1");
    const composer = await screen.findByLabelText("Tulis untuk Sela");
    fireEvent.change(composer, { target: { value: "Aku merasa berat hari ini." } });
    fireEvent.click(screen.getByRole("button", { name: "Kirim" }));
    expect(
      await screen.findByText("Kedengarannya hari ini berat. Apa yang paling terasa?"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Sela · balasan AI \(Meta\) · diperiksa oleh model Meta pemeriksa/),
    ).toBeInTheDocument();
    const send = calls.find((c) => c.path === "/conversations/room-1/turns");
    expect(send?.body).toMatchObject({
      text: "Aku merasa berat hari ini.",
      expectedVersion: 1,
    });
    // The stream route was attempted first and the JSON route is the fallback.
    expect(calls.some((c) => c.path === "/conversations/room-1/turns/stream")).toBe(true);
  });

  it("opens quick-help mode at once when a message reads as danger", async () => {
    const user = profile();
    const empty = {
      id: "room-1",
      save: false,
      version: 1,
      turns: [],
      paused: false,
      busy: false,
      aiState: "not_requested",
    };
    const paused = {
      ...empty,
      version: 2,
      paused: true,
      aiState: "paused",
      turns: [
        {
          role: "user",
          text: "Aku mau bunuh dia.",
          provenance: "user_original",
          createdAt: "2026-11-01T10:00:00Z",
        },
      ],
    };
    mockApi(
      {
        "GET /conversations/room-1": () => ({ data: empty }),
        "POST /conversations/room-1/turns": () => ({ data: paused }),
      },
      user,
    );
    renderPath("/talk/room-1");
    const composer = await screen.findByLabelText("Tulis untuk Sela");
    fireEvent.change(composer, { target: { value: "Aku mau bunuh dia." } });
    fireEvent.click(screen.getByRole("button", { name: "Kirim" }));
    const sheet = await screen.findByRole("alertdialog", { name: "Kamu tidak sendirian" });
    expect(within(sheet).getByRole("link", { name: /Lihat semua bantuan/ })).toHaveAttribute("href", "/help");
    expect(within(sheet).getByRole("button", { name: /Keluar cepat/ })).toBeInTheDocument();
  });

  it("renders validated stream chunks once and keeps the done room", async () => {
    const user = profile();
    const empty = {
      id: "room-1",
      save: false,
      version: 1,
      turns: [],
      paused: false,
      busy: false,
      aiState: "not_requested",
      holdExpiresIn: 1800,
    };
    const reply = "Kedengarannya hari ini berat.";
    const answered = {
      ...empty,
      version: 3,
      aiState: "ready",
      turns: [
        {
          role: "user",
          text: "Aku merasa berat hari ini.",
          provenance: "user_original",
          createdAt: "2026-10-27T10:00:00Z",
        },
        {
          role: "assistant",
          text: reply,
          provenance: "live_meta",
          reviewStatus: "unreviewed",
          model: "meta-model",
          createdAt: "2026-10-27T10:00:05Z",
        },
      ],
    };
    const calls = mockApi(
      {
        "GET /conversations/room-1": () => ({ data: empty }),
        "POST /conversations/room-1/turns/stream": () =>
          sse([
            { event: "status", data: { state: "thinking" } },
            { event: "chunk", data: { text: "Kedengarannya" } },
            { event: "chunk", data: { text: " hari ini berat." } },
            { event: "done", data: answered },
          ]),
      },
      user,
    );
    renderPath("/talk/room-1");
    const composer = await screen.findByLabelText("Tulis untuk Sela");
    fireEvent.change(composer, { target: { value: "Aku merasa berat hari ini." } });
    fireEvent.click(screen.getByRole("button", { name: "Kirim" }));
    expect(await screen.findByText(reply)).toBeInTheDocument();
    expect(screen.getAllByText(reply)).toHaveLength(1);
    expect(calls.some((c) => c.path === "/conversations/room-1/turns/stream")).toBe(true);
    expect(calls.some((c) => c.path === "/conversations/room-1/turns")).toBe(false);
  });

  it("warns before an unsaved hold expires and refreshes it", async () => {
    const user = profile();
    // Four minutes left: the component's threshold check runs on arrival.
    const empty = {
      id: "room-1",
      save: false,
      version: 1,
      turns: [],
      paused: false,
      busy: false,
      aiState: "not_requested",
      holdExpiresIn: 240,
    };
    let firstRead = true;
    const calls = mockApi(
      {
        "GET /conversations/room-1": () => {
          // "Tetap buka" is a real GET: the server restarts the idle timer.
          const data = firstRead ? empty : { ...empty, holdExpiresIn: 1800 };
          firstRead = false;
          return { data };
        },
      },
      user,
    );
    renderPath("/talk/room-1");
    const dialog = await screen.findByRole("alertdialog", {
      name: "Percakapan akan segera dibuang",
    });
    expect(within(dialog).getByRole("button", { name: "Tetap buka" })).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "Akhiri sekarang" })).toBeInTheDocument();

    const reads = () =>
      calls.filter((c) => c.method === "GET" && c.path === "/conversations/room-1").length;
    const before = reads();
    fireEvent.click(within(dialog).getByRole("button", { name: "Tetap buka" }));
    await waitFor(() => expect(reads()).toBeGreaterThan(before));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("ends the room from the hold warning", async () => {
    const user = profile();
    const empty = {
      id: "room-1",
      save: false,
      version: 1,
      turns: [],
      paused: false,
      busy: false,
      aiState: "not_requested",
      holdExpiresIn: 240,
    };
    const calls = mockApi(
      {
        "GET /conversations/room-1": () => ({ data: empty }),
        "DELETE /conversations/room-1": () => ({ data: { ended: true, saved: false } }),
      },
      user,
    );
    renderPath("/talk/room-1");
    const dialog = await screen.findByRole("alertdialog", {
      name: "Percakapan akan segera dibuang",
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Akhiri sekarang" }));
    expect(
      await screen.findByRole("heading", { name: "Percakapan selesai." }),
    ).toBeInTheDocument();
    expect(calls.some((c) => c.method === "DELETE" && c.path === "/conversations/room-1")).toBe(
      true,
    );
  });

  it("never warns for a saved room", async () => {
    const user = profile();
    const saved = {
      id: "room-1",
      save: true,
      version: 1,
      turns: [],
      paused: false,
      busy: false,
      aiState: "not_requested",
      holdExpiresIn: 240,
    };
    mockApi({ "GET /conversations/room-1": () => ({ data: saved }) }, user);
    renderPath("/talk/room-1");
    expect(await screen.findByLabelText("Tulis untuk Sela")).toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });
});

describe("personal memory", () => {
  it("opens a memory detail with its source and saves an edit", async () => {
    const user = profile();
    const memory = {
      id: "mem-1",
      kind: "memory",
      title: "Konteks sintetis",
      text: "Aku lebih tenang kalau ditanya dulu.",
      version: 2,
      provenance: "user_correction",
      reviewStatus: "confirmed",
      safety: "none",
      sourceRefs: [{ recordId: "note-1", version: 2 }],
      createdAt: "2026-10-27T09:00:00Z",
      updatedAt: "2026-10-27T10:00:00Z",
    };
    const source = {
      id: "note-1",
      kind: "note",
      title: "Catatan sumber sintetis",
      text: "Sumber sintetis.",
      version: 2,
      provenance: "user_original",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt: "2026-10-27T09:00:00Z",
      updatedAt: "2026-10-27T09:00:00Z",
    };
    const calls = mockApi(
      {
        "GET /memory": () => ({ data: { enabled: true, items: [memory] } }),
        "GET /memory/mem-1": () => ({ data: memory }),
        "GET /records/note-1": () => ({ data: source }),
        "PATCH /memory/mem-1": (body) => ({
          data: {
            ...memory,
            title: (body as Record<string, string>).title,
            text: (body as Record<string, string>).text,
            version: 3,
            updatedAt: "2026-10-27T11:00:00Z",
          },
        }),
      },
      user,
    );
    renderPath("/notes/memory");
    const link = await screen.findByRole("link", { name: /Konteks sintetis/ });
    expect(link).toHaveAttribute("href", "/notes/memory/mem-1");
    fireEvent.click(link);
    expect(
      await screen.findByRole("heading", { level: 1, name: "Konteks sintetis" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Dikonfirmasi")).toBeInTheDocument();
    expect(screen.getByText("Koreksimu")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Catatan sumber sintetis" }),
    ).toHaveAttribute("href", "/notes/note-1");

    fireEvent.click(screen.getByRole("button", { name: "Perbaiki" }));
    fireEvent.change(screen.getByLabelText("Konteks pilihanmu"), {
      target: { value: "Aku lebih tenang kalau ditanya dulu, pelan-pelan." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Simpan perbaikan" }));
    await waitFor(() =>
      expect(
        calls.some((c) => c.method === "PATCH" && c.path === "/memory/mem-1"),
      ).toBe(true),
    );
    expect(
      await screen.findByText("Aku lebih tenang kalau ditanya dulu, pelan-pelan."),
    ).toBeInTheDocument();
    const patch = calls.find((c) => c.method === "PATCH" && c.path === "/memory/mem-1");
    expect(patch?.body).toMatchObject({
      title: "Konteks sintetis",
      text: "Aku lebih tenang kalau ditanya dulu, pelan-pelan.",
      storageGranted: true,
      expectedVersion: 2,
    });
  });

  it("requests a proposal from a saved conversation and reviews it", async () => {
    const user = profile();
    const createdAt = "2026-10-27T09:00:00Z";
    const memory = {
      id: "mem-1",
      kind: "memory",
      title: "Konteks sintetis",
      text: "Aku lebih tenang kalau ditanya dulu.",
      version: 1,
      provenance: "user_original",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt,
      updatedAt: createdAt,
    };
    const conversation = {
      id: "conv-1",
      kind: "conversation",
      title: "Percakapan sintetis",
      text: "",
      version: 3,
      provenance: "user_original",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt,
      updatedAt: createdAt,
      turns: [],
    };
    const proposal = {
      id: "prop-1",
      kind: "memory",
      title: "",
      text: "Kamu suka menulis di pagi hari.",
      version: 1,
      provenance: "unconfirmed_interpretation",
      reviewStatus: "unreviewed",
      safety: "none",
      sourceRefs: [
        {
          recordId: "conv-1",
          version: 3,
          quote: "Aku suka menulis di pagi hari.",
          turnKey: "turn-0",
        },
      ],
      createdAt,
      updatedAt: createdAt,
    };
    let items: Array<Record<string, unknown>> = [memory];
    const calls = mockApi(
      {
        "GET /memory": () => ({ data: { enabled: true, items } }),
        "GET /records": () => ({ data: { records: [conversation] } }),
        "POST /memory/proposals": () => {
          items = [proposal, ...items];
          return { data: { proposals: [proposal], aiState: "released" } };
        },
        "PATCH /memory/prop-1/review": (body) => {
          const action = (body as Record<string, string>).action;
          const next = {
            ...proposal,
            reviewStatus:
              action === "confirm"
                ? "confirmed"
                : action === "correct"
                  ? "corrected"
                  : "rejected",
          };
          items = items.map((r) => (r.id === "prop-1" ? next : r));
          return { data: next };
        },
      },
      user,
    );
    renderPath("/notes/memory");
    fireEvent.click(await screen.findByRole("button", { name: "Minta usulan" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Buat usulan dari percakapan ini",
      }),
    );
    expect(
      await screen.findByText("Aku suka menulis di pagi hari."),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("Kamu suka menulis di pagi hari.").length,
    ).toBeGreaterThan(0);
    expect(screen.getByText("Tafsiran Sela")).toBeInTheDocument();
    expect(screen.getAllByText("Belum ditinjau").length).toBeGreaterThan(0);
    const request = calls.find(
      (c) => c.method === "POST" && c.path === "/memory/proposals",
    );
    expect(request?.body).toMatchObject({
      conversationId: "conv-1",
      expectedVersion: 3,
    });

    expect(screen.getByRole("button", { name: "Ingat" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Jangan" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ubah dulu" }));
    fireEvent.change(screen.getByLabelText("Kata-katamu"), {
      target: { value: "Menulis di pagi hari membantumu." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Simpan koreksi" }));
    await waitFor(() =>
      expect(
        calls.some(
          (c) => c.method === "PATCH" && c.path === "/memory/prop-1/review",
        ),
      ).toBe(true),
    );
    const patch = calls.find(
      (c) => c.method === "PATCH" && c.path === "/memory/prop-1/review",
    );
    expect(patch?.body).toMatchObject({
      expectedVersion: 1,
      action: "correct",
      text: "Menulis di pagi hari membantumu.",
    });
  });

  it("shows a proposal's quote and review status on its detail and confirms it", async () => {
    const user = profile();
    const createdAt = "2026-10-27T09:00:00Z";
    const proposal = {
      id: "prop-1",
      kind: "memory",
      title: "",
      text: "Kamu suka menulis di pagi hari.",
      version: 1,
      provenance: "unconfirmed_interpretation",
      reviewStatus: "unreviewed",
      safety: "none",
      sourceRefs: [
        {
          recordId: "conv-1",
          version: 3,
          quote: "Aku suka menulis di pagi hari.",
          turnKey: "turn-0",
        },
      ],
      createdAt,
      updatedAt: createdAt,
    };
    const conversation = {
      id: "conv-1",
      kind: "conversation",
      title: "Percakapan sintetis",
      text: "",
      version: 3,
      provenance: "user_original",
      reviewStatus: "confirmed",
      safety: "none",
      createdAt,
      updatedAt: createdAt,
      turns: [],
    };
    const calls = mockApi(
      {
        "GET /memory": () => ({ data: { enabled: true, items: [proposal] } }),
        "GET /memory/prop-1": () => ({ data: proposal }),
        "GET /records/conv-1": () => ({ data: conversation }),
        "PATCH /memory/prop-1/review": () => ({
          data: { ...proposal, reviewStatus: "confirmed" },
        }),
      },
      user,
    );
    renderPath("/notes/memory/prop-1");
    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Kamu suka menulis di pagi hari.",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Aku suka menulis di pagi hari.")).toBeInTheDocument();
    expect(screen.getByText("Tafsiran Sela")).toBeInTheDocument();
    expect(screen.getByText("Belum ditinjau")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Percakapan sintetis" }),
    ).toHaveAttribute("href", "/talk?record=conv-1");
    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));
    await waitFor(() =>
      expect(
        calls.some(
          (c) => c.method === "PATCH" && c.path === "/memory/prop-1/review",
        ),
      ).toBe(true),
    );
    const patch = calls.find(
      (c) => c.method === "PATCH" && c.path === "/memory/prop-1/review",
    );
    expect(patch?.body).toMatchObject({ expectedVersion: 1, action: "confirm" });
    expect(await screen.findByText("Dikonfirmasi")).toBeInTheDocument();
  });
});

describe("consultation follow-ups", () => {
  const summary = {
    id: "sum-1",
    kind: "summary",
    title: "Ringkasan sintetis",
    text: "Kata-kata ringkasan sintetis.",
    version: 1,
    provenance: "manual",
    reviewStatus: "confirmed",
    safety: "none",
    summaryState: "draft",
    sourceRefs: [],
    createdAt: "2026-10-27T09:00:00Z",
    updatedAt: "2026-10-27T10:00:00Z",
  };
  const structured = {
    id: "fu-1",
    kind: "followup",
    title: "Setelah konsultasi",
    text: "Catatan bebas tambahan.",
    version: 1,
    provenance: "user_original",
    reviewStatus: "confirmed",
    safety: "none",
    parentId: "sum-1",
    sections: {
      feeling: "Aku merasa didengar.",
      remember: "Minum obat rutin.",
      agreed: "Coba tidur lebih awal.",
      askNext: "Apakah perlu kontrol lagi?",
    },
    createdAt: "2026-10-28T09:00:00Z",
    updatedAt: "2026-10-28T09:00:00Z",
  };
  const legacy = {
    id: "fu-2",
    kind: "followup",
    title: "Setelah konsultasi",
    text: "Catatan lama tanpa bagian.",
    version: 1,
    provenance: "user_original",
    reviewStatus: "confirmed",
    safety: "none",
    parentId: "sum-1",
    createdAt: "2026-10-29T09:00:00Z",
    updatedAt: "2026-10-29T09:00:00Z",
  };
  const baseHandlers = {
    "GET /records": () => ({ data: { records: [] } }),
    "GET /consultation-summaries/sum-1": () => ({ data: summary }),
    "GET /consultation-summaries/sum-1/followups": () => ({
      data: { followups: [structured, legacy] },
    }),
  };

  it("renders saved sections and the legacy free text, then edits with a version", async () => {
    const calls = mockApi(baseHandlers, profile());
    renderPath("/support/summaries/sum-1");
    expect(
      await screen.findByRole("heading", { level: 2, name: "Setelah percakapan itu" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Siapkan ringkasan berikutnya" }),
    ).toHaveAttribute("href", "/support/summaries/new");
    const heading = screen.getAllByRole("heading", {
      name: /Catatan setelah konsultasi/,
    })[0]!;
    const card = heading.closest("article")!;
    expect(within(card).getByText("Yang aku rasakan")).toBeInTheDocument();
    expect(within(card).getByText("Aku merasa didengar.")).toBeInTheDocument();
    expect(within(card).getByText("Apakah perlu kontrol lagi?")).toBeInTheDocument();
    expect(screen.getByText("Catatan lama tanpa bagian.")).toBeInTheDocument();

    fireEvent.click(within(card).getByRole("button", { name: "Perbaiki" }));
    const feeling = within(card).getByLabelText(/Yang aku rasakan/);
    expect(feeling).toHaveValue("Aku merasa didengar.");
    fireEvent.change(feeling, { target: { value: "Aku merasa lebih tenang." } });
    fireEvent.click(within(card).getByRole("button", { name: "Simpan perbaikan" }));
    await waitFor(() =>
      expect(
        calls.some(
          (c) =>
            c.method === "PATCH" &&
            c.path === "/consultation-summaries/sum-1/followups/fu-1",
        ),
      ).toBe(true),
    );
    const patch = calls.find(
      (c) =>
        c.method === "PATCH" &&
        c.path === "/consultation-summaries/sum-1/followups/fu-1",
    );
    expect(patch?.body).toMatchObject({
      title: "Setelah konsultasi",
      text: "Catatan bebas tambahan.",
      sections: {
        feeling: "Aku merasa lebih tenang.",
        remember: "Minum obat rutin.",
        agreed: "Coba tidur lebih awal.",
        askNext: "Apakah perlu kontrol lagi?",
      },
      storageGranted: true,
      expectedVersion: 1,
    });
  });

  it("keeps the standalone follow-up note read-only and points back to its summary", async () => {
    const calls = mockApi(
      { "GET /records/fu-1": () => ({ data: structured }) },
      profile(),
    );
    renderPath("/notes/fu-1");
    expect(await screen.findByText("Aku merasa didengar.")).toBeInTheDocument();
    expect(screen.getByText("Yang ingin kuingat")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Ubah" })).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Perbaiki di ringkasan" }),
    ).toHaveAttribute("href", "/support/summaries/sum-1");
    expect(calls.some((c) => c.method === "PATCH")).toBe(false);
  });

  it("creates a follow-up from the four sections and the optional free note", async () => {
    const calls = mockApi(
      {
        ...baseHandlers,
        "GET /consultation-summaries/sum-1/followups": () => ({
          data: { followups: [] },
        }),
        "POST /consultation-summaries/sum-1/followups": () => ({
          status: 201,
          data: { ...structured },
        }),
      },
      profile(),
    );
    renderPath("/support/summaries/sum-1");
    const save = await screen.findByRole("button", { name: "Simpan tindak lanjut" });
    expect(save).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/Yang aku rasakan/), {
      target: { value: "Aku merasa lebih tenang." },
    });
    fireEvent.change(screen.getByLabelText(/Yang ingin kuingat/), {
      target: { value: "Minum obat setiap pagi." },
    });
    fireEvent.change(screen.getByLabelText(/Yang disepakati/), {
      target: { value: "Coba tidur sebelum jam sebelas." },
    });
    fireEvent.change(screen.getByLabelText(/Yang ingin kutanyakan lain kali/), {
      target: { value: "Apakah perlu kontrol bulan depan?" },
    });
    fireEvent.change(screen.getByLabelText(/Catatan bebas/), {
      target: { value: "Aku ingin membawa ini ke sesi berikutnya." },
    });
    fireEvent.click(
      screen.getByLabelText("Simpan catatan tindak lanjut ini"),
    );
    fireEvent.click(save);
    await waitFor(() =>
      expect(
        calls.some(
          (c) =>
            c.method === "POST" &&
            c.path === "/consultation-summaries/sum-1/followups",
        ),
      ).toBe(true),
    );
    const create = calls.find(
      (c) =>
        c.method === "POST" &&
        c.path === "/consultation-summaries/sum-1/followups",
    );
    expect(create?.body).toMatchObject({
      title: "Setelah konsultasi",
      text: "Aku ingin membawa ini ke sesi berikutnya.",
      sections: {
        feeling: "Aku merasa lebih tenang.",
        remember: "Minum obat setiap pagi.",
        agreed: "Coba tidur sebelum jam sebelas.",
        askNext: "Apakah perlu kontrol bulan depan?",
      },
      storageGranted: true,
    });
  });
});

describe("consultation summary statements", () => {
  const note = {
    id: "note-items",
    kind: "note",
    title: "Catatan sintetis",
    text: "Teks sumber sintetis untuk bagian.",
    version: 2,
    provenance: "user_original",
    reviewStatus: "confirmed",
    safety: "none",
    createdAt: "2026-10-27T08:00:00Z",
    updatedAt: "2026-10-27T08:30:00Z",
  };
  const summaryWithItems = {
    id: "sum-1",
    kind: "summary",
    title: "Ringkasan dengan bagian",
    text: "Kata-kata bebas.",
    version: 1,
    provenance: "manual",
    reviewStatus: "confirmed",
    safety: "none",
    summaryState: "draft",
    sourceRefs: [{ recordId: note.id, version: note.version }],
    items: [
      {
        id: "item-1",
        text: "Pernyataan pertama.",
        quote: "Kutipan sumber pertama.",
        sourceRef: { recordId: note.id, version: note.version },
        provenance: "user_original",
        reviewStatus: "confirmed",
        position: 0,
      },
    ],
    createdAt: "2026-10-27T09:00:00Z",
    updatedAt: "2026-10-27T09:30:00Z",
  };

  it("renders each statement with its quote and approves the exact version", async () => {
    const calls = mockApi(
      {
        "GET /records": () => ({ data: { records: [note] } }),
        "GET /consultation-summaries/sum-1": () => ({ data: summaryWithItems }),
        "GET /consultation-summaries/sum-1/followups": () => ({ data: { followups: [] } }),
        "POST /consultation-summaries/sum-1/approve": () => ({
          data: { ...summaryWithItems, summaryState: "approved", approvedVersion: 1 },
        }),
      },
      profile(),
    );
    const { container } = renderPath("/support/summaries/sum-1");
    expect(
      await screen.findByRole("heading", { name: "Bagian dari sumbermu" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Pernyataan pertama.")).toBeInTheDocument();
    expect(screen.getByText("Kutipan sumber pertama.")).toBeInTheDocument();
    expect(screen.getByText("Catatan sintetis · v2")).toBeInTheDocument();
    expect(screen.getByText("Kata-kata bebas.")).toBeInTheDocument();
    expect(
      screen.queryByText("Statements from your sources"),
    ).not.toBeInTheDocument();
    const results = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(
      results.violations.filter((v) =>
        ["serious", "critical"].includes(v.impact ?? ""),
      ),
    ).toEqual([]);

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Aku sudah memeriksa versi ini/,
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Setujui versi ini" }));
    await waitFor(() =>
      expect(
        calls.some(
          (c) =>
            c.method === "POST" &&
            c.path === "/consultation-summaries/sum-1/approve",
        ),
      ).toBe(true),
    );
    const approve = calls.find(
      (c) =>
        c.method === "POST" &&
        c.path === "/consultation-summaries/sum-1/approve",
    );
    expect(approve?.body).toEqual({ expectedVersion: 1 });
  });

  it("keeps a legacy text-only summary free of the statement section", async () => {
    mockApi(
      {
        "GET /records": () => ({ data: { records: [note] } }),
        "GET /consultation-summaries/sum-1": () => ({
          data: { ...summaryWithItems, items: undefined },
        }),
        "GET /consultation-summaries/sum-1/followups": () => ({ data: { followups: [] } }),
      },
      profile(),
    );
    renderPath("/support/summaries/sum-1");
    expect(await screen.findByText("Kata-kata bebas.")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Bagian dari sumbermu" }),
    ).not.toBeInTheDocument();
  });

  it("adds, edits, reorders and removes source statements before saving", async () => {
    const calls = mockApi(
      {
        "GET /records": () => ({ data: { records: [note] } }),
        "POST /consultation-summaries": () => ({
          status: 201,
          data: { ...summaryWithItems, id: "sum-new" },
        }),
        "GET /consultation-summaries/sum-new": () => ({
          data: { ...summaryWithItems, id: "sum-new" },
        }),
        "GET /consultation-summaries/sum-new/followups": () => ({
          data: { followups: [] },
        }),
      },
      profile(),
    );
    const { container } = renderPath("/support/summaries/new");
    fireEvent.click(
      await screen.findByRole("checkbox", { name: /Catatan sintetis/ }),
    );
    expect(screen.getByLabelText(/Yang ingin kamu ceritakan/)).toHaveValue(
      note.text,
    );

    const add = screen.getByRole("button", {
      name: "Tambah bagian dari sumber",
    });
    fireEvent.click(add);
    expect(screen.getByText("1 bagian · maksimal 36")).toBeInTheDocument();
    const first = screen.getByLabelText("Kata-katamu untuk bagian 1");
    expect(first).toHaveValue(note.text);
    fireEvent.change(first, { target: { value: "Bagian pertama." } });

    fireEvent.click(add);
    fireEvent.change(
      screen.getByLabelText("Kata-katamu untuk bagian 2"),
      { target: { value: "Bagian kedua." } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Naikkan bagian 2" }));

    fireEvent.click(add);
    fireEvent.change(
      screen.getByLabelText("Kata-katamu untuk bagian 3"),
      { target: { value: "Bagian ketiga." } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Hapus bagian 3" }));
    expect(screen.getByText("2 bagian · maksimal 36")).toBeInTheDocument();
    const results = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(
      results.violations.filter((v) =>
        ["serious", "critical"].includes(v.impact ?? ""),
      ),
    ).toEqual([]);

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Simpan draf ini di server Sela/,
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Simpan draf untuk diperiksa" }),
    );
    await waitFor(() =>
      expect(
        calls.some(
          (c) => c.method === "POST" && c.path === "/consultation-summaries",
        ),
      ).toBe(true),
    );
    const create = calls.find(
      (c) => c.method === "POST" && c.path === "/consultation-summaries",
    );
    const body = create?.body as {
      text: string;
      storageGranted: boolean;
      sourceRefs: Array<{ recordId: string; version: number }>;
      items: Array<Record<string, unknown>>;
    };
    expect(body.text).toBe(note.text);
    expect(body.storageGranted).toBe(true);
    expect(body.sourceRefs).toEqual([
      { recordId: note.id, version: note.version },
    ]);
    expect(body.items).toHaveLength(2);
    expect(body.items[0]).toMatchObject({
      text: "Bagian kedua.",
      quote: note.text,
      sourceRef: { recordId: note.id, version: note.version },
      position: 0,
    });
    expect(body.items[1]).toMatchObject({
      text: "Bagian pertama.",
      quote: note.text,
      sourceRef: { recordId: note.id, version: note.version },
      position: 1,
    });
    expect(body.items[0]).not.toHaveProperty("id");
  });
});

describe("settings", () => {
  it("records an AI consent separately and shows the new state", async () => {
    const user = profile();
    mockApi(
      {
        "GET /me/consents": () => ({ data: { consents } }),
        "GET /me/sessions": () => ({
          data: {
            sessions: [
              {
                id: "s1",
                current: true,
                kind: "guest",
                trustedDevice: false,
                device: { browser: "Firefox", os: "Android" },
                createdAt: "2026-10-27T00:00:00Z",
                lastSeenAt: "2026-10-27T09:00:00Z",
                idleExpiresAt: "2026-10-27T11:00:00Z",
                absoluteExpiresAt: "2026-11-26T00:00:00Z",
              },
            ],
          },
        }),
        "GET /me/audit": () => ({ data: { events: [], nextCursor: null } }),
        "GET /me/consents/history": () => ({ data: { events: [] } }),
        "PUT /me/consents/ai_processing": () => ({
          data: {
            consent: {
              ...consents.find((c) => c.purpose === "ai_processing")!,
              granted: true,
              revision: 1,
            },
            changed: true,
          },
        }),
      },
      user,
    );
    renderPath("/settings/privacy");
    const ai = await screen.findByRole("heading", { name: "Pemrosesan AI" });
    const section = ai.closest("section")!;
    const allow = await within(section).findAllByRole("button", { name: "Izinkan" });
    fireEvent.click(allow[0]!);
    await waitFor(() =>
      expect(within(section).getAllByText(/Diizinkan/).length).toBeGreaterThan(0),
    );
  });

  it("keeps a private-name list and previews masking on the device", async () => {
    const user = profile();
    const calls = mockApi(
      {
        "GET /me/consents": () => ({ data: { consents } }),
        "GET /me/sessions": () => ({ data: { sessions: [] } }),
        "GET /me/audit": () => ({ data: { events: [], nextCursor: null } }),
        "GET /me/consents/history": () => ({ data: { events: [] } }),
        "GET /me/private-names": () => ({
          data: { names: [{ id: "n1", name: "Raka", createdAt: "2026-11-01T00:00:00Z" }] },
        }),
        "POST /me/private-names": (body) => {
          const name = (body as { name: string }).name;
          if (name.toLowerCase() === "raka") return { status: 409, error: "name_taken" };
          return {
            status: 201,
            data: { id: "n2", name, createdAt: "2026-11-01T00:00:00Z" },
          };
        },
        "DELETE /me/private-names/n1": () => ({ data: { deleted: true } }),
      },
      user,
    );
    renderPath("/settings/privacy");
    const heading = await screen.findByRole("heading", { name: "Nama yang disamarkan" });
    const section = heading.closest("section")!;
    expect(await within(section).findByText("Raka")).toBeInTheDocument();
    // The preview is built from the saved name and stays in the browser.
    expect(within(section).getByText("Aku cerita ke Raka kemarin.")).toBeInTheDocument();
    expect(within(section).getByText(/\[NAMA-1\]/)).toBeInTheDocument();

    // A case-insensitive duplicate shows the stable code's message.
    fireEvent.change(within(section).getByLabelText("Tambah nama"), {
      target: { value: "raka" },
    });
    fireEvent.click(within(section).getByRole("button", { name: "Tambah ke daftar" }));
    expect(
      await within(section).findByText("Nama ini sudah ada di daftarmu."),
    ).toBeInTheDocument();

    fireEvent.change(within(section).getByLabelText("Tambah nama"), {
      target: { value: "Dewi" },
    });
    fireEvent.click(within(section).getByRole("button", { name: "Tambah ke daftar" }));
    expect(await within(section).findByText("Dewi")).toBeInTheDocument();
    expect(
      await within(section).findByText("Aku cerita ke Dewi kemarin."),
    ).toBeInTheDocument();

    fireEvent.click(within(section).getByRole("button", { name: "Hapus Raka" }));
    fireEvent.click(await within(section).findByRole("button", { name: "Hapus nama" }));
    await waitFor(() => expect(within(section).queryByText("Raka")).not.toBeInTheDocument());
    expect(
      calls.some((c) => c.method === "DELETE" && c.path === "/me/private-names/n1"),
    ).toBe(true);
    // Only the name list itself was written; the sample text never left.
    expect(
      calls.filter(
        (c) => c.method !== "GET" && !c.path.startsWith("/me/private-names"),
      ),
    ).toEqual([]);
    const posts = calls.filter(
      (c) => c.method === "POST" && c.path === "/me/private-names",
    );
    expect(posts.map((c) => c.body)).toEqual([{ name: "raka" }, { name: "Dewi" }]);
    const results = await axe.run(section, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(
      results.violations.filter((v) => ["serious", "critical"].includes(v.impact ?? "")),
    ).toEqual([]);
  });

  it("applies and persists a text size preference at once", async () => {
    mockApi({}, null);
    renderPath("/settings/display");
    fireEvent.click(await screen.findByRole("radio", { name: "Aa 130%" }));
    expect(document.documentElement.getAttribute("data-text-scale")).toBe("130");
    const stored = window.localStorage.getItem("sela.prefs");
    expect(stored ?? "").toContain('"textScale":"130"');
  });
});

describe("shell menu", () => {
  it("opens as a non-modal panel and closes on Escape or an outside press", async () => {
    mockApi({}, null);
    renderPath("/");
    await screen.findByRole("heading", { level: 1 });
    const button = screen.getByRole("button", { name: "Menu" });
    expect(button).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(button);
    const dialog = document.querySelector(".s-menu-sheet") as HTMLDialogElement;
    await waitFor(() => expect(dialog).toHaveAttribute("open"));
    expect(button).toHaveAttribute("aria-expanded", "true");
    expect(button).toHaveAttribute("aria-controls", "site-menu");
    await waitFor(() => expect(document.activeElement?.id).toBe("menu-help"));

    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(dialog).not.toHaveAttribute("open"));
    expect(button).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(button);
    await waitFor(() => expect(dialog).toHaveAttribute("open"));
    fireEvent.pointerDown(document.body);
    await waitFor(() => expect(dialog).not.toHaveAttribute("open"));
  });
});

describe("accessibility smoke", () => {
  it("has no serious or critical axe findings on the introduction", async () => {
    mockApi({}, null);
    const { container } = renderPath("/");
    await screen.findByRole("heading", { level: 1 });
    const results = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(
      results.violations.filter((v) => ["serious", "critical"].includes(v.impact ?? "")),
    ).toEqual([]);
  });

  it("has no serious or critical axe findings on home", async () => {
    mockApi(
      {
        "GET /records": () => ({ data: { records: [] } }),
        "GET /conversations": () => ({ data: { conversations: [] } }),
      },
      profile(),
    );
    const { container } = renderPath("/home");
    await screen.findByRole("heading", { level: 1, name: "Beranda" });
    const results = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(
      results.violations.filter((v) => ["serious", "critical"].includes(v.impact ?? "")),
    ).toEqual([]);
  });
});

describe("planned care and the sharing space", () => {
  it("shows the condition map stage with its reasons and the demo professionals", async () => {
    const user = profile();
    mockApi(
      {
        "GET /me/consents": () => ({
          data: { consents: [{ purpose: "condition_map", granted: true, outdated: false, revision: 1, currentPolicyVersion: "cm-1" }] },
        }),
        "GET /condition-map": () => ({
          data: {
            stage: "support",
            signals: [{ key: "heavy_streak", count: 3, days: 14 }],
            dataPoints: 6,
            windowDays: 30,
            dataLimited: false,
            sources: { checkin: 6 },
            computedAt: "2026-11-01T10:00:00Z",
          },
        }),
        "GET /care-demo": () => ({
          data: {
            synthetic: true,
            practitioners: [
              { id: "demo-1", name: "Psikolog Klinis Demo A", role: { id: "Psikolog klinis (fiktif)", en: "Clinical psychologist (fictional)" }, focus: { id: "Stres", en: "Stress" }, modes: ["online"], city: "Surabaya", languages: ["id"], stages: ["support"], synthetic: true },
            ],
          },
        }),
        "GET /care-directory": () => ({ data: { entries: [] } }),
      },
      user,
    );
    renderPath("/support/directory");
    expect(await screen.findByText("Butuh dukungan", { selector: "strong" })).toBeInTheDocument();
    expect(screen.getByText("3 cek perasaan terakhir berturut-turut berat")).toBeInTheDocument();
    expect(await screen.findByText("Psikolog Klinis Demo A")).toBeInTheDocument();
    expect(screen.getByText(/Simulasi produk · fiktif/)).toBeInTheDocument();
  });

  it("opens quick-help mode when a community post reads as danger", async () => {
    const user = profile();
    mockApi(
      {
        "GET /me/consents": () => ({
          data: { consents: [{ purpose: "community", granted: true, outdated: false, revision: 1, currentPolicyVersion: "c-1" }] },
        }),
        "GET /community/posts": () => ({ data: { posts: [] } }),
        "POST /community/posts": () => json({ error: { code: "crisis", message: "crisis" } }, 422),
      },
      user,
    );
    renderPath("/support/community");
    const box = await screen.findByLabelText("Ceritamu");
    fireEvent.change(box, { target: { value: "aku mau bunuh diri" } });
    fireEvent.click(screen.getByRole("button", { name: "Bagikan" }));
    expect(await screen.findByRole("alertdialog", { name: "Kamu tidak sendirian" })).toBeInTheDocument();
  });

  it("offers a retry instead of loading forever when the condition map fails", async () => {
    let fail = true;
    mockApi(
      {
        "GET /me/consents": () => ({
          data: { consents: [{ purpose: "condition_map", granted: true, outdated: false, revision: 1, currentPolicyVersion: "cm-1" }] },
        }),
        "GET /condition-map": () =>
          fail
            ? json({ error: { code: "unavailable", message: "unavailable" } }, 503)
            : {
                data: {
                  stage: "everyday",
                  signals: [],
                  dataPoints: 2,
                  windowDays: 30,
                  dataLimited: true,
                  sources: { checkin: 2 },
                  computedAt: "2026-11-01T10:00:00Z",
                },
              },
        "GET /care-demo": () => ({ data: { synthetic: true, practitioners: [] } }),
        "GET /care-directory": () => ({ data: { entries: [] } }),
      },
      profile(),
    );
    renderPath("/support/directory");
    const retry = await screen.findByRole("button", { name: "Coba lagi" });
    expect(screen.queryByText("Membaca catatanmu…")).not.toBeInTheDocument();
    fail = false;
    fireEvent.click(retry);
    expect(await screen.findByText("Sehari-hari", { selector: "strong" })).toBeInTheDocument();
  });

  it("sends one report and one delete however often the buttons are pressed", async () => {
    const user = profile();
    let releaseReport: () => void = () => {};
    let releaseDelete: () => void = () => {};
    const calls = mockApi(
      {
        "GET /me/consents": () => ({
          data: { consents: [{ purpose: "community", granted: true, outdated: false, revision: 1, currentPolicyVersion: "c-1" }] },
        }),
        "GET /community/posts/p1": () => ({
          data: {
            post: { id: "p1", alias: "Bintang", topic: "daily", body: "Hari ini berat.", status: "published", replies: 1, mine: false, createdAt: "2026-11-01T08:00:00Z" },
            replies: [{ id: "r1", alias: "Awan", body: "Semangat ya.", status: "published", mine: true, createdAt: "2026-11-01T09:00:00Z" }],
          },
        }),
        "POST /community/reports": () => new Promise((resolve) => (releaseReport = () => resolve({ data: { ok: true } }))),
        "DELETE /community/replies/r1": () => new Promise((resolve) => (releaseDelete = () => resolve({ data: null }))),
      },
      user,
    );
    renderPath("/support/community/p1");
    expect(await screen.findByText("Hari ini berat.")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Laporkan" }));
    const reason = screen.getByRole("button", { name: "Perundungan" });
    fireEvent.click(reason);
    fireEvent.click(reason);
    fireEvent.click(screen.getByRole("button", { name: "Pelecehan" }));
    expect(reason).toBeDisabled();
    releaseReport();
    await waitFor(() => expect(screen.queryByRole("button", { name: "Perundungan" })).not.toBeInTheDocument());

    const del = screen.getByRole("button", { name: "Hapus balasanku" });
    fireEvent.click(del);
    fireEvent.click(del);
    expect(await screen.findByRole("button", { name: "Menghapus…" })).toBeDisabled();
    releaseDelete();
    await waitFor(() => expect(calls.filter((c) => c.method === "DELETE").length).toBe(1));

    expect(calls.filter((c) => c.method === "POST" && c.path === "/community/reports").length).toBe(1);
    expect(calls.filter((c) => c.method === "DELETE" && c.path === "/community/replies/r1").length).toBe(1);
  });

  it("keeps the sharing space closed to people under 18", async () => {
    mockApi({}, profile({ ageBand: "15_17" }));
    renderPath("/support/community");
    expect(await screen.findByText("Khusus 18 tahun ke atas")).toBeInTheDocument();
  });
});


describe("planned support path (D-076)", () => {
  const summaryEntry = (over: Record<string, unknown> = {}) => ({
    id: "s1",
    kind: "summary",
    title: "Ringkasan untuk klinik",
    text: "Isi ringkasan.",
    version: 2,
    provenance: "user_original",
    reviewStatus: "confirmed",
    safety: "none",
    summaryState: "approved",
    createdAt: "2026-11-01T08:00:00Z",
    updatedAt: "2026-11-01T09:00:00Z",
    ...over,
  });
  const simOrg = {
    id: "org1",
    kind: "clinic",
    name: "Klinik Teduh (simulasi)",
    city: "Bandung",
    modes: ["online", "in_person"],
    stages: ["attention", "support"],
    synthetic: true,
    practitioners: [{ id: "p1", displayName: "Psikolog Demo", professionalType: "psychologist", licenseVerified: true }],
  };
  const request = (over: Record<string, unknown> = {}) => ({
    id: "r1",
    reference: "SR-ABC234",
    orgId: "org1",
    orgName: "Klinik Teduh (simulasi)",
    orgSynthetic: true,
    shareId: "sh1",
    summaryId: "s1",
    summaryVersion: 2,
    mode: "online",
    state: "sent",
    version: 1,
    expiresAt: "2026-11-08T09:00:00Z",
    createdAt: "2026-11-01T09:00:00Z",
    updatedAt: "2026-11-01T09:00:00Z",
    ...over,
  });
  const sharingConsent = (granted: boolean) => ({
    data: { consents: [{ purpose: "sharing", granted, outdated: false, revision: granted ? 1 : 0, currentPolicyVersion: "sh-1" }] },
  });

  it("sends an approved summary to a chosen simulated service and shows it as sent", async () => {
    let requests: unknown[] = [];
    const calls = mockApi(
      {
        "GET /me/consents": () => sharingConsent(true),
        "GET /consultation-summaries": () => ({ data: { summaries: [summaryEntry(), summaryEntry({ id: "s2", title: "Draf lain", summaryState: "draft" })] } }),
        "GET /care-network": () => ({ data: { orgs: [simOrg], realPartners: false } }),
        "GET /care-requests": () => ({ data: { requests } }),
        "POST /care-requests": () => {
          requests = [request()];
          return { status: 201, data: { request: request() } };
        },
      },
      profile(),
    );
    renderPath("/support/plan?summary=s1");
    expect(await screen.findByRole("heading", { name: "Bantuan terencana", level: 1 })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Ringkasan untuk klinik/ })).toBeChecked();
    expect(screen.getAllByText("Simulasi").length).toBeGreaterThan(0);
    expect(screen.getByText(/simulasi yang dijalankan tim Sela/)).toBeInTheDocument();
    expect(screen.getByText("Lisensi terverifikasi")).toBeInTheDocument();
    expect(screen.getByText(/tidak membagikan nomor telepon atau emailmu/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Pilih Klinik Teduh (simulasi)" }));
    fireEvent.click(screen.getByRole("radio", { name: "Daring" }));
    const send = screen.getByRole("button", { name: "Kirim permintaan" });
    expect(send).toBeDisabled();
    fireEvent.click(screen.getByLabelText(/Aku setuju mengirim versi 2 ringkasan ini ke Klinik Teduh/));
    fireEvent.click(send);

    expect(await screen.findByText("Permintaan terkirim")).toBeInTheDocument();
    expect(screen.getAllByText("SR-ABC234").length).toBeGreaterThan(0);
    expect(await screen.findByText("Terkirim")).toBeInTheDocument();
    const posts = calls.filter((c) => c.method === "POST" && c.path === "/care-requests");
    expect(posts).toHaveLength(1);
    expect(posts[0]!.body).toEqual({ summaryId: "s1", orgId: "org1", practitionerId: "", mode: "online" });
  });

  it("confirms one proposed time once however often it is pressed", async () => {
    const slot1 = "2026-11-04T09:00:00Z";
    const slot2 = "2026-11-05T09:00:00Z";
    let release: () => void = () => {};
    const calls = mockApi(
      {
        "GET /me/consents": () => sharingConsent(true),
        "GET /consultation-summaries": () => ({ data: { summaries: [summaryEntry()] } }),
        "GET /care-network": () => ({ data: { orgs: [simOrg] } }),
        "GET /care-requests": () => ({
          data: {
            requests: [
              request({
                state: "proposed",
                version: 3,
                proposal: { slots: [slot1, slot2], mode: "online", contact: "WhatsApp klinik simulasi 0800-SIM", note: "Sesi 45 menit." },
              }),
            ],
          },
        }),
        "POST /care-requests/r1/confirm": () => new Promise((resolve) => (release = () => resolve({ data: { request: request({ state: "confirmed" }) } }))),
      },
      profile(),
    );
    renderPath("/support/plan");
    expect(await screen.findByText("Ada usulan waktu")).toBeInTheDocument();
    expect(screen.getByText(/WhatsApp klinik simulasi 0800-SIM/)).toBeInTheDocument();
    const list = screen.getByText("Usulan waktu").closest("li")!;
    const choose = within(list as HTMLElement).getAllByRole("button", { name: /^Pilih / });
    expect(choose).toHaveLength(2);
    fireEvent.click(choose[0]!);
    fireEvent.click(choose[0]!);
    fireEvent.click(choose[1]!);
    release();
    await waitFor(() => expect(calls.filter((c) => c.path === "/care-requests/r1/confirm")).toHaveLength(1));
    expect(calls.find((c) => c.path === "/care-requests/r1/confirm")!.body).toEqual({ slot: slot1, expectedVersion: 3 });
  });

  it("turns on sharing first, and shows the error instead of sending when that fails", async () => {
    let granted = false;
    let failPut = true;
    const calls = mockApi(
      {
        "GET /me/consents": () => sharingConsent(granted),
        "PUT /me/consents/sharing": () => {
          if (failPut) return json({ error: { code: "internal", message: "internal" } }, 500);
          granted = true;
          return { data: { ok: true } };
        },
        "GET /consultation-summaries": () => ({ data: { summaries: [summaryEntry()] } }),
        "GET /care-network": () => ({ data: { orgs: [simOrg] } }),
        "GET /care-requests": () => ({ data: { requests: [] } }),
        "POST /care-requests": () => ({ status: 201, data: { request: request() } }),
      },
      profile(),
    );
    renderPath("/support/plan?summary=s1");
    fireEvent.click(await screen.findByRole("button", { name: "Pilih Klinik Teduh (simulasi)" }));
    fireEvent.click(screen.getByRole("radio", { name: "Mana saja" }));
    expect(screen.getByText(/Ini juga mengaktifkan izin berbagi ringkasan/)).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText(/Aku setuju mengirim versi 2/));
    fireEvent.click(screen.getByRole("button", { name: "Kirim permintaan" }));
    await waitFor(() => expect(calls.some((c) => c.method === "PUT" && c.path === "/me/consents/sharing")).toBe(true));
    expect((await screen.findAllByRole("alert")).length).toBeGreaterThan(0);
    expect(calls.some((c) => c.method === "POST" && c.path === "/care-requests")).toBe(false);

    failPut = false;
    fireEvent.click(screen.getByRole("button", { name: "Kirim permintaan" }));
    await waitFor(() => expect(calls.some((c) => c.method === "POST" && c.path === "/care-requests")).toBe(true));
    const put = calls.filter((c) => c.method === "PUT" && c.path === "/me/consents/sharing").at(-1)!;
    expect(put.body).toEqual({ granted: true, policyVersion: "sh-1" });
    const post = calls.find((c) => c.method === "POST" && c.path === "/care-requests")!;
    expect(post.body).toEqual({ summaryId: "s1", orgId: "org1", practitionerId: "", mode: "either" });
  });

  it("cancels a request after confirming", async () => {
    let state = "sent";
    const calls = mockApi(
      {
        "GET /me/consents": () => sharingConsent(true),
        "GET /consultation-summaries": () => ({ data: { summaries: [] } }),
        "GET /care-network": () => ({ data: { orgs: [] } }),
        "GET /care-requests": () => ({ data: { requests: [request({ state })] } }),
        "POST /care-requests/r1/cancel": () => {
          state = "cancelled";
          return { data: { request: request({ state }) } };
        },
      },
      profile(),
    );
    renderPath("/support/plan");
    expect(await screen.findByText("Terkirim")).toBeInTheDocument();
    expect(screen.getByText(/Belum ada ringkasan yang kamu setujui/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Batalkan permintaan" }));
    fireEvent.click(screen.getByRole("button", { name: "Batalkan" }));
    expect(await screen.findByText("Dibatalkan")).toBeInTheDocument();
    expect(calls.find((c) => c.path === "/care-requests/r1/cancel")!.body).toEqual({ expectedVersion: 1 });
    expect(screen.queryByRole("button", { name: "Batalkan permintaan" })).not.toBeInTheDocument();
  });
});
