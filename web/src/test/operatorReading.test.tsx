import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import axe from "axe-core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "../i18n";
import { OperatorApp } from "../operator/OperatorApp";
import type {
  Admin,
  ReadingA11yCheck,
  ReadingA11yMeta,
  ReadingA11yReport,
  ReadingAdminItem,
  ReadingApproval,
  ReadingDetail,
} from "../operator/api";

const owner: Admin = {
  username: "reading.owner",
  role: "owner",
  orgId: "",
  permissions: ["ops/read", "research/review", "safety-content/review", "staff/manage"],
};

const detail: ReadingDetail = {
  id: "draft-one",
  version: 2,
  kind: "exercise",
  topics: ["stress"],
  audience: "adult",
  title: { id: "Judul Latihan", en: "Exercise title" },
  summary: { id: "Ringkasan Indonesia.", en: "English summary." },
  steps: { id: ["Langkah satu"], en: ["Step one"] },
  why: { id: "Alasan Indonesia.", en: "English reason." },
  seekHelpWhen: { id: "Cari bantuan.", en: "Seek help." },
  cautions: { id: "Perhatian.", en: "Caution." },
  readingTimeMinutes: { id: 2, en: 2 },
  source: {
    relation: "sela_original",
    locator: "Whole item",
    sources: [
      {
        id: "src-1",
        title: "Source one",
        publisher: "Publisher",
        url: "https://example.org/source",
        licence: "MIT",
      },
    ],
  },
  licence: {
    itemTextLicence: "MIT",
    sourceLicence: "MIT",
    commercialUse: "allowed",
  },
  attribution: { id: "Sela", en: "Sela" },
  adaptationNote: "Adapted for Sela.",
  evidenceNote: "Evidence note.",
  review: {
    status: "machine_drafted_unreviewed",
    reviewedOn: "",
    verifier: "not_run",
  },
};

const hash = "a".repeat(64);

const CHECK_IDS = [
  "language_parity",
  "heading_structure",
  "list_structure",
  "link_text",
  "image_alt",
  "sensory_instructions",
  "sentence_length",
  "readability",
  "block_length",
];

type A11yState = "pass" | "needs_override" | "blocked";

function a11yReport(state: A11yState): ReadingA11yReport {
  const checks: ReadingA11yCheck[] = CHECK_IDS.map((id) => ({
    id,
    status: id === "image_alt" ? "not_applicable" : "pass",
    blocking: !["sentence_length", "readability", "block_length"].includes(id),
    findings: [],
  }));
  if (state === "needs_override") {
    const c = checks.find((x) => x.id === "sentence_length")!;
    c.status = "fail";
    c.findings = [
      { code: "sentence_over_20_words", path: "content.seek_help_when.en", lang: "en", value: 22 },
    ];
  }
  if (state === "blocked") {
    const c = checks.find((x) => x.id === "sensory_instructions")!;
    c.status = "fail";
    c.findings = [
      { code: "sensory_only_instruction", path: "content.steps.en[0]", lang: "en" },
    ];
  }
  return {
    version: "sela-reading-a11y-v1",
    result: state,
    checks,
    metrics: {
      id: { sentences: 6, words: 60, avgWordsPerSentence: 10, maxWordsPerSentence: 14, sentencesOver20Words: 0, fkGrade: null },
      en: { sentences: 6, words: 66, avgWordsPerSentence: 11, maxWordsPerSentence: 22, sentencesOver20Words: state === "needs_override" ? 1 : 0, fkGrade: 5.2 },
    },
  };
}

const meta: ReadingA11yMeta = {
  checksVersion: "sela-reading-a11y-v1",
  manualVersion: "sela-reading-a11y-manual-v1",
  manualChecklist: [
    { id: "read_aloud_plain_language", label: { id: "Saya membaca dengan suara keras.", en: "I read it aloud." } },
    { id: "meaning_without_visuals", label: { id: "Makna jelas tanpa warna.", en: "Meaning is clear without colour." } },
    { id: "names_and_order", label: { id: "Judul dan urutan jelas.", en: "Headings and order are clear." } },
  ],
  overrideReason: { min: 20, max: 300 },
};

const allManual = {
  read_aloud_plain_language: true,
  meaning_without_visuals: true,
  names_and_order: true,
};

function approval(revoked = false): ReadingApproval {
  return {
    slug: "draft-one",
    reviewerName: "Editor Manusia",
    reviewedOn: "2026-11-01",
    note: "read the current text",
    verifierStatus: "supported",
    contentHash: hash,
    approvedBy: "11111111-1111-4111-8111-111111111111",
    approvedAt: "2026-11-02T09:00:00Z",
    revokedAt: revoked ? "2026-11-03T09:00:00Z" : undefined,
    version: revoked ? 2 : 1,
  };
}

