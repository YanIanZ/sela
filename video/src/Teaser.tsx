// 30-second typography teaser, cut from the master (src/timeline.json "teaser").
import { AbsoluteFill, Sequence, useCurrentFrame } from "remotion";
import { C } from "./brand/tokens";
import { Fonts } from "./Fonts";
import { Master } from "./Master";
import { EASE, prog } from "./motion";
import T from "./timeline.json";

export const TEASER_FRAMES = T.teaser.reduce((n, s) => n + (s.to - s.from), 0);

function Blink({ at }: { at: number }) {
  // A 6-frame dip across each cut so jumps read as intentional.
  const f = useCurrentFrame();
  const a = Math.max(0, 1 - Math.abs(f - at) / 4);
  return a > 0 ? <AbsoluteFill style={{ background: C.void, opacity: a * prog(f, at - 4, 4, EASE.soft) + a * 0.4 }} /> : null;
}

export function Teaser() {
  let cum = 0;
  const cuts: number[] = [];
  const segs = T.teaser.map((s, i) => {
    const from = cum;
    const len = s.to - s.from;
    cum += len;
    if (i > 0) cuts.push(from);
    return (
      <Sequence key={i} from={from} durationInFrames={len}>
        <Sequence from={-s.from}>
          <Master />
        </Sequence>
      </Sequence>
    );
  });
  return (
    <AbsoluteFill style={{ background: C.void }}>
      <Fonts />
      {segs}
      {cuts.map((c) => (
        <Blink key={c} at={c} />
      ))}
    </AbsoluteFill>
  );
}
