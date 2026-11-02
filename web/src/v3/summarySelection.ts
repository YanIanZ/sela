/**
 * Owner-side navigation state for "Pilih untuk ringkasan" (LT-11).
 *
 * The Life Context Timeline hands the summary composer the sources the
 * owner chose as record ids with exact versions. The hand-off lives in this
 * module's memory only: never in the URL, browser storage, analytics or a
 * server draft, and never with any text. A reload drops it. The composer
 * takes it once and preselects a source only while it is still at the
 * chosen version.
 */
export type SourceSelection = { recordId: string; version: number };

export const MAX_SUMMARY_SOURCES = 12;

let pending: SourceSelection[] | null = null;

export function setSummarySelection(refs: SourceSelection[]) {
  pending = refs
    .slice(0, MAX_SUMMARY_SOURCES)
    .map(({ recordId, version }) => ({ recordId, version }));
}

/** Consumes the pending selection against the composer's current sources:
 * ids still at the chosen version, and how many were left out because the
 * source changed or is gone. Null when nothing was handed over. */
export function takeSummarySelection(
  current: Array<{ id: string; version: number }>,
): { ids: string[]; dropped: number } | null {
  if (!pending) return null;
  const chosen = pending;
  pending = null;
  const ids = chosen
    .filter((p) => current.some((r) => r.id === p.recordId && r.version === p.version))
    .map((p) => p.recordId);
  return { ids, dropped: chosen.length - ids.length };
}

/** Test and sign-out helper. */
export function clearSummarySelection() {
  pending = null;
}
