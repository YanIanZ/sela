## verdict

1. Resolved — `dashboard-tablet.png` and `dashboard-desktop.png` now show the complete synthetic-data / disabled-controls notice beside the rail. Mobile and narrow recaptures retain the notice without rail offset.
2. Resolved — fresh `e2e/results/v3-frontend/07-timeline-desktop.png` and `06-note-detail-desktop.png` visibly replace both unavailable-summary claims with accurate selection/review copy and a “Buka ringkasan” link. Source targets `/support/summaries`.
3. Resolved — `dashboard-dark.png` now visibly renders the dark scheme across the shell, rail, invitation, panels and history. `dashboard-contrast-dark.png` renders the corresponding high-contrast state. `prefs.tsx` includes `systemTick` in the application effect and provider memo dependencies; the supplied preview-results evidence passes all 11 checks, including seven scheme/viewport states, no API requests, blocked product actions, no browser errors and skip-link-first keyboard order. Parent reports a meaningful red-before-green system-preference regression and 65 passing unit tests; this review inspected the code/evidence rather than rerunning that suite.
4. Resolved — `web/DESIGN.md`, `web/.impeccable/design.json` and the surface contract now describe the accepted 15rem desktop rail, context/help/exit header, cobalt invitation and split feature rail, shared surfaces, responsive behavior and development-only preview limits. The original seed and incumbent token world remain recorded; the contract names browser session 59192355 and accepted Today variant 1/density 1/split.

No material regression introduced by this fix batch was observed in the recaptures. Ship covers the four scored fixes, not exhaustive backend approval, deployment, clinical approval or completion of the parent's still-running final Docker checks.

## remaining

Clear.

disposition: ship
