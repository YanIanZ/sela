import { useEffect, useRef, useState } from 'react';

export type OpEventData = Record<string, unknown>;
export type OpState = {
  phase: 'idle' | 'queued' | 'warming' | 'running' | 'validating' | 'done' | 'error';
  previews: OpEventData[];
  done?: OpEventData;
  errorCode?: string;
  coldStartMs?: number;
};

// useOperation follows an AI operation's server-sent events. EventSource
// reconnects with Last-Event-ID, and the server replays missed events.
export function useOperation(opId: string | null, onFinish: () => void): OpState {
  const [state, setState] = useState<OpState>({ phase: 'idle', previews: [] });
  const finishRef = useRef(onFinish);
  finishRef.current = onFinish;
  useEffect(() => {
    if (!opId) {
      setState({ phase: 'idle', previews: [] });
      return;
    }
    setState({ phase: 'queued', previews: [] });
    const es = new EventSource(`/api/v2/operations/${encodeURIComponent(opId)}/events`);
    const parse = (e: MessageEvent): OpEventData => {
      try {
        return JSON.parse(e.data as string) as OpEventData;
      } catch {
        return {};
      }
    };
    es.addEventListener('status', (e) => {
      const d = parse(e as MessageEvent);
      const phase = d.state as OpState['phase'];
      setState((s) => ({ ...s, phase, coldStartMs: typeof d.coldStartEstimateMs === 'number' ? d.coldStartEstimateMs : s.coldStartMs }));
    });
    es.addEventListener('item', (e) => {
      const d = parse(e as MessageEvent);
      setState((s) => ({ ...s, previews: [...s.previews, d] }));
    });
    es.addEventListener('done', (e) => {
      const d = parse(e as MessageEvent);
      setState((s) => ({ ...s, phase: 'done', done: d }));
      es.close();
      finishRef.current();
    });
    es.addEventListener('error', (e) => {
      const me = e as MessageEvent;
      if (typeof me.data === 'string' && me.data) {
        const d = parse(me);
        setState((s) => ({ ...s, phase: 'error', errorCode: String(d.code ?? 'ai_unavailable') }));
        es.close();
        finishRef.current();
      }
      // Without data this is a transport error; EventSource retries by itself.
    });
    return () => es.close();
  }, [opId]);
  return state;
}
