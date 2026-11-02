import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useRouter } from "../../lib/router";
import { api, localDay, V3Error, type Entry } from "../api";
import { usePrefs } from "../prefs";
import { useAnnounce, useLocale, useWords } from "../shared";
import { Icon, type IconName } from "../icons";
import {
  Button,
  EmptyState,
  ErrorNotice,
  LinkButton,
  Loading,
  PageHeader,
  ProvenanceBadge,
  StatusLine,
  Subnav,
  buttonClass,
} from "../ui";
import { sourceText } from "./Continuity";
import { moodLabel } from "./Notes";
import {
  MAX_SUMMARY_SOURCES,
  setSummarySelection,
  type SourceSelection,
} from "../summarySelection";
import "../timeline.css";

/**
 * Life Context Timeline (BW-003, LIFE_CONTEXT_TIMELINE.md). A read-only,
 * chronological view of the sources the owner chose to save, with each
 * source's version and meaning status. No AI runs here, nothing is scored or
 * interpreted, and no text is put in a URL: the only query parameters are
 * the kind and window filters.
 */

type TimelineKind = "note" | "checkin" | "conversation";
type MeaningStatus = "current" | "stale" | "needs_review";

export type TimelineItem = {
  source: { recordId: string; version: number; kind: TimelineKind };
  occurredAt: string;
  updatedAt: string;
  day?: string;
  title: string;
  excerpt: string;
  truncated: boolean;
  mood?: string;
  energy?: string;
  provenance: string;
  meaningStatus: MeaningStatus;
  stale?: { reason: string; at: string };
  pendingReflections?: number;
};

export type TimelinePage = {
  items: TimelineItem[];
  nextCursor: string | null;
  count: number;
  aiUsed: boolean;
};

const KINDS: TimelineKind[] = ["note", "checkin", "conversation"];
const WINDOWS = [7, 30, 90] as const;
type Window = (typeof WINDOWS)[number];

const kindIcon: Record<TimelineKind, IconName> = {
  note: "notes",
  checkin: "interpretation",
  conversation: "talk",
};

function parseKinds(raw: string | null): TimelineKind[] {
  if (!raw) return KINDS;
  const list = raw.split(",").filter((k): k is TimelineKind => (KINDS as string[]).includes(k));
  return list.length ? KINDS.filter((k) => list.includes(k)) : KINDS;
}

function parseWindow(raw: string | null): Window {
  const n = Number(raw);
  return (WINDOWS as readonly number[]).includes(n) ? (n as Window) : 90;
}

export const sourceRoute = (item: TimelineItem) =>
  item.source.kind === "conversation"
    ? `/talk?record=${item.source.recordId}`
    : `/notes/${item.source.recordId}`;

type Expanded =
  | { state: "loading" }
  | { state: "ready"; entry: Entry }
  | { state: "error"; error: unknown };

