import { useEffect, useState } from "react";
import { api, type Policy } from "../api";
import { useConsent } from "../consent";
import { EmergencySheet } from "../emergency";
import { Icon } from "../icons";
import { useI18n } from "../../i18n";
import { useAnnounce, useWords } from "../shared";
import { Button, CheckboxRow, ErrorNotice, LinkButton, Loading } from "../ui";

type Stage = "everyday" | "attention" | "support" | "urgent";

type ConditionMap = {
  stage: Stage;
  signals: { key: string; count: number; days: number }[];
  dataPoints: number;
  windowDays: number;
  dataLimited: boolean;
  sources: Record<string, number>;
  computedAt: string;
};

type Practitioner = {
  id: string;
  name: string;
  role: { id: string; en: string };
  focus: { id: string; en: string };
  modes: string[];
  city: string;
  languages: string[];
  stages: Stage[];
  synthetic: boolean;
};

const stageText: Record<Stage, { id: string; en: string; leadId: string; leadEn: string }> = {
  everyday: {
    id: "Sehari-hari",
    en: "Everyday",
    leadId: "Catatanmu belakangan ini tidak menunjukkan tanda yang berat. Terus jaga ritmemu dan tetap cerita kapan pun perlu.",
    leadEn: "Your recent entries show no heavy signs. Keep your rhythm, and keep talking whenever you need to.",
  },
  attention: {
    id: "Perlu perhatian",
    en: "Needs attention",
    leadId: "Ada beberapa hal berat yang muncul. Ini saat yang baik untuk memberi ruang bagi dirimu dan, kalau mau, ngobrol dengan orang yang kamu percaya.",
    leadEn: "Some heavy things have come up. It's a good moment to give yourself room and, if you like, talk with someone you trust.",
  },
  support: {
    id: "Butuh dukungan",
    en: "Needs support",
    leadId: "Yang kamu alami tampak cukup berat dan berulang. Berbicara dengan tenaga profesional bisa membantu; kamu bisa menyiapkan apa yang ingin dibawa.",
    leadEn: "What you're going through looks heavy and recurring. Talking with a professional could help; you can prepare what you want to bring.",
  },
  urgent: {
    id: "Butuh bantuan segera",
    en: "Needs help now",
    leadId: "Ada tulisanmu yang terbaca sebagai bahaya dalam seminggu terakhir. Utamakan keselamatanmu: hubungi layanan bantuan atau orang terdekat sekarang.",
    leadEn: "Something you wrote in the last week reads as danger. Put your safety first: contact a help service or someone close now.",
  },
};

const stageOrder: Stage[] = ["everyday", "attention", "support", "urgent"];

function signalText(s: { key: string; count: number; days: number }, w: (id: string, en: string) => string): string {
  switch (s.key) {
    case "crisis_entries":
      return w(`${s.count} tulisan terbaca sebagai bahaya dalam ${s.days} hari`, `${s.count} entries read as danger in ${s.days} days`);
    case "heavy_words":
      return w(`${s.count} tulisan dengan kata-kata berat dalam ${s.days} hari`, `${s.count} entries with heavy words in ${s.days} days`);
    case "heavy_checkins":
      return w(`${s.count} cek perasaan berat dalam ${s.days} hari`, `${s.count} heavy check-ins in ${s.days} days`);
    case "heavy_streak":
      return w(`${s.count} cek perasaan terakhir berturut-turut berat`, `Your last ${s.count} check-ins were all heavy`);
    case "low_energy":
      return w(`${s.count} kali tenaga rendah dalam ${s.days} hari`, `Low energy ${s.count} times in ${s.days} days`);
  }
  return s.key;
}

const helpFor: Record<Stage, { id: string; en: string }[]> = {
  everyday: [
    { id: "Cek perasaan rutin agar petamu tetap akurat", en: "Regular check-ins keep your map accurate" },
    { id: "Bacaan untuk jeda dari sumber resmi", en: "Reading from official sources for a pause" },
  ],
  attention: [
    { id: "Ngobrol dengan orang yang kamu percaya", en: "Talk with someone you trust" },
    { id: "Konselor atau layanan konseling kampus/kantor", en: "A counsellor or campus/workplace counselling" },
  ],
  support: [
    { id: "Psikolog klinis atau Puskesmas untuk pemeriksaan awal", en: "A clinical psychologist or primary care for a first check" },
    { id: "Siapkan ringkasan yang ingin kamu bawa", en: "Prepare a summary you want to bring" },
  ],
  urgent: [
    { id: "Layanan bantuan krisis sekarang", en: "A crisis help service now" },
    { id: "Psikiater atau IGD bila kamu merasa tidak aman", en: "A psychiatrist or emergency department if you don't feel safe" },
  ],
};

