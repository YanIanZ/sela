/*
 * Applies the person's non-sensitive display choices before the first paint
 * so the page never flashes the wrong scheme or text size. Same-origin, no
 * inline script (the Content-Security-Policy allows script-src 'self').
 */
(function () {
  var root = document.documentElement;
  try {
    var lang = window.localStorage.getItem("sela.lang");
    if (lang === "id" || lang === "en") root.lang = lang;
    var raw = window.localStorage.getItem("sela.prefs");
    if (!raw) return;
    var prefs = JSON.parse(raw);
    if (prefs.theme && prefs.theme !== "system")
      root.setAttribute("data-scheme", prefs.theme);
    if (prefs.textScale && prefs.textScale !== "100")
      root.setAttribute("data-text-scale", prefs.textScale);
    if (prefs.spacing && prefs.spacing !== "normal")
      root.setAttribute("data-spacing", prefs.spacing);
    if (prefs.motion && prefs.motion !== "system")
      root.setAttribute("data-motion", prefs.motion);
  } catch (error) {
    /* preferences are a convenience; the page keeps working without them */
  }
})();
