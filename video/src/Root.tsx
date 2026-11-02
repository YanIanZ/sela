import { Composition } from "remotion";
import { FPS, H, W } from "./brand/tokens";
import { Chapter, Master } from "./Master";
import { Teaser, TEASER_FRAMES } from "./Teaser";
import T from "./timeline.json";

export function Root() {
  return (
    <>
      <Composition id="Master" component={Master} durationInFrames={T.durationInFrames} fps={FPS} width={W} height={H} />
      <Composition id="Teaser" component={Teaser} durationInFrames={TEASER_FRAMES} fps={FPS} width={W} height={H} />
      {T.chapters.map((ch) => (
        <Composition key={ch.id} id={ch.id} component={Chapter} defaultProps={{ id: ch.id }} durationInFrames={ch.duration} fps={FPS} width={W} height={H} />
      ))}
    </>
  );
}