export function ContextTimelinePage() {
  const w = useWords();
  const loc = useLocale();
  const { query, navigate } = useRouter();
  const { prefs } = usePrefs();
  const { announce } = useAnnounce();
  const kinds = parseKinds(query.get("kinds"));
  const days = parseWindow(query.get("days"));
  const kindsKey = kinds.join(",");

  const [items, setItems] = useState<TimelineItem[] | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [moreError, setMoreError] = useState<unknown>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [selected, setSelected] = useState<SourceSelection[]>([]);
  const [expanded, setExpanded] = useState<Record<string, Expanded>>({});
  const [selectionHint, setSelectionHint] = useState("");
  // Whether anything is saved at all, asked only when a filtered page is
  // empty, so the empty state never claims "nothing saved" wrongly.
  const [anySaved, setAnySaved] = useState<boolean | null>(null);
  const focusAfterLoad = useRef<string | null>(null);
  const request = useRef(0);

  const params = useCallback(
    (after?: string) => {
      const p = new URLSearchParams({ days: String(days), kinds: kindsKey, limit: "20" });
      if (after) p.set("cursor", after);
      return p.toString();
    },
    [days, kindsKey],
  );

  const load = useCallback(() => {
    const id = ++request.current;
    setError(null);
    setMoreError(null);
    setItems(null);
    setCursor(null);
    setAnySaved(null);
    setExpanded({});
    api<TimelinePage>(`/context-timeline?${params()}`)
      .then(async (page) => {
        if (id !== request.current) return;
        if (page.items.length === 0) {
          const probe = await api<TimelinePage>("/context-timeline?limit=1").catch(() => null);
          if (id !== request.current) return;
          setAnySaved(probe ? probe.items.length > 0 : true);
        }
        setItems(page.items);
        setCursor(page.nextCursor);
      })
      .catch((e) => {
        if (id !== request.current) return;
        setError(e);
      });
  }, [params]);

  // Every visit and every filter change reads the server again: a revised
  // or deleted source is never shown from a stale client copy (LT-07).
  useEffect(load, [load]);

  const loadMore = () => {
    if (!cursor || loadingMore) return;
    const id = request.current;
    setLoadingMore(true);
    setMoreError(null);
    api<TimelinePage>(`/context-timeline?${params(cursor)}`)
      .then((page) => {
        if (id !== request.current) return;
        const first = page.items[0];
        focusAfterLoad.current = first ? first.source.recordId : null;
        setItems((cur) => [...(cur ?? []), ...page.items]);
        setCursor(page.nextCursor);
        announce(
          "status",
          page.items.length === 1
            ? w("1 cerita lagi dimuat.", "1 more story loaded.")
            : w(`${page.items.length} cerita lagi dimuat.`, `${page.items.length} more stories loaded.`),
        );
      })
      .catch((e) => {
        if (id === request.current) setMoreError(e);
      })
      .finally(() => {
        if (id === request.current) setLoadingMore(false);
      });
  };

  // Progressive loading keeps the keyboard place: focus moves to the first
  // newly loaded story, not back to the top or to the end of the list.
  useEffect(() => {
    const id = focusAfterLoad.current;
    if (!id) return;
    focusAfterLoad.current = null;
    document.getElementById(`ct-item-${id}`)?.focus();
  }, [items]);

  const setFilters = (next: { kinds?: TimelineKind[]; days?: Window }) => {
    const k = next.kinds ?? kinds;
    const d = next.days ?? days;
    const p = new URLSearchParams();
    if (k.length !== KINDS.length) p.set("kinds", k.join(","));
    if (d !== 90) p.set("days", String(d));
    const qs = p.toString();
    navigate("/notes/context" + (qs ? "?" + qs : ""), { replace: true });
  };

  const toggleKind = (kind: TimelineKind) => {
    const has = kinds.includes(kind);
    if (has && kinds.length === 1) return; // at least one kind stays chosen
    setFilters({ kinds: has ? kinds.filter((k) => k !== kind) : KINDS.filter((k) => kinds.includes(k) || k === kind) });
  };

  const grouped = useMemo(() => {
    const map = new Map<string, TimelineItem[]>();
    for (const it of items ?? []) {
      const key = localDay(new Date(it.occurredAt));
      const list = map.get(key) ?? [];
      list.push(it);
      map.set(key, list);
    }
    return [...map.entries()];
  }, [items]);

  const isSelected = (it: TimelineItem) => selected.some((s) => s.recordId === it.source.recordId);
  const toggleSelected = (it: TimelineItem, checked: boolean) => {
    setSelectionHint("");
    if (!checked) {
      setSelected((cur) => cur.filter((s) => s.recordId !== it.source.recordId));
      return;
    }
    if (selected.length >= MAX_SUMMARY_SOURCES) {
      setSelectionHint(w("Maksimal 12 sumber untuk satu ringkasan.", "At most 12 sources for one summary."));
      return;
    }
    setSelected((cur) => [...cur, { recordId: it.source.recordId, version: it.source.version }]);
  };

  const goToSummary = () => {
    if (selected.length === 0) {
      setSelectionHint(
        w("Pilih setidaknya satu cerita dulu.", "Choose at least one story first."),
      );
      return;
    }
    // Ids and versions only, through in-memory navigation state (LT-11).
    setSummarySelection(selected);
    navigate("/support/summaries/new");
  };

  const toggleSource = (it: TimelineItem) => {
    const id = it.source.recordId;
    if (expanded[id]) {
      setExpanded((cur) => {
        const next = { ...cur };
        delete next[id];
        return next;
      });
      return;
    }
    setExpanded((cur) => ({ ...cur, [id]: { state: "loading" } }));
    api<Entry>(`/records/${encodeURIComponent(id)}`)
      .then((entry) => setExpanded((cur) => (cur[id] ? { ...cur, [id]: { state: "ready", entry } } : cur)))
      .catch((error) => setExpanded((cur) => (cur[id] ? { ...cur, [id]: { state: "error", error } } : cur)));
  };

  const kindLabel = (k: TimelineKind) =>
    k === "conversation"
      ? w("Percakapan tersimpan", "Saved conversation")
      : k === "checkin"
        ? w("Cek perasaan", "Check-in")
        : w("Catatan", "Note");

  return (
    <div className="ct-page">
      <PageHeader
        back={{ to: "/notes", label: w("Catatanku", "My notes") }}
        title={w("Perjalanan ceritaku", "My story over time")}
        lead={w(
          "Lihat pengalaman yang kamu simpan, berdasarkan waktunya—tanpa tebakan tentang kondisimu.",
          "See the experiences you chose to save in chronological order—without guesses about your condition.",
        )}
      />
      <Subnav
        label={w("Bagian Catatanku", "My notes sections")}
        current="/notes/context"
        items={[
          { to: "/notes", label: w("Linimasa", "Timeline") },
          { to: "/notes/context", label: w("Perjalanan cerita", "Story over time") },
          { to: "/notes/weekly", label: w("Refleksi mingguan", "Weekly reflection") },
          { to: "/notes/memory", label: w("Yang Sela ingat", "What Sela remembers") },
        ]}
      />
      <p className="s-hint ct-note">
        <Icon name="info" size={16} />{" "}
        {w(
          "Urutan ini hanya menurut waktu. Halaman ini tidak memakai AI, tidak memberi skor, dan tidak menyimpulkan penyebab atau kondisimu.",
          "This order is by time only. This page uses no AI, gives no score, and draws no conclusion about causes or your condition.",
        )}
      </p>

      <div className="s-filters" role="group" aria-labelledby="ct-kinds-label">
        <span className="s-filters-label" id="ct-kinds-label">
          {w("Jenis", "Kind")}
        </span>
        {KINDS.map((k) => (
          <label key={k} className="s-filter">
            <input type="checkbox" checked={kinds.includes(k)} onChange={() => toggleKind(k)} />
            {kinds.includes(k) && <Icon name="check" size={16} />}
            <span>{kindLabel(k)}</span>
          </label>
        ))}
      </div>
      <div className="s-filters" role="radiogroup" aria-labelledby="ct-window-label">
        <span className="s-filters-label" id="ct-window-label">
          {w("Rentang waktu", "Time range")}
        </span>
        {WINDOWS.map((d) => (
          <label key={d} className="s-filter">
            <input
              type="radio"
              name="ct-window"
              checked={days === d}
              onChange={() => setFilters({ days: d })}
            />
            {days === d && <Icon name="check" size={16} />}
            <span>{w(`${d} hari terakhir`, `Last ${d} days`)}</span>
          </label>
        ))}
      </div>

      <ErrorNotice error={error} onRetry={load} />

      {items === null && !error ? (
        <Loading label={w("Memuat perjalanan ceritamu…", "Loading your story over time…")} />
      ) : items && items.length === 0 ? (
        anySaved ? (
          <EmptyState
            title={w("Tidak ada cerita dalam pilihan ini.", "No stories for this choice.")}
            body={w(
              "Coba rentang waktu yang lebih panjang atau tampilkan semua jenis.",
              "Try a longer time range or show every kind.",
            )}
            actions={
              kinds.length === KINDS.length && days === 90 ? (
                <LinkButton to="/notes" variant="secondary">
                  {w("Lihat semua di Linimasa", "See everything in Timeline")}
                </LinkButton>
              ) : (
                <Button variant="secondary" onClick={() => setFilters({ kinds: KINDS, days: 90 })}>
                  {w("Tampilkan semua, 90 hari", "Show everything, 90 days")}
                </Button>
              )
            }
          />
        ) : (
          <EmptyState
            title={w("Belum ada cerita tersimpan.", "No saved stories yet.")}
            body={w(
              "Kamu bisa menulis atau memilih untuk menyimpan percakapan.",
              "You can write, or choose to save a conversation.",
            )}
            actions={
              <LinkButton to="/notes/write" variant="secondary">
                {w("Tulis catatan", "Write a note")}
              </LinkButton>
            }
            mark
          />
        )
      ) : items ? (
        <>
          {grouped.map(([day, list]) => (
            <section className="ct-day" key={day} aria-labelledby={`ct-day-${day}`}>
              <h2 id={`ct-day-${day}`}>
                {loc.dayHeading(list[0]!.occurredAt)}
              </h2>
              <ol className="ct-list">
                {list.map((it) => {
                  const id = it.source.recordId;
                  const headingId = `ct-title-${id}`;
                  const panelId = `ct-source-${id}`;
                  const open = expanded[id];
                  return (
                    <li key={id}>
                      <article
                        className={`ct-item ct-item--${it.meaningStatus}`}
                        aria-labelledby={headingId}
                        id={`ct-item-${id}`}
                        tabIndex={-1}
                      >
                        <div className="ct-item-head">
                          <Icon name={kindIcon[it.source.kind]} />
                          <h3 id={headingId}>{it.title.trim() || kindLabel(it.source.kind)}</h3>
                        </div>
                        <p className="ct-meta">
                          <span>{kindLabel(it.source.kind)}</span>
                          {" · "}
                          <time dateTime={it.occurredAt}>{loc.time(it.occurredAt)}</time>
                          {it.source.kind === "checkin" && it.mood
                            ? " · " + moodLabel(it.mood, loc.lang)
                            : ""}
                        </p>
                        <div className="ct-badges">
                          <ProvenanceBadge value={it.provenance} />
                          <span className="s-badge ct-version">
                            {w(`Versi ${it.source.version}`, `Version ${it.source.version}`)}
                          </span>
                          <MeaningChip status={it.meaningStatus} />
                        </div>
                        {it.meaningStatus === "stale" && (
                          <p className="ct-explain">
                            {w(
                              "Kamu mengubah tulisan ini setelah ada ringkasan atau ingatan yang memakai versi lamanya. Bagian itu tidak dibagikan sampai kamu meninjaunya lagi.",
                              "You changed this after a summary or memory used its earlier version. That part is not shared until you review it again.",
                            )}
                          </p>
                        )}
                        {it.meaningStatus === "needs_review" && (
                          <p className="ct-explain">
                            {w(
                              "Ada pemahaman sementara dari Sela di percakapan ini yang belum kamu tinjau.",
                              "There is a tentative reflection from Sela in this conversation that you haven't reviewed.",
                            )}
                          </p>
                        )}
                        {!prefs.hidePreviews && it.excerpt && !open && (
                          <p className="ct-excerpt">
                            {it.excerpt}
                            {it.truncated ? "…" : ""}
                          </p>
                        )}
                        {open && (
                          <div className="ct-source" id={panelId}>
                            <SourcePanel item={it} state={open} />
                          </div>
                        )}
                        <div className="ct-actions">
                          <button
                            type="button"
                            className={buttonClass("quiet")}
                            aria-expanded={Boolean(open)}
                            aria-controls={open ? panelId : undefined}
                            onClick={() => toggleSource(it)}
                          >
                            <Icon name={open ? "eye-off" : "eye"} size={16} />
                            {open
                              ? w("Tutup sumber asli", "Hide original source")
                              : w("Lihat sumber asli", "Show original source")}
                          </button>
                          <Link to={sourceRoute(it)} className={buttonClass("quiet")}>
                            {it.source.kind === "conversation"
                              ? w("Buka percakapan", "Open conversation")
                              : w("Buka catatan", "Open entry")}
                          </Link>
                          <label className="ct-select">
                            <input
                              type="checkbox"
                              checked={isSelected(it)}
                              onChange={(e) => toggleSelected(it, e.target.checked)}
                              aria-describedby={headingId}
                            />
                            <span>{w("Bawa ke ringkasan", "Bring to a summary")}</span>
                          </label>
                        </div>
                      </article>
                    </li>
                  );
                })}
              </ol>
            </section>
          ))}
          <ErrorNotice error={moreError} onRetry={loadMore} />
          {cursor && (
            <div className="s-actions ct-more">
              <Button variant="secondary" onClick={loadMore} disabled={loadingMore}>
                {loadingMore ? w("Memuat…", "Loading…") : w("Muat cerita sebelumnya", "Load earlier stories")}
              </Button>
            </div>
          )}
        </>
      ) : null}

      {items && items.length > 0 && (
        <section className="ct-summary" aria-labelledby="ct-summary-title">
          <h2 id="ct-summary-title">{w("Siapkan ringkasan", "Prepare a summary")}</h2>
          <p className="s-hint">
            {w(
              "Yang kamu pilih dibawa sebagai versi saat ini. Kamu tetap memeriksa kata-katanya di halaman ringkasan; tidak ada yang dibagikan otomatis.",
              "What you choose is carried at its current version. You still review the words on the summary page; nothing is shared automatically.",
            )}
          </p>
          <p role="status" className="ct-count">
            {selected.length === 0
              ? w("Belum ada yang dipilih.", "Nothing chosen yet.")
              : w(`${selected.length} cerita dipilih.`, `${selected.length} stories chosen.`)}
          </p>
          {selectionHint && <StatusLine tone="warning">{selectionHint}</StatusLine>}
          <Button
            variant="secondary"
            onClick={goToSummary}
            aria-disabled={selected.length === 0 ? true : undefined}
          >
            {w("Pilih untuk ringkasan", "Select for summary")}
          </Button>
        </section>
      )}
    </div>
  );
}