function item(
  state: "draft" | "approved" | "revoked",
  a11y: A11yState = "pass",
): ReadingAdminItem {
  const ap = state === "draft" ? undefined : approval(state === "revoked");
  return {
    slug: "draft-one",
    detail,
    contentHash: hash,
    accessibility: a11yReport(a11y),
    embeddedReview: {
      status: "machine_drafted_unreviewed",
      verifierStatus: "not_run",
      verifierNote: "Verifier has not run.",
      publishable: false,
    },
    approval: ap,
    hashMatches: Boolean(ap),
    published: state === "approved",
  };
}

function reply(status: number, body: unknown): Promise<Response> {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as unknown as Response);
}

type Call = { url: string; init: RequestInit };

function stubReading(
  session: Admin,
  start: "draft" | "approved" = "draft",
  a11y: A11yState = "pass",
  approveError?: { status: number; code: string; field: string },
): { calls: Call[]; state: { value: "draft" | "approved" | "revoked" } } {
  const calls: Call[] = [];
  const state = { value: start as "draft" | "approved" | "revoked" };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
      const url = String(input);
      calls.push({ url, init });
      if (url.endsWith("/api/admin/v1/me")) {
        return reply(200, { data: { admin: session } });
      }
      if (url.endsWith("/api/admin/v1/reading/audit")) {
        return reply(200, { data: { events: [] } });
      }
      if (url.endsWith("/api/admin/v1/reading/draft-one/approve")) {
        if (approveError) {
          return reply(approveError.status, {
            error: { code: approveError.code, message: "x", field: approveError.field },
          });
        }
        state.value = "approved";
        return reply(200, {
          data: { slug: "draft-one", approval: approval() },
        });
      }
      if (url.endsWith("/api/admin/v1/reading/draft-one/revoke")) {
        state.value = "revoked";
        return reply(200, {
          data: { slug: "draft-one", approval: approval(true) },
        });
      }
      if (url.endsWith("/api/admin/v1/reading")) {
        return reply(200, {
          data: { items: [item(state.value, a11y)], accessibility: meta },
        });
      }
      throw new Error(`unexpected request ${url}`);
    }),
  );
  return { calls, state };
}

function renderReading() {
  window.history.replaceState(null, "", "/operator/reading");
  return render(
    <I18nProvider initial="en">
      <OperatorApp />
    </I18nProvider>,
  );
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
  window.localStorage.clear();
});

async function openItem() {
  await screen.findByRole("heading", { name: "Reading reviews" });
  const row = await screen.findByRole("button", { name: /Exercise title/ });
  fireEvent.click(row);
  await screen.findByRole("heading", { name: /Exercise title/ });
}