export function ConditionMapSection({ policy }: { policy?: Policy }) {
  const w = useWords();
  const { announce } = useAnnounce();
  const consent = useConsent("condition_map", policy);
  const [allow, setAllow] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [map, setMap] = useState<ConditionMap | null>(null);
  const [emergency, setEmergency] = useState(false);
  const [mapError, setMapError] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!consent.granted) {
      setMap(null);
      return;
    }
    let active = true;
    setMapError(null);
    api<ConditionMap>("/condition-map")
      .then((m) => {
        if (!active) return;
        setMap(m);
        if (m.stage === "urgent") {
          const key = "sela.emergency.condition." + m.computedAt.slice(0, 10);
          try {
            if (window.sessionStorage.getItem(key) !== "1") {
              window.sessionStorage.setItem(key, "1");
              setEmergency(true);
            }
          } catch {
            setEmergency(true);
          }
        }
      })
      .catch((e) => {
        if (!active) return;
        // The server no longer holds the consent (withdrawn elsewhere or a
        // new policy version): re-read it so the page offers it again.
        if ((e as { code?: string }).code === "consent_required") consent.reload();
        setMapError(e);
      });
    return () => {
      active = false;
    };
    // consent.reload is stable per purpose
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [consent.granted, attempt]);

  const decide = async (granted: boolean) => {
    setBusy(true);
    setError(null);
    try {
      await consent.set(granted);
      setAllow(false);
      announce("status", granted ? w("Peta kondisi aktif.", "Condition map is on.") : w("Peta kondisi dimatikan.", "Condition map is off."));
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="c-condition" aria-labelledby="condition-title">
      <div className="c-condition-head">
        <h2 id="condition-title">{w("Peta kondisimu", "Your condition map")}</h2>
        <p className="s-hint">
          {w(
            "Dibaca dari catatan, cek perasaan, refleksi, dan percakapan yang kamu simpan. Bukan diagnosis.",
            "Read from the notes, check-ins, reflections and conversations you saved. Not a diagnosis.",
          )}
        </p>
      </div>
      <ErrorNotice error={error ?? consent.error} />
      {consent.loading ? (
        <Loading label={w("Memuat…", "Loading…")} />
      ) : !consent.granted ? (
        <div className="c-condition-consent">
          <p>
            {w(
              "Aktifkan untuk melihat di tahap mana kamu sekarang — dari Sehari-hari sampai Butuh bantuan segera — beserta alasannya, lalu bantuan yang cocok.",
              "Turn it on to see which stage you're at — from Everyday to Needs help now — with its reasons, and the help that fits.",
            )}
          </p>
          {policy && (
            <details>
              <summary>{w("Baca kebijakan peta kondisi", "Read the condition map policy")}</summary>
              {policy.body.map((b, i) => (
                <p className="s-small s-muted" key={i}>
                  {w(b.id, b.en)}
                </p>
              ))}
            </details>
          )}
          <CheckboxRow checked={allow} onChange={setAllow}>
            {w("Aku mengizinkan Sela membaca catatan tersimpanku untuk peta kondisi", "I allow Sela to read my saved entries for the condition map")}
          </CheckboxRow>
          <div className="s-actions">
            <Button variant="primary" disabled={!allow || busy} busy={busy} onClick={() => void decide(true)}>
              {w("Aktifkan peta kondisi", "Turn on the condition map")}
            </Button>
          </div>
        </div>
      ) : !map ? (
        mapError ? (
          <ErrorNotice error={mapError} onRetry={() => setAttempt((n) => n + 1)} />
        ) : (
          <Loading label={w("Membaca catatanmu…", "Reading your entries…")} />
        )
      ) : (
        <>
          <div className={`c-stage c-stage--${map.stage}`}>
            <ol className="c-stage-track" aria-label={w("Tahap", "Stages")}>
              {stageOrder.map((s) => (
                <li key={s} aria-current={s === map.stage ? "step" : undefined} className={s === map.stage ? "is-current" : undefined}>
                  <span>{w(stageText[s].id, stageText[s].en)}</span>
                </li>
              ))}
            </ol>
            <p className="c-stage-label">
              <Icon name={map.stage === "urgent" ? "alert" : map.stage === "everyday" ? "check" : "info"} size={20} />
              <strong>{w(stageText[map.stage].id, stageText[map.stage].en)}</strong>
            </p>
            <p>{w(stageText[map.stage].leadId, stageText[map.stage].leadEn)}</p>
            {map.signals.length > 0 && (
              <ul className="c-stage-signals">
                {map.signals.map((s) => (
                  <li key={s.key}>{signalText(s, w)}</li>
                ))}
              </ul>
            )}
            <p className="s-hint">
              {map.dataLimited
                ? w(
                    "Datanya masih sedikit. Cek perasaan beberapa kali agar petamu lebih tepat.",
                    "There isn't much yet. Check in a few times so your map fits better.",
                  )
                : w(`Dari ${map.dataPoints} catatan tersimpan dalam ${map.windowDays} hari terakhir.`, `From ${map.dataPoints} saved entries in the last ${map.windowDays} days.`)}{" "}
              {w("Kalau terasa tidak cocok, kamu yang paling tahu dirimu.", "If it doesn't feel right, you know yourself best.")}
            </p>
            <div className="s-actions">
              {map.stage === "urgent" ? (
                <Button variant="help" icon="help" onClick={() => setEmergency(true)}>
                  {w("Buka bantuan cepat", "Open quick help")}
                </Button>
              ) : (
                <LinkButton to="/notes/check-in" variant="secondary">
                  {w("Cek perasaan sekarang", "Check in now")}
                </LinkButton>
              )}
              <LinkButton to="/support/plan" variant="secondary">
                {w("Mulai bantuan terencana", "Start planned support")}
              </LinkButton>
              <LinkButton to="/support/summaries/new" variant="quiet">
                {w("Siapkan ringkasan", "Prepare a summary")}
              </LinkButton>
            </div>
          </div>
          <div className="c-stage-help">
            <h3>{w("Bantuan yang cocok untuk tahap ini", "Help that fits this stage")}</h3>
            <ul>
              {helpFor[map.stage].map((h) => (
                <li key={h.en}>{w(h.id, h.en)}</li>
              ))}
            </ul>
          </div>
          <DemoPractitioners stage={map.stage} />
          <p className="c-condition-off">
            <button type="button" className="s-text-button" disabled={busy} onClick={() => void decide(false)}>
              {w("Matikan peta kondisi", "Turn off the condition map")}
            </button>
          </p>
          <EmergencySheet open={emergency} onClose={() => setEmergency(false)} context="map" />
        </>
      )}
    </section>
  );
}