function MeaningChip({ status }: { status: MeaningStatus }) {
  const w = useWords();
  const map: Record<MeaningStatus, [string, string, IconName]> = {
    current: ["Terkini", "Current", "check"],
    stale: ["Perlu ditinjau ulang", "Needs review again", "alert"],
    needs_review: ["Menunggu tinjauanmu", "Waiting for your review", "interpretation"],
  };
  const [id, en, icon] = map[status] ?? map.current;
  return (
    <span className={`s-status-chip ct-status ct-status--${status}`}>
      <Icon name={icon} size={16} />
      {w(id, en)}
    </span>
  );
}

function SourcePanel({ item, state }: { item: TimelineItem; state: Expanded }) {
  const w = useWords();
  if (state.state === "loading") {
    return (
      <p className="s-loading" role="status">
        {w("Membuka sumber asli…", "Opening the original source…")}
      </p>
    );
  }
  if (state.state === "error") {
    const gone = state.error instanceof V3Error && state.error.code === "not_found";
    return gone ? (
      <StatusLine tone="warning">
        {w(
          "Sumber ini sudah tidak ada. Muat ulang halaman untuk melihat daftar terbaru.",
          "This source no longer exists. Reload the page to see the latest list.",
        )}
      </StatusLine>
    ) : (
      <ErrorNotice error={state.error} />
    );
  }
  const entry = state.entry;
  const text = sourceText(entry);
  return (
    <>
      {entry.version !== item.source.version && (
        <StatusLine tone="warning">
          {w(
            `Sumber ini sudah berubah ke versi ${entry.version} sejak daftar dimuat. Yang tampil di bawah adalah versi terbarunya.`,
            `This source changed to version ${entry.version} after the list loaded. Below is its latest version.`,
          )}
        </StatusLine>
      )}
      <p className="ct-source-label">
        {w(`Tulisanmu · versi ${entry.version}`, `Your words · version ${entry.version}`)}
      </p>
      <div className="ct-source-text">
        {text.trim() ? text : w("Tanpa teks.", "No text.")}
      </div>
    </>
  );
}
