import { useEffect, useState, type FormEvent } from "react";
import { Link, useRouter } from "../../lib/router";
import { api, type Entry } from "../api";
import { useLocale, useWords } from "../shared";
import {
  Button,
  CheckboxRow,
  EmptyState,
  ErrorNotice,
  LinkButton,
  Loading,
  PageHeader,
  TextArea,
} from "../ui";
import { recordsInWeek, weekWindow } from "../weekly";
import { entryRoute } from "./Continuity";

export function WeeklyPage() {
  const w = useWords();
  const loc = useLocale();
  const { navigate } = useRouter();
  const [offset, setOffset] = useState(0);
  const [records, setRecords] = useState<Entry[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const [text, setText] = useState("");
  const [allow, setAllow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [now] = useState(() => new Date());
  const load = () => {
    setError(null);
    api<{ records: Entry[] }>("/records")
      .then((d) => setRecords(d.records))
      .catch(setError);
  };
  useEffect(load, []);
  const { days } = weekWindow(now, offset);
  const selected = recordsInWeek(records ?? [], now, offset);
  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const r = await api<Entry>(
        "/records",
        {
          kind: "note",
          title:
            w("Refleksi minggu ", "Reflection for the week of ") +
            loc.shortDate(days[0] + "T12:00:00"),
          text,
          storageGranted: true,
        },
        "POST",
      );
      navigate(`/notes/${r.id}`);
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="c-weekly">
      <PageHeader
        back={{ to: "/notes", label: w("Catatanku", "My notes") }}
        title={w("Lihat kembali, pelan-pelan", "Look back, at your pace")}
        lead={w(
          "Satu minggu dari tulisanmu sendiri. Tanpa skor, kesimpulan otomatis, atau kewajiban mengisi setiap hari.",
          "One week of your own writing. No scores, automatic conclusions, or obligation to write every day.",
        )}
      />
      <ErrorNotice error={error} onRetry={load} />
      <nav
        className="c-week-nav"
        aria-label={w("Pilih minggu", "Choose a week")}
      >
        <Button onClick={() => setOffset((x) => x - 1)}>
          {w("Minggu sebelumnya", "Previous week")}
        </Button>
        <h2>
          {loc.shortDate(days[0] + "T12:00:00")} –{" "}
          {loc.shortDate(days[6] + "T12:00:00")}
        </h2>
        <Button disabled={offset === 0} onClick={() => setOffset((x) => x + 1)}>
          {w("Minggu berikutnya", "Next week")}
        </Button>
      </nav>
      {!records && !error && (
        <Loading label={w("Membuka tulisanmu…", "Opening your writing…")} />
      )}
      {records && (
        <>
          <ol
            className="c-week-strip"
            aria-label={w("Tulisan per hari", "Writing by day")}
          >
            {days.map((day) => {
              const count = selected.filter(
                (r) =>
                  (r.kind === "checkin" && r.day
                    ? r.day
                    : new Date(r.createdAt).toLocaleDateString("sv-SE")) ===
                  day,
              ).length;
              return (
                <li key={day} data-filled={count > 0}>
                  <time dateTime={day}>
                    {new Intl.DateTimeFormat(loc.locale, {
                      weekday: "short",
                    }).format(new Date(day + "T12:00:00"))}
                  </time>
                  <span>{new Date(day + "T12:00:00").getDate()}</span>
                  <small>
                    {count ? w("Ada tulisan", "Writing saved") : w("—", "—")}
                  </small>
                </li>
              );
            })}
          </ol>
          <div className="c-week-content">
            <section>
              <h2>
                {w("Yang kamu simpan minggu ini", "What you saved this week")}
              </h2>
              {selected.length === 0 ? (
                <EmptyState
                  title={w(
                    "Minggu ini belum berisi tulisan",
                    "No writing this week",
                  )}
                  body={w(
                    "Hari tanpa catatan tetap bagian dari hidupmu. Kamu boleh mulai kapan saja.",
                    "Days without notes are still part of your life. Start whenever you want.",
                  )}
                  actions={
                    <LinkButton to="/notes/write">
                      {w("Tulis catatan", "Write a note")}
                    </LinkButton>
                  }
                />
              ) : (
                <ul className="c-week-records">
                  {selected.map((r) => (
                    <li key={r.id}>
                      <Link to={entryRoute(r)}>
                        {r.title || w("Tulisanmu", "Your writing")}
                      </Link>
                      <time dateTime={r.createdAt}>
                        {loc.shortDate(r.createdAt)}
                      </time>
                    </li>
                  ))}
                </ul>
              )}
            </section>
            <form className="c-week-reflection" onSubmit={save}>
              <h2>
                {w(
                  "Apa yang ingin kamu bawa ke depan?",
                  "What would you like to carry forward?",
                )}
              </h2>
              <p>
                {w(
                  "Apa yang terasa penting? Adakah hal kecil yang ingin kamu ingat?",
                  "What felt important? Is there something small you want to remember?",
                )}
              </p>
              <TextArea
                label={w("Refleksimu", "Your reflection")}
                value={text}
                onChange={(e) => setText(e.target.value)}
                maxLength={6000}
                rows={6}
                required
              />
              <CheckboxRow checked={allow} onChange={setAllow}>
                {w(
                  "Simpan sebagai catatan pribadiku",
                  "Save as my personal note",
                )}
              </CheckboxRow>
              <Button
                variant="primary"
                type="submit"
                disabled={!allow || !text.trim() || busy}
              >
                {w("Simpan refleksi", "Save reflection")}
              </Button>
            </form>
          </div>
        </>
      )}
    </div>
  );
}
