import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import axe from "axe-core";
import { I18nProvider } from "../i18n";
import { RouterProvider } from "../lib/router";
import { AnnounceProvider } from "../v3/shared";
import type { Entry, Profile } from "../v3/api";
import { ActionStepsPage, type ActionStep } from "../v3/screens/ActionSteps";

// Satu langkah, pilihanku (BW-3). All data is synthetic; the API is mocked.

const user: Profile = {
  id: "user-sintetis",
  kind: "guest",
  username: null,
  locale: "id",
  ageBand: "18_plus",
  version: 1,
  createdAt: "2026-11-01T00:00:00Z",
  lastActiveAt: "2026-11-02T00:00:00Z",
  guest: null,
  recovery: null,
  guardianConsent: null,
  actionsRequired: [],
  environment: "test",
  synthetic: true,
};

const note: Entry = {
  id: "note-1",
  kind: "note",
  title: "Konsultasi sintetis",
  text: "Aku sering lupa apa yang mau kutanyakan.",
  version: 2,
  provenance: "user_original",
  reviewStatus: "confirmed",
  safety: "none",
  createdAt: "2026-11-01T00:00:00Z",
  updatedAt: "2026-11-02T00:00:00Z",
};

function step(over: Partial<ActionStep> = {}): ActionStep {
  return {
    id: "step-1",
    version: 1,
    text: "Tuliskan dua pertanyaan sintetis.",
    status: "chosen",
    sourceRefs: [],
    origin: { kind: "user_written" },
    review: { userApproved: true, stale: false },
    scope: "private",
    safety: "none",
    createdAt: "2026-11-02T00:00:00Z",
    updatedAt: "2026-11-02T00:00:00Z",
    ...over,
  };
}

type Call = { method: string; path: string; body: unknown };

function mockApi(opts: {
  steps?: ActionStep[];
  aiGranted?: boolean;
  suggest?: unknown;
  onPost?: (body: Record<string, unknown>) => ActionStep;
  onPatch?: (body: Record<string, unknown>) => ActionStep;
}) {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input).replace(/^.*\/api\/v3/, "");
      const method = init?.method ?? "GET";
      const body = init?.body ? JSON.parse(String(init.body)) : undefined;
      calls.push({ method, path, body });
      let data: unknown = {};
      let status = 200;
      if (path === "/action-steps" && method === "GET") data = { actionSteps: opts.steps ?? [] };
      else if (path === "/records") data = { records: [note] };
      else if (path === "/me/consents")
        data = {
          consents: [
            { purpose: "ai_processing", granted: Boolean(opts.aiGranted), outdated: false, revision: 1, currentPolicyVersion: "ai-v1" },
          ],
        };
      else if (path === "/action-steps/suggest") data = opts.suggest;
      else if (path === "/action-steps" && method === "POST") {
        status = 201;
        data = opts.onPost ? opts.onPost(body) : step({ text: body.text });
      } else if (path.startsWith("/action-steps/") && method === "PATCH") data = opts.onPatch!(body);
      else if (path.startsWith("/action-steps/") && method === "DELETE") data = { deleted: true };
      return new Response(JSON.stringify({ data }), { status, headers: { "Content-Type": "application/json" } });
    }),
  );
  return calls;
}

function renderPage(lang: "id" | "en" = "id") {
  return render(
    <I18nProvider initial={lang}>
      <RouterProvider>
        <AnnounceProvider>
          <ActionStepsPage user={user} />
        </AnnounceProvider>
      </RouterProvider>
    </I18nProvider>,
  );
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const sent = (calls: Call[], method: string, path: string) => calls.filter((c) => c.method === method && c.path === path);

it("saves a manual step with AI consent off and never asks for a suggestion", async () => {
  const calls = mockApi({ aiGranted: false });
  renderPage();
  expect(await screen.findByText("Belum ada langkah")).toBeInTheDocument();
  expect(screen.getByText(/Hanya kamu yang memilih apa yang dicoba; tidak ada nilai\./)).toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "Tulis sendiri" }));
  fireEvent.click(screen.getByRole("button", { name: "Simpan langkah" }));
  expect(await screen.findByText("Tulis langkahnya dengan kata-katamu sendiri.")).toBeInTheDocument();
  expect(sent(calls, "POST", "/action-steps")).toHaveLength(0);

  fireEvent.change(screen.getByLabelText(/Dengan kata-katamu sendiri/), { target: { value: "Tuliskan dua pertanyaan sintetis." } });
  fireEvent.click(screen.getByRole("button", { name: "Simpan langkah" }));
  expect(await screen.findByText("Tuliskan dua pertanyaan sintetis.", { selector: "p" })).toBeInTheDocument();
  const [post] = sent(calls, "POST", "/action-steps");
  expect(post!.body).toEqual({ text: "Tuliskan dua pertanyaan sintetis.", dueDate: "", storageGranted: true, sourceRefs: [] });
  expect(screen.getByText("Ditulis olehmu")).toBeInTheDocument();
  expect(screen.getByText("Kupilih")).toBeInTheDocument();

  // "See a suggestion" with consent off explains and offers the manual path.
  fireEvent.click(screen.getByRole("button", { name: "Lihat usulan" }));
  expect(await screen.findByText("Usulan memerlukan izin pemrosesan AI")).toBeInTheDocument();
  expect(sent(calls, "POST", "/action-steps/suggest")).toHaveLength(0);
});

