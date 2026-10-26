import type { Lang } from './types';

// ApiError carries the server's stable error code; the UI localizes by code.
export class ApiError extends Error {
  constructor(
    public readonly code: string,
    public readonly status: number,
    public readonly field?: string,
  ) {
    super(code);
  }
}

let currentLang: Lang = 'id';
export function setApiLanguage(lang: Lang) {
  currentLang = lang;
}

type Options = { method?: string; body?: unknown; headers?: Record<string, string>; signal?: AbortSignal };

// request calls /api/v2. Mutations carry the same-origin guard header the
// server requires (X-Sela-Request). Network failures become code "network".
export async function request<T>(path: string, opts: Options = {}): Promise<T> {
  const method = opts.method ?? 'GET';
  const headers: Record<string, string> = { Accept: 'application/json', 'X-Sela-Language': currentLang, ...opts.headers };
  if (method !== 'GET') headers['X-Sela-Request'] = '1';
  let body: string | undefined;
  if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json';
    body = JSON.stringify(opts.body);
  }
  let res: Response;
  try {
    res = await fetch(`/api/v2${path}`, { method, headers, body, credentials: 'same-origin', signal: opts.signal });
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err;
    throw new ApiError('network', 0);
  }
  let payload: unknown = null;
  try {
    payload = await res.json();
  } catch {
    // non-JSON (e.g. proxy error page)
  }
  if (!res.ok) {
    const e = (payload as { error?: { code?: string; field?: string } } | null)?.error;
    throw new ApiError(e?.code ?? (res.status >= 500 ? 'internal' : 'invalid_input'), res.status, e?.field);
  }
  return (payload as { data: T }).data;
}

// newIdempotencyKey returns a random key accepted by POST /handoffs.
export function newIdempotencyKey(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}
