import type { ReactNode } from 'react';
import { vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { RouterProvider } from '../lib/router';
import { CatalogProvider } from '../lib/catalog';
import { SessionProvider } from '../lib/session';
import type { Catalog, Lang, SessionView } from '../lib/types';

// A tiny fetch double for component tests: handlers are keyed by
// "METHOD /path" (path without /api/v2 and without the query string).
export type Reply = { status?: number; data?: unknown; error?: string } | Promise<{ status?: number; data?: unknown; error?: string }>;
export type Handler = (body: unknown, init: RequestInit) => Reply;
export type Call = { method: string; path: string; body: unknown };

export function mockApi(routes: Record<string, Handler>, opts: { honorAbort?: boolean } = {}) {
  // honorAbort=false models a response that was complete before the abort
  // reached it: the caller then gets the data and must ignore it itself.
  const honorAbort = opts.honorAbort ?? true;
  const calls: Call[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init: RequestInit = {}) => {
    const url = new URL(String(input), 'http://localhost');
    const method = (init.method ?? 'GET').toUpperCase();
    const path = url.pathname.replace(/^\/api\/v2/, '');
    const body = typeof init.body === 'string' ? JSON.parse(init.body) : undefined;
    calls.push({ method, path, body });
    const h = routes[`${method} ${path}`];
    if (!h) return new Response(JSON.stringify({ error: { code: 'not_found' } }), { status: 404 });
    const signal = init.signal;
    const r = await h(body, init);
    if (honorAbort && signal?.aborted) throw new DOMException('aborted', 'AbortError');
    const status = r.status ?? 200;
    const payload = r.error ? { error: { code: r.error } } : { data: r.data ?? null };
    return new Response(JSON.stringify(payload), { status, headers: { 'Content-Type': 'application/json' } });
  });
  vi.stubGlobal('fetch', fetchMock);
  return { calls, fetchMock, count: (key: string) => calls.filter((c) => `${c.method} ${c.path}` === key).length };
}

/** deferred returns a promise and its resolver, to answer requests out of order. */
export function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

const text = (s: string) => ({ id: s, en: s });

export function catalogFixture(): Catalog {
  const policy = (v: string) => ({ version: v, title: text('Policy'), body: [text('Body')] });
  return {
    version: 'test',
    demoClock: '2026-10-26T00:00:00Z',
    synthetic: true,
    scenarios: [],
    professionals: [
      {
        id: 'pro-a',
        name: 'Fictional Counsellor A',
        role: text('Counsellor'),
        category: 'counsellor',
        languages: ['en', 'id'],
        formats: ['online'],
        costTier: 'free',
        costLabel: text('Free'),
        availability: ['any'],
        area: text('Online'),
        bio: text('Bio'),
        slots: [{ id: 's1', start: '2026-11-10T03:00:00Z', format: 'online', available: true }],
        plans: ['plan-a'],
      },
    ],
    plans: [{ id: 'plan-a', version: '1', title: text('Plan A'), steps: [text('Step')], attribution: text('Fixture') }],
    checkIns: [],
    clarifyReasons: [{ id: 'c1', label: text('Clarify reason') }],
    declineReasons: [{ id: 'd1', label: text('Decline reason') }],
    nextSteps: [],
    clarifications: { concern: text('?'), duration: text('?'), daily_impact: text('?'), goal: text('?'), support_preference: text('?') },
    help: [],
    helpNotice: text('Catalog help notice'),
    policies: { privacy: policy('p1'), ai_processing: policy('a1'), storage: policy('s1'), sharing: policy('sh1') },
  };
}

export function sessionFixture(over: Partial<SessionView> = {}): SessionView {
  const later = new Date(Date.now() + 3600_000).toISOString();
  return {
    version: 3,
    synthetic: true,
    language: 'en',
    identity: { mode: 'anonymous' },
    createdAt: new Date().toISOString(),
    expiresAt: later,
    idleExpiresAt: later,
    retention: 'session',
    consents: { ai_processing: { granted: true, policyVersion: 'a1', decidedAt: new Date().toISOString(), revision: 1 } },
    policies: { privacy: 'p1', ai_processing: 'a1', storage: 's1', sharing: 'sh1' },
    safety: { level: 'none', interrupted: false },
    stage: 'ledger',
    handoffs: [],
    checkIns: [],
    ops: [],
    ai: { provider: { mode: 'test_double' }, available: true, likelyWarm: true, coldStartEstimateMs: 0, inUse: 0 },
    fixturesVersion: 'test',
    ...over,
  };
}

/** approvedSession has a story, one confirmed manual item and an approval of version 2. */
export function approvedSession(over: Partial<SessionView> = {}): SessionView {
  return sessionFixture({
    story: { version: 1, mode: 'scenario', scenarioId: 'sc', segments: [{ id: 'g1', text: 'I feel on edge.', origin: 'story' }] },
    ledger: {
      version: 2,
      items: [
        {
          id: 'i1', kind: 'feeling', segmentId: 'g1', quote: 'I feel on edge.', start: 0, end: 15, wording: 'I feel on edge.', wordingOrigin: 'quote',
          qualifiers: { negated: false, subject: 'self', time: 'present', certainty: 'certain' }, status: 'confirmed', provenance: 'manual',
        },
      ],
      unknowns: [],
      skipped: [],
      provenance: 'manual',
      approved: { version: 2, hash: 'h', at: new Date().toISOString() },
      reviewed: 1,
    },
    ...over,
  });
}

/** setPath puts the jsdom location on a path before the router reads it. */
export function setPath(path: string) {
  window.history.replaceState(null, '', path);
}

export function Providers({ children, lang = 'en', catalog = catalogFixture(), session = true }: { children: ReactNode; lang?: Lang; catalog?: Catalog | null; session?: boolean }) {
  const inner = session ? <SessionProvider>{children}</SessionProvider> : children;
  return (
    <I18nProvider initial={lang}>
      <RouterProvider>
        <CatalogProvider initial={catalog ?? undefined}>{inner}</CatalogProvider>
      </RouterProvider>
    </I18nProvider>
  );
}
