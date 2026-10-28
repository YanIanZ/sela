import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { act, render } from "@testing-library/react";
import {
  ASR_SAMPLE_RATE,
  asrSocketURL,
  downsampleTo24k,
  floatToPCM16Bytes,
  useMetaDictation,
} from "../v3/asr";

describe("asr pcm helpers", () => {
  it("converts float samples to int16 PCM", () => {
    const buf = floatToPCM16Bytes(new Float32Array([0, 1, -1, 0.5]));
    expect(new Int16Array(buf)).toEqual(new Int16Array([0, 32767, -32768, 16384]));
  });

  it("clamps out-of-range samples", () => {
    const buf = floatToPCM16Bytes(new Float32Array([2, -2]));
    expect(new Int16Array(buf)).toEqual(new Int16Array([32767, -32768]));
  });

  it("passes 24 kHz audio through untouched", () => {
    const input = new Float32Array([0.1, 0.2, 0.3]);
    expect(downsampleTo24k(input, ASR_SAMPLE_RATE)).toBe(input);
  });

  it("downsamples 48 kHz to 24 kHz by picking every other sample", () => {
    const input = new Float32Array([0, 0.9, 0.25, 0.9, 0.5, 0.9]);
    expect(Array.from(downsampleTo24k(input, 48000))).toEqual([0, 0.25, 0.5]);
  });

  it("builds a same-origin socket URL", () => {
    expect(asrSocketURL().endsWith("/api/v3/asr/realtime")).toBe(true);
  });
});

// Minimal DOM/mic stubs so the dictation hook can run under jsdom.
function micStubs() {
  const track = { stop: vi.fn() };
  const stream = { getTracks: () => [track] };
  const getUserMedia = vi.fn().mockResolvedValue(stream);
  Object.defineProperty(window.navigator, "mediaDevices", {
    configurable: true,
    value: { getUserMedia },
  });
}

class MockWS {
  static OPEN = 1;
  static CONNECTING = 0;
  static last: MockWS | null = null;
  readyState = MockWS.CONNECTING;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onerror: (() => void) | null = null;
  onclose: ((e: CloseEvent) => void) | null = null;
  sent: (string | ArrayBuffer)[] = [];
  constructor() {
    MockWS.last = this;
  }
  send(data: string | ArrayBuffer) {
    this.sent.push(data);
  }
  close(code = 1005) {
    this.readyState = 3;
    this.onclose?.({ code, wasClean: false } as CloseEvent);
  }
  open() {
    this.readyState = MockWS.OPEN;
  }
  emit(data: unknown) {
    this.onmessage?.({ data } as MessageEvent);
  }
}

class MockAudioContext {
  sampleRate = 48000;
  createMediaStreamSource() {
    return { connect: vi.fn() };
  }
  createScriptProcessor() {
    return { onaudioprocess: null as null | ((e: unknown) => void), connect: vi.fn(), disconnect: vi.fn() };
  }
  createGain() {
    return { gain: { value: 0 }, connect: vi.fn(), disconnect: vi.fn() };
  }
  close() {}
  get destination() {
    return {};
  }
}

function renderHook<T>(fn: () => T): { result: { current: T } } {
  const ref = { current: undefined as unknown as T };
  function Probe() {
    ref.current = fn();
    return null;
  }
  render(<Probe />);
  return { result: ref };
}

describe("useMetaDictation", () => {
  beforeEach(() => {
    micStubs();
    vi.stubGlobal("WebSocket", MockWS);
    vi.stubGlobal("AudioContext", MockAudioContext);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("starts the mic on toggle, sends PCM to the websocket and appends the final transcript", async () => {
    const appended: string[] = [];
    const { result } = renderHook(() => useMetaDictation((final) => appended.push(final)));
    expect(result.current.dictating).toBe(false);
    await act(async () => {
      result.current.toggle();
      await Promise.resolve();
    });
    expect(result.current.dictating).toBe(true);
    const ws = MockWS.last!;
    expect(ws).toBeDefined();
    await act(async () => {
      ws.open();
      ws.emit(JSON.stringify({ type: "transcript", transcript: "halo", final: false }));
    });
    expect(result.current.livePartial).toBe("halo");
    await act(async () => {
      ws.emit(JSON.stringify({ type: "transcript", transcript: "halo dunia", final: true }));
    });
    expect(appended).toEqual(["halo dunia"]);
    expect(result.current.dictating).toBe(false);
    expect(result.current.livePartial).toBe("");
  });

  it("maps close code 4403 to the consent error", async () => {
    const { result } = renderHook(() => useMetaDictation(() => {}));
    await act(async () => {
      result.current.toggle();
      await Promise.resolve();
    });
    const ws = MockWS.last!;
    await act(async () => {
      ws.open();
      ws.close(4403);
    });
    expect(result.current.errorCode).toBe("asr_consent_required");
    expect(result.current.dictating).toBe(false);
  });

  it("maps close code 4413 to the unavailable error", async () => {
    const { result } = renderHook(() => useMetaDictation(() => {}));
    await act(async () => {
      result.current.toggle();
      await Promise.resolve();
    });
    const ws = MockWS.last!;
    await act(async () => {
      ws.open();
      ws.close(4413);
    });
    expect(result.current.errorCode).toBe("asr_unavailable");
    expect(result.current.dictating).toBe(false);
  });

  it("maps mic denial to mic_denied", async () => {
    Object.defineProperty(window.navigator, "mediaDevices", {
      configurable: true,
      value: { getUserMedia: vi.fn().mockRejectedValue(new Error("no")) },
    });
    const { result } = renderHook(() => useMetaDictation(() => {}));
    await act(async () => {
      result.current.toggle();
      await Promise.resolve();
    });
    expect(result.current.errorCode).toBe("mic_denied");
    expect(result.current.dictating).toBe(false);
  });
});
