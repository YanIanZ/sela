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
  ai_provider_anthropic: {
    version: "anth-v1",
    title: { id: "Pemeriksa · Claude", en: "Checker · Claude" },
    body: [text("Pemeriksa memeriksa balasan yang menyebut hal kesehatan.")],
  },
  ai_provider_deepseek: {
    version: "ds-v1",
    title: { id: "Model cadangan · DeepSeek", en: "Backup model · DeepSeek" },
    body: [text("Hanya dipakai jika model utama tidak tersedia.")],
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
  {
    purpose: "ai_provider_anthropic",
    required: false,
    granted: false,
    outdated: false,
    revision: 0,
    currentPolicyVersion: "anth-v1",
  },
  {
    purpose: "ai_provider_deepseek",
    required: false,
    granted: false,
    outdated: false,
    revision: 0,
    currentPolicyVersion: "ds-v1",
  },
];

type Reply = { status?: number; data?: unknown; error?: string } | "unauth";
type Handler = (body: unknown, url: URL) => Reply | Promise<Reply>;

function json(payload: unknown, status = 200) {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function mockApi(handlers: Record<string, Handler>, signedIn: Profile | null) {
  const calls: Array<{ method: string; path: string; body: unknown }> = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
      const url = new URL(String(input), "http://localhost");
      const method = (init.method ?? "GET").toUpperCase();
      const path = url.pathname.replace(/^\/api\/v3/, "");
      const body = typeof init.body === "string" ? JSON.parse(init.body) : undefined;
      calls.push({ method, path, body });
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
            verification: { provenance: "live_anthropic", status: "released" },
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
      screen.getByText(/Sela · balasan AI \(Meta\) · diperiksa oleh Claude/),
    ).toBeInTheDocument();
    const send = calls.find((c) => c.path === "/conversations/room-1/turns");
    expect(send?.body).toMatchObject({
      text: "Aku merasa berat hari ini.",
      expectedVersion: 1,
    });
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
