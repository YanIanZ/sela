import { useCallback, useEffect, useRef, useState } from "react";

/*
 * Dictation over Sela's server-side transcription relay
 * (GET /api/v3/asr/realtime, SPEC C25, backlog S-025). The browser
 * captures raw PCM audio (mono 16 kHz int16, little-endian) and streams
 * it to Sela; Sela relays it to Amazon Nova Sonic with credentials held
 * server-side (a legacy Meta relay remains behind SELA_ASR_MODE=meta).
 * Credentials never reach the browser. The transcript stays editable:
 * partials preview live, the final text is appended to the composer,
 * and nothing is ever auto-submitted.
 */

export const ASR_SAMPLE_RATE = 16000;
/** Frontend stops recording by itself after this long. */
const MAX_RECORD_MS = 180_000;
/** Wait for the provider's final answer after endStream before giving up. */
const FINAL_WAIT_MS = 20_000;

/** Float32 [-1, 1] to little-endian int16 PCM bytes. Pure: unit-tested. */
export function floatToPCM16Bytes(input: Float32Array): ArrayBuffer {
  const bytes = new ArrayBuffer(input.length * 2);
  const view = new DataView(bytes);
  for (let i = 0; i < input.length; i++) {
    const v = Math.max(-1, Math.min(1, input[i] as number));
    view.setInt16(i * 2, v < 0 ? Math.round(v * 0x8000) : Math.round(v * 0x7fff), true);
  }
  return bytes;
}

/** Downsample one mono channel to 16 kHz (nearest pick). Pure: unit-tested. */
export function downsampleTo16k(input: Float32Array, fromRate: number): Float32Array {
  if (fromRate === ASR_SAMPLE_RATE) return input;
  const ratio = fromRate / ASR_SAMPLE_RATE;
  const len = Math.floor(input.length / ratio);
  const out = new Float32Array(len);
  for (let i = 0; i < len; i++) out[i] = input[Math.floor(i * ratio)] as number;
  return out;
}

export function asrSocketURL(): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return proto + "//" + window.location.host + "/api/v3/asr/realtime";
}

export type DictationError =
  | "mic_denied"
  | "asr_unavailable"
  | "asr_consent_required"
  | "asr_failed";

type Session = {
  ws: WebSocket;
  stream: MediaStream;
  audio: AudioContext;
  node: ScriptProcessorNode;
  source: MediaStreamAudioSourceNode;
  sink: GainNode;
  finishing: boolean;
  onPageHide: () => void;
};

function closeSession(s: Session): void {
  window.removeEventListener("pagehide", s.onPageHide);
  for (const track of s.stream.getTracks()) {
    try {
      track.stop();
    } catch {
      /* already gone */
    }
  }
  try {
    s.node.disconnect();
    s.source.disconnect();
    s.sink.disconnect();
  } catch {
    /* already gone */
  }
  try {
    void s.audio.close();
  } catch {
    /* already gone */
  }
  if (s.ws.readyState === WebSocket.OPEN || s.ws.readyState === WebSocket.CONNECTING) {
    try {
      s.ws.close();
    } catch {
      /* already gone */
    }
  }
}

