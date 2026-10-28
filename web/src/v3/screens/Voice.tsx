import { useEffect, useRef, useState } from "react";
import { useI18n } from "../../i18n";
import { api, type Consent, type Profile } from "../api";
import { helpData } from "../helpdata";
import { Icon } from "../icons";
import { useSession } from "../session";
import { useAnnounce, useWords } from "../shared";
import { Button, ErrorNotice, LinkButton, Loading, Notice, PageHeader } from "../ui";
import { useVoiceRoom, type VoiceError } from "../voice";

const PURPOSE = "ai_provider_aws_voice";

function errorCopy(code: VoiceError, w: (id: string, en: string) => string): string {
  switch (code) {
    case "voice_unavailable":
      return w("Ruang suara belum diaktifkan di server ini.", "The voice room isn't enabled on this server.");
    case "voice_consent_required":
      return w("Izin percakapan suara belum diberikan.", "Voice conversation permission hasn't been given.");
    case "voice_age_band":
      return w("Ruang suara hanya untuk usia 18 tahun ke atas.", "The voice room is for people aged 18 and over only.");
    case "voice_busy":
      return w("Ruang suara sedang terbuka di tab atau perangkat lain.", "The voice room is already open in another tab or device.");
    case "voice_time_limit":
      return w("Sesi suara berakhir setelah 8 menit. Kamu bisa memulai lagi.", "Voice sessions end after 8 minutes. You can start again.");
    case "mic_denied":
      return w("Sela tidak mendapat akses mikrofon. Izinkan mikrofon di browser, lalu coba lagi.", "Sela couldn't use the microphone. Allow it in your browser, then try again.");
    case "voice_backend_unreachable":
    case "voice_backend_ended":
      return w("Layanan suara Amazon tidak menjawab. Coba lagi sebentar lagi, atau tulis di ruang cerita.", "Amazon's voice service didn't respond. Try again shortly, or write in the talk room.");
    default:
      return w("Sambungan suara terputus.", "The voice connection was lost.");
  }
}

