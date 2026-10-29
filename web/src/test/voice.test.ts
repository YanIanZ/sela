import { describe, expect, it } from "vitest";
import { fromPCM16, resample, toPCM16, VOICE_IN_RATE } from "../v3/voice";

describe("voice audio helpers", () => {
  it("resamples 48 kHz microphone audio to 16 kHz with interpolation", () => {
    const input = new Float32Array(480).map((_, i) => i / 480);
    const out = resample(input, 48000, VOICE_IN_RATE);
    expect(out.length).toBe(160);
    expect(out[0]).toBeCloseTo(0);
    expect(out[80]).toBeCloseTo(240 / 480, 5);
  });

  it("leaves audio already at the target rate untouched", () => {
    const input = new Float32Array([0.1, -0.2]);
    expect(resample(input, 16000, 16000)).toBe(input);
  });

  it("round-trips int16 PCM within one quantisation step and clips out-of-range samples", () => {
    const input = new Float32Array([0, 0.5, -0.5, 1, -1, 2, -2]);
    const back = fromPCM16(toPCM16(input));
    expect(back.length).toBe(input.length);
    for (let i = 0; i < 5; i++) expect(Math.abs(back[i]! - input[i]!)).toBeLessThan(1 / 0x7fff + 1e-6);
    expect(back[5]).toBeCloseTo(1, 3);
    expect(back[6]).toBe(-1);
  });

  it("writes little-endian samples", () => {
    const bytes = new Uint8Array(toPCM16(new Float32Array([-1])));
    expect(Array.from(bytes)).toEqual([0x00, 0x80]);
  });
});