it("skipping stores nothing", async () => {
  const calls = mockApi({});
  renderPage();
  fireEvent.click(await screen.findByRole("button", { name: "Lewati" }));
  expect(screen.getAllByText("Tidak apa-apa. Tidak ada yang disimpan.").length).toBeGreaterThan(0);
  expect(calls.filter((c) => c.method !== "GET")).toHaveLength(0);
});

const proposal = {
  aiState: "released",
  help: false,
  proposal: {
    text: "Mau menuliskan satu pertanyaan yang paling ingin kamu bawa?",
    token: "tok.sig",
    sourceRef: { recordId: "note-1", version: 2 },
    origin: { kind: "meta_proposal", model: "test-generator", draft: "test_double" },
    tentative: true,
    expiresInSeconds: 1800,
  },
};

it("shows a tentative suggestion, saves nothing until the owner chooses, and keeps the owner's rewrite", async () => {
  const calls = mockApi({
    aiGranted: true,
    suggest: proposal,
    onPost: (b) => step({ text: String(b.text), origin: { kind: "meta_proposal", edited: true, model: "test-generator" } }),
  });
  renderPage();
  fireEvent.click(await screen.findByRole("button", { name: "Lihat usulan" }));
  const select = await screen.findByLabelText("Catatan yang jadi dasar");
  fireEvent.change(select, { target: { value: "note-1" } });
  fireEvent.click(screen.getByRole("button", { name: "Minta satu usulan" }));
  expect(await screen.findByText(proposal.proposal.text)).toBeInTheDocument();
  expect(screen.getByText(/belum disimpan/)).toBeInTheDocument();
  expect(sent(calls, "POST", "/action-steps/suggest")[0]!.body).toEqual({ sourceRef: { recordId: "note-1", version: 2 } });
  expect(sent(calls, "POST", "/action-steps")).toHaveLength(0);

  fireEvent.click(screen.getByRole("button", { name: "Pakai atau ubah" }));
  const box = screen.getByLabelText(/Dengan kata-katamu sendiri/) as HTMLTextAreaElement;
  expect(box.value).toBe(proposal.proposal.text);
  fireEvent.change(box, { target: { value: "Tuliskan dua pertanyaan yang ingin kutanyakan." } });
  fireEvent.click(screen.getByRole("button", { name: "Simpan langkah" }));
  expect(await screen.findByText("Tuliskan dua pertanyaan yang ingin kutanyakan.", { selector: "p" })).toBeInTheDocument();
  const [post] = sent(calls, "POST", "/action-steps");
  expect(post!.body).toEqual({
    text: "Tuliskan dua pertanyaan yang ingin kutanyakan.",
    dueDate: "",
    storageGranted: true,
    sourceRefs: [],
    proposal: { token: "tok.sig", text: proposal.proposal.text },
  });
  expect(screen.getByText("Usulan Meta yang kamu ubah lalu pilih")).toBeInTheDocument();
});

it("rejecting a suggestion stores nothing", async () => {
  const calls = mockApi({ aiGranted: true, suggest: proposal });
  renderPage();
  fireEvent.click(await screen.findByRole("button", { name: "Lihat usulan" }));
  fireEvent.change(await screen.findByLabelText("Catatan yang jadi dasar"), { target: { value: "note-1" } });
  fireEvent.click(screen.getByRole("button", { name: "Minta satu usulan" }));
  fireEvent.click(await screen.findByRole("button", { name: "Tolak" }));
  expect(screen.queryByText(proposal.proposal.text)).not.toBeInTheDocument();
  expect(sent(calls, "POST", "/action-steps")).toHaveLength(0);
  expect(calls.filter((c) => c.method === "PATCH" || c.method === "DELETE")).toHaveLength(0);
});

it("crisis text gets help, never a step suggestion", async () => {
  mockApi({ aiGranted: true, suggest: { aiState: "crisis", proposal: null, help: true } });
  renderPage();
  fireEvent.click(await screen.findByRole("button", { name: "Lihat usulan" }));
  fireEvent.change(await screen.findByLabelText("Catatan yang jadi dasar"), { target: { value: "note-1" } });
  fireEvent.click(screen.getByRole("button", { name: "Minta satu usulan" }));
  expect(await screen.findByRole("link", { name: /Bantuan sekarang/ })).toHaveAttribute("href", "/help");
  expect(screen.queryByRole("button", { name: "Pakai atau ubah" })).not.toBeInTheDocument();
});

