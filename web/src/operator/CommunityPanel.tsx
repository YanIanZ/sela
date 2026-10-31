import { useCallback, useEffect, useState } from "react";
import { useWords } from "../v3/shared";
import { AdminError, adminApi, type CommunityQueueItem } from "./api";

const LABELS: Record<string, [string, string]> = {
  bullying: ["Perundungan", "Bullying"],
  harassment: ["Pelecehan", "Harassment"],
  hate: ["Ujaran kebencian", "Hate"],
  sexual: ["Seksual", "Sexual"],
  self_harm_encouragement: ["Mendorong menyakiti diri", "Self-harm encouragement"],
  violent_threat: ["Ancaman kekerasan", "Violent threat"],
  personal_info: ["Data pribadi", "Personal info"],
  spam: ["Spam", "Spam"],
  reported: ["Dilaporkan pengguna", "Reported by users"],
  moderation_unavailable: ["Moderasi tidak tersedia", "Moderation unavailable"],
  moderation_unclear: ["Moderasi ragu", "Moderation unsure"],
};

/**
 * CommunityPanel is the moderation queue of the sharing space (D-072). It
 * shows only held, hidden and reported items, never an author or profile.
 */
export function CommunityPanel({ onExpired }: { onExpired: () => void }) {
  const w = useWords();
  const [items, setItems] = useState<CommunityQueueItem[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState("");
  const [reason, setReason] = useState<Record<string, string>>({});

  const fail = useCallback(
    (err: unknown) => {
      if (err instanceof AdminError && err.status === 401) onExpired();
      else setError(w("Antrean belum bisa dimuat.", "The queue could not be loaded."));
    },
    [onExpired, w],
  );

  const load = useCallback(() => {
    setError("");
    adminApi.communityQueue().then(setItems).catch(fail);
  }, [fail]);
  useEffect(load, [load]);

  const decide = async (item: CommunityQueueItem, action: "restore" | "remove") => {
    setBusy(item.id);
    try {
      await adminApi.communityDecide(item.kind, item.id, action, reason[item.id] ?? "");
      load();
    } catch (err) {
      fail(err);
    } finally {
      setBusy("");
    }
  };

  return (
    <section className="op-section" aria-labelledby="op-community">
      <h2 id="op-community">{w("Moderasi ruang berbagi", "Sharing space moderation")}</h2>
      <p>
        {w(
          "Tulisan yang ditahan, disembunyikan, atau dilaporkan. Penulis tidak pernah ditampilkan. Setiap keputusan tercatat di audit.",
          "Posts that are held, hidden or reported. Authors are never shown. Every decision is audited.",
        )}
      </p>
      {error && <p role="alert">{error}</p>}
      {items === null ? (
        <p>{w("Memuat antrean…", "Loading the queue…")}</p>
      ) : items.length === 0 ? (
        <p>{w("Antrean kosong.", "The queue is empty.")}</p>
      ) : (
        <ul className="op-community-list">
          {items.map((it) => (
            <li key={it.kind + it.id} className="op-community-item">
              <p className="op-community-meta">
                <strong>{it.kind === "post" ? w("Cerita", "Story") : w("Balasan", "Reply")}</strong> · {it.alias} ·{" "}
                {it.status === "held" ? w("Ditahan", "Held") : it.status === "hidden" ? w("Disembunyikan", "Hidden") : w("Tayang", "Published")}
                {it.reports > 0 ? ` · ${it.reports} ${w("laporan", "reports")}` : ""}
              </p>
              <blockquote className="op-community-body">{it.body}</blockquote>
              {it.labels.length > 0 && (
                <p className="op-community-labels">
                  {it.labels.map((l) => (
                    <span key={l}>{LABELS[l] ? w(LABELS[l][0], LABELS[l][1]) : l}</span>
                  ))}
                </p>
              )}
              <div className="op-inline">
                <label className="op-community-reason">
                  <span>{w("Catatan (opsional)", "Note (optional)")}</span>
                  <input
                    value={reason[it.id] ?? ""}
                    maxLength={200}
                    onChange={(e) => setReason((r) => ({ ...r, [it.id]: e.target.value }))}
                  />
                </label>
                <button type="button" className="op-btn" disabled={busy === it.id} onClick={() => void decide(it, "restore")}>
                  {w("Tayangkan", "Restore")}
                </button>
                <button type="button" className="op-btn op-btn--primary" disabled={busy === it.id} onClick={() => void decide(it, "remove")}>
                  {w("Hapus permanen", "Remove")}
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