export function DemoPractitioners({ stage }: { stage?: Stage }) {
  const w = useWords();
  const { lang } = useI18n();
  const [list, setList] = useState<Practitioner[] | null>(null);
  useEffect(() => {
    api<{ practitioners: Practitioner[] }>("/care-demo")
      .then((d) => setList(d.practitioners))
      .catch(() => setList([]));
  }, []);
  if (!list || list.length === 0) return null;
  const sorted = stage ? [...list].sort((a, b) => Number(b.stages.includes(stage)) - Number(a.stages.includes(stage))) : list;
  return (
    <section className="c-demo" aria-labelledby="demo-title">
      <div className="c-demo-head">
        <h3 id="demo-title">{w("Tenaga profesional", "Professionals")}</h3>
        <span className="c-demo-badge">{w("Simulasi produk · fiktif", "Product simulation · fictional")}</span>
      </div>
      <p className="s-hint">
        {w(
          "Untuk demo produk. Nama dan layanan di bawah tidak nyata, tidak bisa dihubungi, dan tidak pernah menerima datamu.",
          "For the product demo. The names and services below are not real, cannot be contacted and never receive your data.",
        )}
      </p>
      <ul className="c-demo-list">
        {sorted.map((p) => (
          <li key={p.id} className={stage && p.stages.includes(stage) ? "is-match" : undefined}>
            <div>
              <strong>{p.name}</strong>
              <span className="s-hint">{p.role[lang]} · {p.city}</span>
            </div>
            <p>{p.focus[lang]}</p>
            <p className="s-hint">
              {p.modes.map((m) => (m === "online" ? w("Daring", "Online") : w("Tatap muka", "In person"))).join(" · ")}
              {stage && p.stages.includes(stage) ? " · " + w("Cocok untuk tahapmu", "Fits your stage") : ""}
            </p>
          </li>
        ))}
      </ul>
    </section>
  );
}
