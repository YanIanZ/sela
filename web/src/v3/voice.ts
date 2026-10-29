import { useCallback, useEffect, useRef, useState } from "react";

/*
 * Voice room over Sela's server-side Amazon Nova 2.5 Sonic relay
 * (GET /api/v3/voice/realtime, SPEC C27). The browser sends microphone PCM
 * (mono 16 kHz int16) and plays assistant PCM (mono 24 kHz int16). The IAM
 * credentials stay on the server. Sela's server holds every assistant audio
 * block until its text has passed the crisis router and output guards, and
 * ends the session if the person's words signal a crisis.
 */

export const VOICE_IN_RATE = 16000;
export const VOICE_OUT_RATE = 24000;

/** Linear resample of one mono channel. Pure: unit-tested. */
export function resample(input: Float32Array, fromRate: number, toRate: number): Float32Array {
  if (fromRate === toRate) return input;
  const ratio = fromRate / toRate;
  const len = Math.floor(input.length / ratio);
  const out = new Float32Array(len);
  for (let i = 0; i < len; i++) {
    const pos = i * ratio;
    const j = Math.floor(pos);
    const f = pos - j;
    const a = input[j] ?? 0;
    const b = input[j + 1] ?? a;
    out[i] = a + (b - a) * f;
  }
  return out;
}

/** Float32 [-1, 1] to little-endian int16 PCM. Pure: unit-tested. */
export function toPCM16(input: Float32Array): ArrayBuffer {
  const out = new Int16Array(input.length);
  for (let i = 0; i < input.length; i++) {
    const v = Math.max(-1, Math.min(1, input[i] ?? 0));
    out[i] = v < 0 ? Math.round(v * 0x8000) : Math.round(v * 0x7fff);
  }
  return out.buffer;
}

/** Little-endian int16 PCM to Float32 [-1, 1]. Pure: unit-tested. */
export function fromPCM16(buf: ArrayBuffer): Float32Array<ArrayBuffer> {
  const view = new DataView(buf);
  const n = Math.floor(buf.byteLength / 2);
  const out = new Float32Array(new ArrayBuffer(n * 4));
  for (let i = 0; i < n; i++) out[i] = view.getInt16(2 * i, true) / 0x8000;
  return out;
}

export type VoiceStatus =
  | "idle"
  | "connecting"
  | "listening"
  | "speaking"
  | "ended"
  | "crisis"
  | "error";

export type VoiceTurn = { id: number; who: "you" | "sela"; text: string; final: boolean };

/** Server refusal and failure codes, mapped by the screen to copy. */
export type VoiceError =
  | "voice_unavailable"
  | "voice_consent_required"
  | "voice_age_band"
  | "voice_busy"
  | "voice_backend_unreachable"
  | "voice_backend_ended"
  | "voice_time_limit"
  | "mic_denied"
  | "connection_lost";

export type VoiceInfo = { model: string; testDouble: boolean; maxSeconds: number };

function socketURL(): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return proto + "//" + window.location.host + "/api/v3/voice/realtime";
}

type Live = {
  ws: WebSocket;
  stream: MediaStream;
  ctx: AudioContext;
  node: ScriptProcessorNode;
  source: MediaStreamAudioSourceNode;
  playing: AudioBufferSourceNode[];
  nextAt: number;
};