export function useDictation(
  appendFinal: (final: string) => void,
): {
  dictating: boolean;
  livePartial: string;
  errorCode: DictationError | null;
  clearError: () => void;
  toggle: () => void;
} {
  const [dictating, setDictating] = useState(false);
  const [livePartial, setLivePartial] = useState("");
  const [errorCode, setErrorCode] = useState<DictationError | null>(null);
  const session = useRef<Session | null>(null);
  const appendRef = useRef(appendFinal);
  appendRef.current = appendFinal;

  const shutdown = useCallback((s: Session | null) => {
    if (s) {
      if (session.current === s) session.current = null;
      closeSession(s);
    }
    setLivePartial("");
    setDictating(false);
  }, []);

  // Unmount safety: stop the mic; the proxy session ends with the socket.
  useEffect(() => {
    return () => {
      const s = session.current;
      session.current = null;
      if (s) closeSession(s);
    };
  }, []);

  const fail = useCallback(
    (s: Session, code: DictationError) => {
      shutdown(s);
      setErrorCode(code);
    },
    [shutdown],
  );

  const finishSession = useCallback(
    (s: Session, final: string) => {
      shutdown(s);
      const text = final.trim();
      if (text) appendRef.current(text);
    },
    [shutdown],
  );

  const start = useCallback(async () => {
    if (session.current) return;
    setErrorCode(null);
    setLivePartial("");
    let stream: MediaStream;
    try {
      stream = await window.navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true },
      });
    } catch {
      setErrorCode("mic_denied");
      return;
    }
    let ws: WebSocket;
    try {
      ws = new WebSocket(asrSocketURL());
    } catch {
      for (const track of stream.getTracks()) track.stop();
      setErrorCode("asr_failed");
      return;
    }
    const audio = new AudioContext();
    const source = audio.createMediaStreamSource(stream);
    const node = audio.createScriptProcessor(4096, 1, 1);
    const sink = audio.createGain();
    sink.gain.value = 0;
    const inRate = audio.sampleRate;
    node.onaudioprocess = (e) => {
      if (ws.readyState !== WebSocket.OPEN) return;
      const ch = e.inputBuffer.getChannelData(0);
      const resampled = downsampleTo16k(
        ch instanceof Float32Array ? ch : new Float32Array(ch),
        inRate,
      );
      const bytes = floatToPCM16Bytes(resampled);
      if (bytes.byteLength > 0) ws.send(bytes);
    };
    source.connect(node);
    node.connect(sink);
    sink.connect(audio.destination);

    const s: Session = { ws, stream, audio, node, source, sink, finishing: false, onPageHide: () => {} };
    session.current = s;
    setDictating(true);

    const autoStop = window.setTimeout(() => {
      if (session.current === s && !s.finishing) {
        s.finishing = true;
        try {
          ws.send(JSON.stringify({ type: "endStream" }));
        } catch {
          fail(s, "asr_failed");
        }
      }
    }, MAX_RECORD_MS);

    ws.onmessage = (e) => {
      if (session.current !== s || typeof e.data !== "string") return;
      let ev: { type?: string; transcript?: string; final?: boolean; message?: string };
      try {
        ev = JSON.parse(e.data) as typeof ev;
      } catch {
        return;
      }
      if (ev.type === "error") {
        window.clearTimeout(autoStop);
        fail(s, ev.message === "asr_consent_required" ? "asr_consent_required" : "asr_failed");
        return;
      }
      if (ev.type !== "transcript" || typeof ev.transcript !== "string") return;
      if (ev.final) {
        window.clearTimeout(autoStop);
        finishSession(s, ev.transcript);
      } else {
        setLivePartial(ev.transcript);
      }
    };
    const onDead = () => {
      if (session.current !== s) return;
      window.clearTimeout(autoStop);
      // The server closed before a final (quota, idle, 503/403 at
      // upgrade surfaces as a failed open): keep the typed text.
      fail(s, "asr_failed");
    };
    ws.onerror = onDead;
    ws.onclose = (e) => {
      if (session.current !== s) return;
      window.clearTimeout(autoStop);
      if (!s.finishing && e.code === 4403) {
        fail(s, "asr_consent_required");
        return;
      }
      if (!s.finishing && e.code === 4413) {
        fail(s, "asr_unavailable");
        return;
      }
      if (!s.finishing) fail(s, "asr_failed");
      // While finishing, the proxy closes after the final; if it never
      // came, just release the mic and keep the typed text.
      shutdown(s);
    };
    // A page hide or crash should release the microphone promptly.
    s.onPageHide = () => {
      if (session.current !== s) return;
      shutdown(s);
    };
    window.addEventListener("pagehide", s.onPageHide);
  }, [fail, finishSession, shutdown]);

  const stop = useCallback(() => {
    const s = session.current;
    if (!s || s.finishing) return;
    s.finishing = true;
    if (s.ws.readyState === WebSocket.OPEN) {
      try {
        s.ws.send(JSON.stringify({ type: "endStream" }));
      } catch {
        shutdown(s);
        return;
      }
      // If no final arrives, release the mic anyway.
      window.setTimeout(() => {
        if (session.current === s) shutdown(s);
      }, FINAL_WAIT_MS);
    } else {
      shutdown(s);
    }
  }, [shutdown]);

  const toggle = useCallback(() => {
    if (session.current) stop();
    else void start();
  }, [start, stop]);

  const clearError = useCallback(() => setErrorCode(null), []);

  return { dictating, livePartial, errorCode, clearError, toggle };
}
