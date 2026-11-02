/*
 * Browser entry for the offline renderer (scripts/render.mjs). It mounts the
 * Remotion root, collects <Composition>s and exposes window.__sela to draw any
 * frame synchronously. With real Remotion this file is unused.
 */
import { createRoot } from "react-dom/client";
import { flushSync } from "react-dom";
import { FrameRoot, getCompositions, getRoot, setStaticBase } from "remotion";
import "./index";

declare global {
  interface Window {
    __sela: { ready: Promise<void>; compositions: () => Array<{ id: string; durationInFrames: number; fps: number; width: number; height: number }>; draw: (id: string, frame: number) => Promise<void> };
  }
}

setStaticBase("/");
const Root = getRoot();
const host = document.getElementById("root")!;
const root = createRoot(host);
flushSync(() => root.render(Root ? <Root /> : null)); // registers compositions

async function preload() {
  await document.fonts.ready;
  await Promise.all(["Lexend", "Hanken Grotesk"].flatMap((f) => [400, 600, 700, 800].map((w) => document.fonts.load(`${w} 40px "${f}"`))));
  const manifest = (await (await fetch("/footage/footage.json")).json()) as Record<string, { file: string }>;
  await Promise.all(
    Object.values(manifest).map(
      (m) =>
        new Promise<void>((res) => {
          const img = new Image();
          img.onload = () => img.decode().then(() => res(), () => res());
          img.onerror = () => res();
          img.src = "/" + m.file;
        }),
    ),
  );
}

window.__sela = {
  ready: preload(),
  compositions: () => getCompositions().map(({ id, durationInFrames, fps, width, height }) => ({ id, durationInFrames, fps, width, height })),
  draw: async (id, frame) => {
    const comp = getCompositions().find((c) => c.id === id);
    if (!comp) throw new Error("no composition " + id);
    const C = comp.component;
    flushSync(() =>
      root.render(
        <div style={{ position: "relative", width: comp.width, height: comp.height, overflow: "hidden" }}>
          <FrameRoot frame={frame} config={{ fps: comp.fps, width: comp.width, height: comp.height, durationInFrames: comp.durationInFrames, id }}>
            <C {...(comp.defaultProps ?? {})} />
          </FrameRoot>
        </div>,
      ),
    );
    // Wait for any image that mounted this frame to decode.
    const imgs = Array.from(host.querySelectorAll("img"));
    await Promise.all(imgs.map((i) => (i.complete ? i.decode().catch(() => {}) : new Promise((r) => (i.onload = i.onerror = r)))));
  },
};