describe("operator reading review", () => {
  it("shows the full bilingual text and approves only with an explicit confirmation", async () => {
    const { calls } = stubReading(owner);
    const { container } = renderReading();
    await openItem();

    expect(screen.getByText("Source one")).toBeInTheDocument();
    expect(screen.getAllByText("MIT").length).toBeGreaterThan(0);
    expect(screen.getByText("English summary.")).toBeInTheDocument();
    expect(screen.getByText("Ringkasan Indonesia.")).toBeInTheDocument();
    expect(screen.getByText("Step one")).toBeInTheDocument();
    expect(screen.getByText("English reason.")).toBeInTheDocument();
    expect(screen.getAllByText("Machine draft, unreviewed").length).toBeGreaterThan(0);
    expect(screen.getByText(/Verifier has not run/)).toBeInTheDocument();
    expect(screen.getByLabelText("Reviewer name")).toHaveValue("");

    const submit = screen.getByRole("button", {
      name: "Approve for publication",
    });
    expect(submit).toHaveAttribute("aria-disabled", "true");
    fireEvent.change(screen.getByLabelText("Reviewer name"), {
      target: { value: "Editor Manusia" },
    });
    fireEvent.change(screen.getByLabelText("Review date"), {
      target: { value: "2026-11-01" },
    });
    fireEvent.change(screen.getByLabelText("Note (optional)"), {
      target: { value: "read it" },
    });
    expect(submit).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(
      screen.getByRole("checkbox", { name: /I have read this exact text/ }),
    );
    // The manual accessibility checklist is still open.
    expect(submit).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByText(/Still needed: accessibility checklist \(0 of 3\)/)).toBeInTheDocument();
    const checklist = screen.getByRole("group", {
      name: "Manual accessibility checklist (required)",
    });
    for (const box of within(checklist).getAllByRole("checkbox")) {
      fireEvent.click(box);
    }
    expect(submit).not.toHaveAttribute("aria-disabled");
    fireEvent.click(submit);

    await screen.findByText(/draft-one is approved/);
    const call = calls.find((c) =>
      c.url.endsWith("/api/admin/v1/reading/draft-one/approve"),
    );
    expect(call?.init.method).toBe("POST");
    expect(JSON.parse(String(call?.init.body))).toEqual({
      reviewerName: "Editor Manusia",
      reviewedOn: "2026-11-01",
      note: "read it",
      verifierStatus: "supported",
      contentHash: hash,
      confirm: true,
      accessibility: {
        checksVersion: "sela-reading-a11y-v1",
        manualVersion: "sela-reading-a11y-manual-v1",
        manualChecks: allManual,
        overrideReason: "",
      },
    });
    expect(new Headers(call?.init.headers).get("X-Sela-Request")).toBe("1");
    await waitFor(() => expect(screen.getAllByText("Published").length).toBeGreaterThan(0));

    const results = await axe.run(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(
      results.violations.filter((v) =>
        ["serious", "critical"].includes(v.impact ?? ""),
      ),
    ).toEqual([]);
  });

  it("ends publication after an inline confirmation and sends the reason", async () => {
    const { calls, state } = stubReading(owner, "approved");
    renderReading();
    await openItem();
    expect(screen.getByText("Current for this text")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "End publication" }));
    expect(
      await screen.findByText(
        "End publication of this item? The public library stops serving it immediately.",
      ),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Reason (optional)"), {
      target: { value: "withdrawn" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Yes, end publication" }));

    await screen.findByText(/is no longer publicly served/);
    expect(state.value).toBe("revoked");
    const call = calls.find((c) =>
      c.url.endsWith("/api/admin/v1/reading/draft-one/revoke"),
    );
    expect(call?.init.method).toBe("POST");
    expect(JSON.parse(String(call?.init.body))).toEqual({ reason: "withdrawn" });
    await waitFor(() => expect(screen.getAllByText("Revoked").length).toBeGreaterThan(0));
  });

  it("marks the nav entry and sends a role without a review permission to Home", async () => {
    stubReading(owner);
    renderReading();
    await screen.findByRole("heading", { name: "Reading reviews" });
    expect(
      screen.getByRole("link", { name: "Reading reviews" }),
    ).toHaveAttribute("aria-current", "page");

    cleanup();
    vi.unstubAllGlobals();
    const ops: Admin = {
      username: "ops.one",
      role: "ops",
      orgId: "",
      permissions: ["ops/read"],
    };
    const { calls } = stubReading(ops);
    renderReading();
    // A page the role may not use falls back to Home (operator shell).
    await screen.findByRole("heading", { name: "Home" });
    await waitFor(() => expect(window.location.pathname).toBe("/operator"));
    expect(
      screen.queryByRole("link", { name: "Reading reviews" }),
    ).not.toBeInTheDocument();
    expect(calls.some((c) => c.url.endsWith("/api/admin/v1/reading"))).toBe(false);
  });
});

