import { staticFile } from "remotion";

/** Self-hosted OFL fonts from web/public/fonts (Lexend 100–900, Hanken Grotesk). */
export function Fonts() {
  return (
    <style>{`
@font-face{font-family:"Lexend";src:url("${staticFile("fonts/lexend-latin.woff2")}") format("woff2");font-weight:100 900;font-display:block}
@font-face{font-family:"Hanken Grotesk";src:url("${staticFile("fonts/hanken-grotesk-latin.woff2")}") format("woff2");font-weight:100 900;font-display:block}
*{box-sizing:border-box;-webkit-font-smoothing:antialiased;text-rendering:geometricPrecision}
`}</style>
  );
}