export function useVoiceRoom() {
  const [status, setStatus] = useState<VoiceStatus>("idle");
  const [turns, setTurns] = useState<VoiceTurn[]>([]);
  const [error, setError] = useState<VoiceError | null>(null);
  const [info, setInfo] = useState<VoiceInfo | null>(null);
  const [withheld, setWithheld] = useState(0);
  const [muted, setMuted] = useState(false);
  const live = useRef<Live | null>(null);
  const mutedRef = useRef(false);
  const nextId = useRef(1);
  const endedByUs = useRef(false);

  const stopPlayback = useCallback(() => {
    const l = live.current;
    if (!l) return;
    for (const s of l.playing) {
      try {
        s.stop();
      } catch {
        // already stopped
      }
    }
    l.playing = [];
    l.nextAt = 0;
  }, []);

  const teardown = useCallback(() => {
    const l = live.current;
    live.current = null;
    if (!l) return;
    for (const s of l.playing) {
      try {
        s.stop();
      } catch {
        // already stopped
      }
    }
    l.node.onaudioprocess = null;
    l.node.disconnect();
    l.source.disconnect();
    l.stream.getTracks().forEach((t) => t.stop());
    void l.ctx.close();
    if (l.ws.readyState === WebSocket.OPEN || l.ws.readyState === WebSocket.CONNECTING) l.ws.close();
  }, []);

  useEffect(() => teardown, [teardown]);

  const addTurn = useCallback((who: VoiceTurn["who"], text: string, final: boolean) => {
    if (!text.trim()) return;
    setTurns((t) => {
      const last = t[t.length - 1];
      // Speculative Sela text is replaced by its final version.
      if (who === "sela" && last && last.who === "sela" && !last.final) {
        return [...t.slice(0, -1), { ...last, text, final }];
      }
      return [...t, { id: nextId.current++, who, text, final }];
    });
  }, []);

  const play = useCallback((buf: ArrayBuffer) => {
    const l = live.current;
    if (!l) return;
    const pcm = fromPCM16(buf);
    if (pcm.length === 0) return;
    const audio = l.ctx.createBuffer(1, pcm.length, VOICE_OUT_RATE);
    audio.copyToChannel(pcm, 0);
    const src = l.ctx.createBufferSource();
    src.buffer = audio;
    src.connect(l.ctx.destination);
    const at = Math.max(l.ctx.currentTime + 0.03, l.nextAt);
    src.start(at);
    l.nextAt = at + audio.duration;
    l.playing.push(src);
    setStatus("speaking");
    src.onended = () => {
      const cur = live.current;
      if (!cur) return;
      cur.playing = cur.playing.filter((s) => s !== src);
      if (cur.playing.length === 0) setStatus((s) => (s === "speaking" ? "listening" : s));
    };
  }, []);

  const start = useCallback(async () => {
    if (live.current) return;
    setError(null);
    setTurns([]);
    setWithheld(0);
    setStatus("connecting");
    endedByUs.current = false;
    let stream: MediaStream;
    try {
      stream = await window.navigator.mediaDevices.getUserMedia({
        audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      });
    } catch {
      setError("mic_denied");
      setStatus("error");
      return;
    }
    const ctx = new AudioContext();
    const source = ctx.createMediaStreamSource(stream);
    const node = ctx.createScriptProcessor(2048, 1, 1);
    const ws = new WebSocket(socketURL());
    ws.binaryType = "arraybuffer";
    const l: Live = { ws, stream, ctx, node, source, playing: [], nextAt: 0 };
    live.current = l;
    let lastError: VoiceError | null = null;

    node.onaudioprocess = (e) => {
      if (ws.readyState !== WebSocket.OPEN) return;
      const input = e.inputBuffer.getChannelData(0);
      // A muted microphone still sends silence: Sonic expects a stream.
      const frame = mutedRef.current ? new Float32Array(input.length) : input;
      ws.send(toPCM16(resample(frame, ctx.sampleRate, VOICE_IN_RATE)));
    };
    // ScriptProcessor only runs while connected; its output stays silent.
    const sink = ctx.createGain();
    sink.gain.value = 0;
    source.connect(node);
    node.connect(sink);
    sink.connect(ctx.destination);

    ws.onmessage = (ev) => {
      if (ev.data instanceof ArrayBuffer) {
        play(ev.data);
        return;
      }
      let m: { type?: string; text?: string; final?: boolean; message?: string; model?: string; testDouble?: boolean; maxSeconds?: number };
      try {
        m = JSON.parse(String(ev.data));
      } catch {
        return;
      }
      switch (m.type) {
        case "ready":
          setInfo({ model: m.model ?? "", testDouble: Boolean(m.testDouble), maxSeconds: m.maxSeconds ?? 0 });
          setStatus("listening");
          break;
        case "transcript":
          addTurn("you", m.text ?? "", true);
          break;
        case "reply":
          addTurn("sela", m.text ?? "", Boolean(m.final));
          break;
        case "interrupted":
          stopPlayback();
          setStatus("listening");
          break;
        case "withheld":
          stopPlayback();
          setWithheld((n) => n + 1);
          break;
        case "crisis":
          stopPlayback();
          setStatus("crisis");
          break;
        case "error":
          lastError = (m.message as VoiceError) ?? "connection_lost";
          break;
      }
    };
    ws.onclose = (ev) => {
      const wasCrisis = ev.code === 4409;
      teardown();
      if (wasCrisis) {
        setStatus("crisis");
        return;
      }
      if (endedByUs.current || ev.code === 1000) {
        setStatus("ended");
        return;
      }
      setError(lastError ?? "connection_lost");
      setStatus("error");
    };
  }, [addTurn, play, stopPlayback, teardown]);

  const stop = useCallback(() => {
    const l = live.current;
    if (!l) return;
    live.current = null;
    endedByUs.current = true;
    if (l.ws.readyState === WebSocket.OPEN) l.ws.send(JSON.stringify({ type: "end" }));
    stopPlayback();
    window.setTimeout(() => {
      for (const s of l.playing) { try { s.stop(); } catch { /* already stopped */ } }
      l.node.onaudioprocess = null;
      l.node.disconnect();
      l.source.disconnect();
      l.stream.getTracks().forEach((t) => t.stop());
      void l.ctx.close();
      if (l.ws.readyState === WebSocket.OPEN || l.ws.readyState === WebSocket.CONNECTING) l.ws.close();
    }, 400);
    setStatus("ended");
  }, [stopPlayback]);

  const toggleMute = useCallback(() => {
    mutedRef.current = !mutedRef.current;
    setMuted(mutedRef.current);
  }, []);

  return { status, turns, error, info, withheld, muted, start, stop, toggleMute };
}