describe("operator reading review: accessibility gate", () => {
  async function fillCommon() {
    fireEvent.change(screen.getByLabelText("Reviewer name"), {
      target: { value: "Editor Manusia" },
    });
    fireEvent.change(screen.getByLabelText("Review date"), {
      target: { value: "2026-11-01" },
    });
    const checklist = screen.getByRole("group", {
      name: "Manual accessibility checklist (required)",
    });
    for (const box of within(checklist).getAllByRole("checkbox")) {
      fireEvent.click(box);
    }
    fireEvent.click(
      screen.getByRole("checkbox", { name: /I have read this exact text/ }),
    );
  }

  it("lists every automatic check with a text status, not colour alone", async () => {
    stubReading(owner, "draft", "needs_override");
    renderReading();
    await openItem();
    const section = screen.getByRole("region", {
      name: "Automatic accessibility checks",
    });
    expect(within(section).getByText("Sentence length (at most 20 words)")).toBeInTheDocument();
    expect(
      within(section).getByText("Fails · can be excepted with a reason"),
    ).toBeInTheDocument();
    expect(within(section).getAllByText("Pass").length).toBe(7);
    expect(within(section).getByText("Not applicable")).toBeInTheDocument();
    expect(within(section).getByText(/22-word sentence/)).toBeInTheDocument();
    expect(within(section).getByText("content.seek_help_when.en")).toBeInTheDocument();
    // The list row carries the same result as words.
    expect(
      screen.getAllByText("Accessibility: needs a reason").length,
    ).toBeGreaterThan(0);
  });

  it("requires a reasoned override when only readability fails and sends it", async () => {
    const { calls } = stubReading(owner, "draft", "needs_override");
    renderReading();
    await openItem();
    await fillCommon();
    const submit = screen.getByRole("button", { name: "Approve for publication" });
    expect(submit).toHaveAttribute("aria-disabled", "true");
    const reason = screen.getByRole("textbox", {
      name: "Readability exception reason (required)",
    });
    expect(reason).toHaveAccessibleDescription(/Checks not met: Sentence length/);
    fireEvent.change(reason, { target: { value: "too short" } });
    expect(submit).toHaveAttribute("aria-disabled", "true");
    fireEvent.change(reason, {
      target: { value: "Emergency line kept whole so the number is not split." },
    });
    expect(submit).not.toHaveAttribute("aria-disabled");
    fireEvent.click(submit);
    await screen.findByText(/draft-one is approved/);
    const call = calls.find((c) => c.url.endsWith("/draft-one/approve"));
    expect(JSON.parse(String(call?.init.body)).accessibility).toEqual({
      checksVersion: "sela-reading-a11y-v1",
      manualVersion: "sela-reading-a11y-manual-v1",
      manualChecks: allManual,
      overrideReason: "Emergency line kept whole so the number is not split.",
    });
  });

  it("cannot be approved when a required check fails", async () => {
    stubReading(owner, "draft", "blocked");
    renderReading();
    await openItem();
    expect(
      screen.getByText("Fails · must be fixed"),
    ).toBeInTheDocument();
    const checklist = screen.getByRole("group", {
      name: "Manual accessibility checklist (required)",
    });
    for (const box of within(checklist).getAllByRole("checkbox")) {
      expect(box).toBeDisabled();
    }
    const submit = screen.getByRole("button", { name: "Approve for publication" });
    expect(submit).toHaveAttribute("aria-disabled", "true");
    expect(submit).toHaveAccessibleDescription(/can't be approved: a required accessibility check failed/);
    expect(
      screen.queryByRole("textbox", { name: /exception reason/ }),
    ).not.toBeInTheDocument();
  });

  it("shows a refusal inside the form, focuses it and marks the field invalid", async () => {
    stubReading(owner, "draft", "needs_override", {
      status: 422,
      code: "accessibility_override_required",
      field: "accessibility.overrideReason",
    });
    renderReading();
    await openItem();
    await fillCommon();
    const reason = screen.getByRole("textbox", {
      name: "Readability exception reason (required)",
    });
    fireEvent.change(reason, {
      target: { value: "A reason long enough for the client check." },
    });
    const submit = screen.getByRole("button", { name: "Approve for publication" });
    const form = submit.closest("form")!;
    fireEvent.click(submit);
    const alert = await within(form).findByText(
      "Give a reason for the readability exception, 20 to 300 characters.",
    );
    expect(alert).toHaveAttribute("role", "alert");
    await waitFor(() => expect(alert).toHaveFocus());
    expect(reason).toHaveAttribute("aria-invalid", "true");
    expect(reason).toHaveAccessibleDescription(/Give a reason for the readability exception/);
  });
});

describe("operator reading review: unavailable approve button", () => {
  it("stays focusable, says what is missing and sends nothing", async () => {
    const { calls } = stubReading(owner);
    renderReading();
    await openItem();
    const submit = screen.getByRole("button", { name: "Approve for publication" });
    expect(submit).not.toBeDisabled();
    submit.focus();
    expect(submit).toHaveFocus();
    expect(submit).toHaveAttribute("aria-disabled", "true");
    expect(submit).toHaveAccessibleDescription(
      /Still needed: reviewer name, review date, accessibility checklist \(0 of 3\), confirmation/,
    );
    fireEvent.click(submit);
    expect(calls.some((c) => c.url.endsWith("/approve"))).toBe(false);
  });
});

describe("operator reading review: focus and live regions", () => {
  it("keeps status and alert live regions mounted before any message", async () => {
    stubReading(owner);
    renderReading();
    await screen.findByRole("heading", { name: "Reading reviews" });
    expect(screen.getAllByRole("status").length).toBeGreaterThan(0);
    expect(screen.getAllByRole("alert").length).toBeGreaterThan(0);
  });

  it("moves focus to the opened item and keeps it inside the revoke step", async () => {
    stubReading(owner, "approved");
    renderReading();
    await screen.findByRole("heading", { name: "Reading reviews" });
    fireEvent.click(await screen.findByRole("button", { name: /Exercise title/ }));
    const heading = await screen.findByRole("heading", { level: 3, name: /Exercise title/ });
    await waitFor(() => expect(heading).toHaveFocus());

    fireEvent.click(screen.getByRole("button", { name: "End publication" }));
    const reasonInput = await screen.findByLabelText("Reason (optional)");
    await waitFor(() => expect(reasonInput).toHaveFocus());
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "End publication" })).toHaveFocus(),
    );
  });

  it("names a source link that opens a new tab", async () => {
    stubReading(owner);
    renderReading();
    await openItem();
    expect(
      screen.getByRole("link", { name: "Source one (opens in a new tab)" }),
    ).toHaveAttribute("target", "_blank");
  });
});