it("status buttons map to the lifecycle and send the expected version", async () => {
  let current = step();
  const calls = mockApi({
    steps: [current],
    onPatch: (b) => {
      current = step({ version: current.version + 1, status: b.status as ActionStep["status"] });
      return current;
    },
  });
  renderPage();
  const card = (await screen.findByText("Tuliskan dua pertanyaan sintetis.")).closest("li")!;
  fireEvent.click(within(card).getByRole("button", { name: "Sudah kucoba" }));
  await waitFor(() => expect(within(card).getByText("Sudah kucoba", { selector: ".a-step-status" })).toBeInTheDocument());
  expect(sent(calls, "PATCH", "/action-steps/step-1")[0]!.body).toEqual({ expectedVersion: 1, status: "tried" });
  fireEvent.click(within(card).getByRole("button", { name: "Buka lagi" }));
  await waitFor(() => expect(within(card).getByRole("button", { name: "Nanti dulu" })).toBeInTheDocument());
  fireEvent.click(within(card).getByRole("button", { name: "Nanti dulu" }));
  await waitFor(() => expect(within(card).getByRole("button", { name: "Lanjutkan" })).toBeInTheDocument());
  fireEvent.click(within(card).getByRole("button", { name: "Tidak cocok" }));
  await waitFor(() => expect(within(card).getByRole("button", { name: "Pakai lagi" })).toBeInTheDocument());
  expect(sent(calls, "PATCH", "/action-steps/step-1").map((c) => c.body)).toEqual([
    { expectedVersion: 1, status: "tried" },
    { expectedVersion: 2, status: "chosen" },
    { expectedVersion: 3, status: "paused" },
    { expectedVersion: 4, status: "discarded" },
  ]);
});

it("a step whose source changed asks for review: reaffirm, unlink or discard", async () => {
  const review = step({
    status: "needs_review",
    sourceRefs: [{ recordId: "note-1", version: 1, kind: "note", current: false }],
    review: { userApproved: false, stale: true, staleReason: "source_edited", staleAt: "2026-11-02T01:00:00Z" },
  });
  const calls = mockApi({
    steps: [review],
    onPatch: () => step({ version: 2, sourceRefs: [] }),
  });
  renderPage();
  expect(await screen.findByText("Catatan sumbernya berubah")).toBeInTheDocument();
  expect(screen.getByText("Perlu ditinjau")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Tetap pakai dengan versi baru" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Sudah kucoba" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Lepas dari catatan" }));
  await waitFor(() => expect(screen.queryByText("Catatan sumbernya berubah")).not.toBeInTheDocument());
  expect(sent(calls, "PATCH", "/action-steps/step-1")[0]!.body).toEqual({ expectedVersion: 1, sourceRefs: [] });
  expect(screen.getByText("Tuliskan dua pertanyaan sintetis.")).toBeInTheDocument();
});

it("a deleted source cannot be reaffirmed", async () => {
  mockApi({
    steps: [
      step({
        status: "needs_review",
        sourceRefs: [{ recordId: "note-gone", version: 1, current: false }],
        review: { userApproved: false, stale: true, staleReason: "source_deleted" },
      }),
    ],
  });
  renderPage();
  expect(await screen.findByText(/sudah dihapus\. Kata-katamu tetap di sini/)).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Tetap pakai dengan versi baru" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Lepas dari catatan" })).toBeInTheDocument();
});

it("never uses streak, score or success wording, in either language", async () => {
  for (const lang of ["id", "en"] as const) {
    mockApi({
      steps: [step(), step({ id: "s2", status: "tried" }), step({ id: "s3", status: "paused", dueDate: "2026-11-09" })],
      aiGranted: true,
      suggest: proposal,
    });
    const { container, unmount } = renderPage(lang);
    await screen.findAllByText("Tuliskan dua pertanyaan sintetis.");
    const body = container.textContent!.toLowerCase();
    for (const word of ["streak", "skor", "poin", "lencana", "berhasil", "runtun", "score", "points", "badge", "success", "you missed", "terlewat"]) {
      expect(body).not.toContain(word);
    }
    if (lang === "en") expect(body).toContain("only you choose what to try; there are no grades.");
    unmount();
    vi.unstubAllGlobals();
  }
});

it("the create and review flows have no serious accessibility violations", async () => {
  mockApi({
    steps: [
      step(),
      step({
        id: "s2",
        status: "needs_review",
        sourceRefs: [{ recordId: "note-1", version: 1, kind: "note", current: false }],
        review: { userApproved: false, stale: true, staleReason: "source_edited" },
      }),
    ],
  });
  const { container } = renderPage();
  fireEvent.click(await screen.findByRole("button", { name: "Tulis sendiri" }));
  await screen.findByText("Catatan sumbernya berubah");
  const results = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
  expect(results.violations.filter((v) => ["serious", "critical"].includes(v.impact ?? ""))).toEqual([]);
});