export function VoicePage({ user }: { user: Profile }) {
  const w = useWords();
  const { policies } = useSession();
  const { announce } = useAnnounce();
  const room = useVoiceRoom();
  const [consent, setConsent] = useState<Consent | null | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [elapsed, setElapsed] = useState(0);
  const transcriptEnd = useRef<HTMLLIElement>(null);
  const { lang } = useI18n();
  const adult = user.ageBand === "18_plus";
  const policy = policies?.[PURPOSE];

  const loadConsent = () => {
    api<{ consents: Consent[] }>("/me/consents")
      .then(({ consents }) => setConsent(consents.find((c) => c.purpose === PURPOSE) ?? null))
      .catch((e) => {
        setError(e);
        setConsent(null);
      });
  };
  useEffect(loadConsent, []);

  const granted = Boolean(consent?.granted && !consent.outdated);
  const active = room.status === "connecting" || room.status === "listening" || room.status === "speaking";

  useEffect(() => {
    if (!active) return;
    setElapsed(0);
    const t = window.setInterval(() => setElapsed((s) => s + 1), 1000);
    return () => window.clearInterval(t);
  }, [active]);

  useEffect(() => {
    transcriptEnd.current?.scrollIntoView({ block: "nearest" });
  }, [room.turns.length]);

  useEffect(() => {
    if (room.status === "listening") announce("status", w("Sela mendengarkan.", "Sela is listening."));
    if (room.status === "crisis") announce("alert", w("Sesi suara dihentikan. Bantuan manusia ditampilkan.", "The voice session stopped. Human help is shown."));
  }, [room.status, announce, w]);

  const grant = async (value: boolean) => {
    setBusy(true);
    setError(null);
    try {
      const version = consent?.currentPolicyVersion ?? policy?.version;
      if (!version) throw new Error(w("Kebijakan belum dapat dimuat. Coba lagi.", "The policy couldn't be loaded. Try again."));
      await api("/me/consents/" + PURPOSE, { granted: value, policyVersion: version }, "PUT");
      loadConsent();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  const remaining = room.info?.maxSeconds ? Math.max(0, room.info.maxSeconds - elapsed) : 0;
  const clock = `${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, "0")}`;
  const provider = room.info?.testDouble
    ? w("TEST DOUBLE · bukan Amazon Nova Sonic", "TEST DOUBLE · not Amazon Nova Sonic")
    : w("Amazon Nova 2.5 Sonic · AWS (Amerika Serikat)", "Amazon Nova 2.5 Sonic · AWS (United States)");

  return (
    <div className="s-voice">
      <PageHeader
        back={{ to: "/talk", label: w("Cerita", "Talk") }}
        title={w("Ruang suara", "Voice room")}
        lead={w(
          "Bicara saja, Sela menjawab dengan suara. Eksperimental: bahasa Indonesia belum didukung resmi oleh Amazon, jadi ucapanmu bisa salah dikenali.",
          "Just speak; Sela answers with a voice. Experimental: Amazon doesn't officially support Indonesian yet, so your speech may be misheard.",
        )}
      />
      <ErrorNotice error={error} />

      {!adult ? (
        <Notice tone="info" title={w("Hanya untuk usia 18 tahun ke atas", "For people aged 18 and over only")}>
          <p>{w("Kamu tetap bisa bercerita dengan menulis di ruang cerita.", "You can still talk by writing in the talk room.")}</p>
          <div className="s-actions">
            <LinkButton to="/talk/new" variant="secondary">{w("Buka ruang cerita", "Open the talk room")}</LinkButton>
          </div>
        </Notice>
      ) : consent === undefined ? (
        <Loading label={w("Memeriksa izin…", "Checking permission…")} />
      ) : !granted ? (
        <section className="s-voice-consent" aria-labelledby="voice-consent-title">
          <h2 id="voice-consent-title">{policy?.title[lang] ?? w("Percakapan suara", "Voice conversation")}</h2>
          {(policy?.body ?? []).map((p, i) => (
            <p key={i}>{p[lang]}</p>
          ))}
          <div className="s-actions">
            <Button variant="primary" busy={busy} onClick={() => void grant(true)}>
              {w("Izinkan percakapan suara", "Allow voice conversation")}
            </Button>
            <LinkButton to="/talk" variant="quiet">{w("Tidak sekarang", "Not now")}</LinkButton>
          </div>
        </section>
      ) : (
        <>
          {room.status === "crisis" && <VoiceCrisis />}
          <section className="s-voice-stage" aria-labelledby="voice-stage-title" hidden={room.status === "crisis"}>
            <h2 id="voice-stage-title" className="s-sr-only">{w("Kendali suara", "Voice controls")}</h2>
            <p className="s-voice-provider">
              <Icon name="ai" size={16} />
              <span>{provider}</span>
              <span className="s-voice-flag">{w("Eksperimental", "Experimental")}</span>
            </p>
            <div className={"s-voice-orb s-voice-orb--" + room.status} aria-hidden="true">
              <span />
            </div>
            <p className="s-voice-status" role="status">
              {room.status === "idle" && w("Tekan mulai, lalu bicara dengan tenang.", "Press start, then speak calmly.")}
              {room.status === "connecting" && w("Menyambungkan…", "Connecting…")}
              {room.status === "listening" && (room.muted ? w("Mikrofon dibisukan.", "Microphone muted.") : w("Sela mendengarkan…", "Sela is listening…"))}
              {room.status === "speaking" && w("Sela sedang menjawab. Kamu boleh menyela.", "Sela is answering. You can interrupt.")}
              {room.status === "ended" && w("Sesi suara selesai. Tidak ada yang disimpan.", "The voice session ended. Nothing was saved.")}
              {room.status === "error" && room.error && errorCopy(room.error, w)}
            </p>
            <div className="s-actions s-voice-controls">
              {active ? (
                <>
                  <Button variant="primary" icon="stop" onClick={room.stop}>{w("Akhiri", "End")}</Button>
                  <Button variant="secondary" icon="mic" onClick={room.toggleMute} aria-pressed={room.muted}>
                    {room.muted ? w("Nyalakan mikrofon", "Unmute") : w("Bisukan", "Mute")}
                  </Button>
                  {remaining > 0 && <span className="s-voice-clock">{w("Sisa ", "Left ")}{clock}</span>}
                </>
              ) : room.status !== "crisis" ? (
                <Button variant="primary" icon="mic" onClick={() => void room.start()}>
                  {room.status === "idle" ? w("Mulai bicara", "Start speaking") : w("Mulai lagi", "Start again")}
                </Button>
              ) : null}
            </div>
            <p className="s-hint">
              {w(
                "Pakai earphone supaya suara Sela tidak tertangkap mikrofon. Percakapan suara tidak disimpan.",
                "Use earphones so Sela's voice isn't picked up by the microphone. Voice conversations aren't saved.",
              )}
            </p>
          </section>

          {room.withheld > 0 && (
            <Notice tone="info" title={w("Jawaban ditahan", "Answer withheld")}>
              <p>
                {w(
                  "Satu atau lebih jawaban tidak diputar karena tidak lolos pemeriksaan keselamatan Sela (misalnya terdengar seperti diagnosis atau saran obat).",
                  "One or more answers weren't played because they didn't pass Sela's safety checks (for example, they sounded like a diagnosis or medication advice).",
                )}
              </p>
            </Notice>
          )}

          <section className="s-voice-transcript" aria-labelledby="voice-transcript-title">
            <h2 id="voice-transcript-title">{w("Transkrip", "Transcript")}</h2>
            {room.turns.length === 0 ? (
              <p className="s-hint">{w("Teks percakapan muncul di sini. Hanya ada di tab ini.", "The conversation text appears here. It exists in this tab only.")}</p>
            ) : (
              <ol>
                {room.turns.map((t) => (
                  <li key={t.id} className={"s-voice-turn s-voice-turn--" + t.who}>
                    <span className="s-voice-who">
                      {t.who === "you"
                        ? w("Kamu · dikenali oleh Sonic, bisa keliru", "You · recognised by Sonic, may be wrong")
                        : w("Sela · dijawab oleh ", "Sela · answered by ") + provider}
                    </span>
                    <p>{t.text}</p>
                  </li>
                ))}
                <li ref={transcriptEnd} aria-hidden="true" className="s-voice-end" />
              </ol>
            )}
          </section>

          <div className="s-actions">
            <Button variant="quiet" onClick={() => void grant(false)} disabled={active || busy}>
              {w("Cabut izin percakapan suara", "Withdraw voice permission")}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}

function VoiceCrisis() {
  const w = useWords();
  const { lang } = useI18n();
  const service = helpData.help[0];
  return (
    <section className="s-crisis-panel" aria-labelledby="voice-crisis-title">
      <h2 id="voice-crisis-title">{w("Sesi suara dihentikan. Kamu tidak sendirian.", "The voice session stopped. You're not alone.")}</h2>
      <p>
        {w(
          "Ada kata-kata yang terdengar sangat berat, jadi Sela menghentikan percakapan suara dan menunjukkan bantuan manusia. Kalau kamu dalam bahaya sekarang, hubungi layanan darurat setempat atau orang terdekat.",
          "Some words sounded very heavy, so Sela stopped the voice conversation and is showing human help. If you're in danger right now, contact local emergency services or someone nearby.",
        )}
      </p>
      {service && (
        <p>
          <strong>{service.name}</strong> · {service.description[lang]}
        </p>
      )}
      <div className="s-actions">
        <LinkButton to="/help?from=talk" variant="help" icon="help">{w("Lihat bantuan sekarang", "See help now")}</LinkButton>
        <LinkButton to="/talk" variant="quiet">{w("Kembali ke Cerita", "Back to Talk")}</LinkButton>
      </div>
    </section>
  );
}
