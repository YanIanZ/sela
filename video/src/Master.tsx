// The 180-second master: eight chapters on one timeline (src/timeline.json).
import { AbsoluteFill, Sequence } from "remotion";
import { C } from "./brand/tokens";
import { Fonts } from "./Fonts";
import T from "./timeline.json";
import { Ch01 } from "./scenes/Ch01";
import { Ch02 } from "./scenes/Ch02";
import { Ch03 } from "./scenes/Ch03";
import { Ch04 } from "./scenes/Ch04";
import { Ch05 } from "./scenes/Ch05";
import { Ch06 } from "./scenes/Ch06";
import { Ch07 } from "./scenes/Ch07";
import { Ch08 } from "./scenes/Ch08";

export const SCENES = { ch01: Ch01, ch02: Ch02, ch03: Ch03, ch04: Ch04, ch05: Ch05, ch06: Ch06, ch07: Ch07, ch08: Ch08 } as const;

export function Master() {
  return (
    <AbsoluteFill style={{ background: C.void }}>
      <Fonts />
      {T.chapters.map((ch) => {
        const Scene = SCENES[ch.id as keyof typeof SCENES];
        return (
          <Sequence key={ch.id} from={ch.from} durationInFrames={ch.duration} name={ch.id}>
            <Scene />
          </Sequence>
        );
      })}
    </AbsoluteFill>
  );
}

/** One chapter on its own, for previews and per-scene renders. */
export function Chapter({ id }: { id: keyof typeof SCENES }) {
  const Scene = SCENES[id];
  return (
    <AbsoluteFill style={{ background: C.void }}>
      <Fonts />
      <Scene />
    </AbsoluteFill>
  );
}
