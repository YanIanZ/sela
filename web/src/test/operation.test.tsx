import '@testing-library/jest-dom/vitest';
import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react';
import { App } from '../App';
import { CLOSED_POLL_MS, MAX_RESUBSCRIBE, RESUBSCRIBE_MS, useOperation } from '../lib/operation';
import type { OpRecord } from '../lib/types';
import { Providers, approvedSession, mockApi, setPath } from './helpers';

// FakeEventSource stands in for the browser's EventSource (jsdom has none).
class FakeEventSource {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSED = 2;
  static all: FakeEventSource[] = [];
  readyState = FakeEventSource.CONNECTING;
  listeners: Record<string, ((e: MessageEvent) => void)[]> = {};
  constructor(public url: string) {
    FakeEventSource.all.push(this);
  }
  addEventListener(type: string, fn: (e: MessageEvent) => void) {
    (this.listeners[type] ??= []).push(fn);
  }
  close() {
    this.readyState = FakeEventSource.CLOSED;
  }
  emit(type: string, data?: unknown) {
    const e = new MessageEvent(type, data === undefined ? {} : { data: JSON.stringify(data) });
    for (const fn of this.listeners[type] ?? []) fn(e);
  }
}

beforeEach(() => {
  FakeEventSource.all = [];
  vi.stubGlobal('EventSource', FakeEventSource);
});
afterEach(() => vi.unstubAllGlobals());

