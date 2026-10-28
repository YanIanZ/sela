/// <reference types="vite/client" />
import { useState } from "react";
import { createRoot } from "react-dom/client";
import { I18nProvider } from "./i18n";
import { RouterProvider } from "./lib/router";
import { PrefsProvider } from "./v3/prefs";
import { AnnounceProvider, useWords } from "./v3/shared";
import { DesignPreviewSession } from "./v3/session";
import { DashboardPage } from "./v3/screens/Dashboard";
import { HomePage } from "./v3/screens/Home";
import { AppShell } from "./v3/shell";
import type { Entry, Profile } from "./v3/api";
import "./v3/styles.css";

const now = new Date().toISOString();
const profile: Profile = {
  id: "design-preview", kind: "guest", username: null, locale: "id",
  ageBand: "18_plus", version: 1, createdAt: now, lastActiveAt: now,
  guest: null, recovery: null, guardianConsent: null, actionsRequired: [],
  environment: "design-preview", synthetic: true,
};
const records: Entry[] = [
  { id: "example-note", kind: "note", title: "Contoh catatan", text: "Hari ini aku ingin memberi ruang untuk istirahat." },
  { id: "example-checkin", kind: "checkin", title: "Contoh cek perasaan", text: "Sedang mencoba berjalan lebih pelan.", mood: "okay" },
  { id: "example-memory", kind: "memory", title: "Contoh memori", text: "Berjalan sebentar membantuku mengambil jeda." },
].map(record => ({ ...record, version: 1, provenance: "synthetic-design-preview", reviewStatus: "unreviewed", safety: "unknown", createdAt: now, updatedAt: now })) as Entry[];

function PreviewScreen() {
  const w = useWords();
  const [view, setView] = useState<"dashboard" | "home">("dashboard");
  return <DesignPreviewSession profile={view === "home" ? profile : null}><div onClickCapture={event => {
    if ((event.target as Element).closest("a, button, input, select, textarea")) {
      const anchor = (event.target as Element).closest("a");
      if (["/home", "/start/age"].includes(anchor?.getAttribute("href") ?? "")) setView("home");
      if (["/dashboard", "/"].includes(anchor?.getAttribute("href") ?? "")) setView("dashboard");
      event.preventDefault(); event.stopPropagation();
    }
  }}>
    <p className={`s-design-preview-notice${view === "home" ? " s-design-preview-notice--workspace" : ""}`} role="status">{w("Pratinjau desain · data contoh. Tidak disimpan atau dikirim ke AI. Navigasi Dashboard–Beranda tersedia; aksi lainnya dinonaktifkan.", "Design preview · sample data. Nothing is saved or sent to AI. Dashboard–Home navigation is available; other actions are disabled.")}</p>
    <AppShell title={view === "dashboard" ? w("Dashboard Sela", "Sela dashboard") : w("Beranda", "Home")}>{view === "dashboard" ? <DashboardPage /> : <HomePage user={profile} previewEntries={records} />}</AppShell>
  </div></DesignPreviewSession>;
}

// This standalone HTML entry is never included by the production Vite build.
// Block product actions so previewing cannot navigate into a real account flow.
const root = document.getElementById("root");
if (import.meta.env.DEV && root) createRoot(root).render(
  <I18nProvider><RouterProvider><PrefsProvider><AnnounceProvider>
    <PreviewScreen />
  </AnnounceProvider></PrefsProvider></RouterProvider></I18nProvider>,
);