describe('useOperation', () => {
  it('ends the operation and reloads the session when the stream is closed for good', () => {
    const settle = vi.fn();
    const { result } = renderHook(() => useOperation('op1', settle));
    const es = FakeEventSource.all[0]!;
    act(() => es.emit('status', { state: 'running' }));
    expect(result.current.phase).toBe('running');
    // A transport error while reconnecting is retried by the browser...
    act(() => es.emit('error'));
    expect(result.current.phase).toBe('running');
    expect(settle).not.toHaveBeenCalled();
    // ...but a refused reconnect (401 after a restart) closes the stream.
    act(() => {
      es.readyState = FakeEventSource.CLOSED;
      es.emit('error');
    });
    expect(result.current.phase).toBe('error');
    expect(result.current.errorCode).toBe('ai_unavailable');
    expect(settle).toHaveBeenCalledTimes(1);
  });

  it('subscribes again after a refused reconnect while the session still shows the operation running', () => {
    vi.useFakeTimers();
    try {
      const running: OpRecord = { id: 'op1', kind: 'ledger_draft', state: 'running', startedAt: '' };
      const { result } = renderHook(() => useOperation('op1', () => undefined, running));
      const first = FakeEventSource.all[0]!;
      act(() => {
        first.readyState = FakeEventSource.CLOSED;
        first.emit('error');
      });
      // A refused stream (e.g. a 429) is no failure of the step itself.
      expect(result.current.phase).toBe('reconnecting');
      expect(result.current.errorCode).toBeUndefined();
      expect(result.current.stoppable).toBe(true);
      act(() => vi.advanceTimersByTime(2500));
      expect(FakeEventSource.all.length).toBe(2);
      expect(result.current.phase).toBe('queued');
      act(() => FakeEventSource.all[1]!.emit('done', { accepted: 2 }));
      expect(result.current.phase).toBe('done');
    } finally {
      vi.useRealTimers();
    }
  });

  it('keeps a step that cannot be followed stoppable, polls the session, and shows how it ended', () => {
    vi.useFakeTimers();
    try {
      const settle = vi.fn();
      const running: OpRecord = { id: 'op1', kind: 'ledger_draft', state: 'running', startedAt: '' };
      const { result, rerender } = renderHook(({ rec }) => useOperation('op1', settle, rec), { initialProps: { rec: running } });
      const refuse = () =>
        act(() => {
          const es = FakeEventSource.all[FakeEventSource.all.length - 1]!;
          es.readyState = FakeEventSource.CLOSED;
          es.emit('error');
        });
      refuse();
      for (let i = 0; i < MAX_RESUBSCRIBE; i++) {
        expect(result.current.phase).toBe('reconnecting');
        act(() => vi.advanceTimersByTime(RESUBSCRIBE_MS * 2 ** i + 10));
        expect(FakeEventSource.all.length).toBe(i + 2);
        refuse();
      }
      // The attempts are used up: no more streams, an error, but Stop stays.
      act(() => vi.advanceTimersByTime(RESUBSCRIBE_MS + 10));
      expect(FakeEventSource.all.length).toBe(MAX_RESUBSCRIBE + 1);
      expect(result.current.phase).toBe('error');
      expect(result.current.stoppable).toBe(true);
      const calls = settle.mock.calls.length;
      act(() => vi.advanceTimersByTime(CLOSED_POLL_MS * 2 + 10));
      expect(settle.mock.calls.length).toBe(calls + 2);
      // The reloaded session shows the step succeeded on the server.
      rerender({ rec: { ...running, state: 'succeeded' } });
      act(() => vi.advanceTimersByTime(10));
      expect(result.current.phase).toBe('done');
      expect(result.current.stoppable).toBe(false);
      act(() => vi.advanceTimersByTime(CLOSED_POLL_MS * 2));
      expect(settle.mock.calls.length).toBe(calls + 2);
    } finally {
      vi.useRealTimers();
    }
  });

  it('a refused stream for a step the session reports canceled ends as stopped', () => {
    vi.useFakeTimers();
    try {
      const running: OpRecord = { id: 'op1', kind: 'ledger_draft', state: 'running', startedAt: '' };
      const { result, rerender } = renderHook(({ rec }) => useOperation('op1', () => undefined, rec), { initialProps: { rec: running } });
      act(() => {
        const es = FakeEventSource.all[0]!;
        es.readyState = FakeEventSource.CLOSED;
        es.emit('error');
      });
      rerender({ rec: { ...running, state: 'canceled' } });
      act(() => vi.advanceTimersByTime(10));
      expect(result.current.phase).toBe('error');
      expect(result.current.errorCode).toBe('canceled');
      act(() => vi.advanceTimersByTime(RESUBSCRIBE_MS * 4));
      expect(FakeEventSource.all.length).toBe(1);
      expect(result.current.stoppable).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });

  it('ends the panel when the session view reports a terminal state the stream never sent', async () => {
    vi.useFakeTimers();
    try {
      const running: OpRecord = { id: 'op1', kind: 'ledger_draft', state: 'running', startedAt: '' };
      const { result, rerender } = renderHook(({ rec }) => useOperation('op1', () => undefined, rec), { initialProps: { rec: running } });
      act(() => FakeEventSource.all[0]!.emit('status', { state: 'running' }));
      rerender({ rec: { ...running, state: 'failed', errorCode: 'ai_timeout' } });
      act(() => vi.advanceTimersByTime(5000));
      expect(result.current.phase).toBe('error');
      expect(result.current.errorCode).toBe('ai_timeout');
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('Practice opening draft', () => {
  it('resumes a running draft from the session and can stop it', async () => {
    setPath('/practice');
    const op: OpRecord = { id: 'op-open', kind: 'opening_draft', state: 'running', startedAt: new Date().toISOString() };
    const s = approvedSession({ ops: [op] });
    const api = mockApi({
      'GET /session': () => ({ data: s }),
      'GET /ai/status': () => ({ data: s.ai }),
      'DELETE /operations/op-open': () => ({ data: { ...s, version: s.version + 1, ops: [{ ...op, state: 'canceled' }] } }),
    });
    render(
      <Providers>
        <App />
      </Providers>,
    );
    const stop = await screen.findByRole('button', { name: 'Stop' });
    expect(FakeEventSource.all.some((e) => e.url.includes('op-open'))).toBe(true);
    fireEvent.click(stop);
    await waitFor(() => expect(api.count('DELETE /operations/op-open')).toBe(1));
    act(() => FakeEventSource.all.find((e) => e.url.includes('op-open'))!.emit('error', { code: 'canceled' }));
    expect(await screen.findByText('The AI step was stopped. Nothing was saved.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Stop' })).toBeNull();
  });
});

describe('Ledger AI draft with a refused stream', () => {
  it('keeps the step stoppable instead of reporting the model unavailable', async () => {
    setPath('/ledger');
    const op: OpRecord = { id: 'op-l', kind: 'ledger_draft', state: 'queued', startedAt: new Date().toISOString() };
    const s = approvedSession({ ops: [op] });
    const api = mockApi({
      'GET /session': () => ({ data: s }),
      'GET /ai/status': () => ({ data: s.ai }),
      'DELETE /operations/op-l': () => ({ data: { ...s, version: s.version + 1, ops: [{ ...op, state: 'canceled' }] } }),
    });
    render(
      <Providers>
        <App />
      </Providers>,
    );
    await screen.findByRole('button', { name: 'Stop' });
    act(() => {
      const es = FakeEventSource.all.find((e) => e.url.includes('op-l'))!;
      es.readyState = FakeEventSource.CLOSED;
      es.emit('error'); // e.g. the events request answered 429
    });
    expect(await screen.findByText('Reconnecting to the AI step…')).toBeInTheDocument();
    expect(screen.queryByText(/The AI model is unavailable/)).toBeNull();
    expect(screen.queryByRole('button', { name: /Try again|Help draft my note with AI/ })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Stop' }));
    await waitFor(() => expect(api.count('DELETE /operations/op-l')).toBe(1));
    expect(await screen.findByText('The AI step was stopped. Nothing was saved.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Stop' })).toBeNull();
  });
});
