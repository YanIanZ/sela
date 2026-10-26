import { useEffect, useRef, useState } from 'react';
import type { OpRecord } from './types';

export type OpEventData = Record<string, unknown>;
export type OpState = {
  /** reconnecting: the stream was refused (e.g. a 429) while the session
   * still shows the operation queued or running; it is subscribed again. */
  phase: 'idle' | 'queued' | 'warming' | 'running' | 'validating' | 'reconnecting' | 'done' | 'error';
  previews: OpEventData[];
  done?: OpEventData;
  errorCode?: string;
  coldStartMs?: number;
  /** streamClosed marks a state that came from a refused reconnect, not from the operation. */
  streamClosed?: boolean;
};

/** The hook's result: the state, and whether the operation may still be
 * running on the server, so Stop must stay offered (also after the stream
 * could not be followed any more). */
export type OpView = OpState & { stoppable: boolean };

/** A stream without any event for this long makes the caller reload the session. */
export const OP_QUIET_MS = 30000;
/** How long the stream may lag behind a terminal state in the session view. */
export const RECORD_GRACE_MS = 3000;
/** After a refused reconnect, how long to wait for the reloaded session
 * before resubscribing; it doubles with each further attempt. */
export const RESUBSCRIBE_MS = 2000;
export const MAX_RESUBSCRIBE = 3;
/** Once the stream cannot be followed, how often the session is reloaded
 * while it still shows the operation running (so its end is shown). */
export const CLOSED_POLL_MS = 10000;

const ACTIVE: OpState['phase'][] = ['queued', 'warming', 'running', 'validating', 'reconnecting'];
const running = (st?: OpRecord['state']) => st === 'queued' || st === 'running';
export const isActive = (s: OpState) => ACTIVE.includes(s.phase);

// useOperation follows an AI operation's server-sent events. EventSource
// reconnects with Last-Event-ID, and the server replays missed events.
// onSettle is called when the operation ends, when the stream closes for
// good (the server answered the reconnect with an error, e.g. after a
// restart or expiry), and when the stream has been quiet for OP_QUIET_MS;
// callers reload the session there. record is the operation as the session
// view reports it: a terminal state there ends the panel even if the stream
// never said so. A refused reconnect (e.g. a 429) says nothing about the
// operation itself: while the record still shows it queued or running, the
// panel stays active ("reconnecting", with Stop) and the stream is opened
// again a few times. If it still cannot be followed, the result is an error
// that stays stoppable, and the session is reloaded every CLOSED_POLL_MS
// until the record shows how the operation ended.
export function useOperation(opId: string | null, onSettle: () => void, record?: OpRecord): OpView {
  const [state, setState] = useState<OpState>({ phase: 'idle', previews: [] });
  const [subscription, setSubscription] = useState(0);
  // Counts refused streams; each one schedules the resubscribe decision.
  const [closures, setClosures] = useState(0);
  const resubscribed = useRef(0);
  const closedRef = useRef(false);
  closedRef.current = state.streamClosed === true;
  const settleRef = useRef(onSettle);
  settleRef.current = onSettle;
  const stopRef = useRef<(() => void) | null>(null);
  const recordRef = useRef<OpRecord | undefined>(record);
  recordRef.current = record && record.id === opId ? record : undefined;
  useEffect(() => {
    resubscribed.current = 0;
    setClosures(0); // drops a resubscribe decision pending for the previous operation
  }, [opId]);
  useEffect(() => {
    if (!opId) {
      setState({ phase: 'idle', previews: [] });
      return;
    }
    setState({ phase: 'queued', previews: [] });
    const es = new EventSource(`/api/v2/operations/${encodeURIComponent(opId)}/events`);
    let quiet: number | undefined;
    const stop = () => {
      es.close();
      if (quiet !== undefined) window.clearInterval(quiet);
      quiet = undefined;
    };
    stopRef.current = stop;
    let lastEvent = Date.now();
    quiet = window.setInterval(() => {
      if (Date.now() - lastEvent >= OP_QUIET_MS) {
        lastEvent = Date.now();
        settleRef.current();
      }
    }, 5000);
    const parse = (e: MessageEvent): OpEventData => {
      lastEvent = Date.now();
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
      stop();
      settleRef.current();
    });
    es.addEventListener('error', (e) => {
      const me = e as MessageEvent;
      if (typeof me.data === 'string' && me.data) {
        const d = parse(me);
        setState((s) => ({ ...s, phase: 'error', errorCode: String(d.code ?? 'ai_unavailable') }));
        stop();
        settleRef.current();
        return;
      }
      // A transport error while CONNECTING is retried by EventSource itself.
      // CLOSED means the reconnect was refused (401 after a restart or
      // expiry, 404 for an unknown operation, 429): no more events will come
      // on this stream. The operation may still run on the server, so the
      // panel stays active while the reloaded session decides.
      if (es.readyState === EventSource.CLOSED) {
        stop();
        const known = recordRef.current !== undefined;
        setState((s) =>
          ACTIVE.includes(s.phase)
            ? known
              ? { ...s, phase: 'reconnecting', streamClosed: true }
              : { ...s, phase: 'error', errorCode: 'ai_unavailable', streamClosed: true }
            : s,
        );
        if (known) setClosures((n) => n + 1);
        settleRef.current();
      }
    });
    return () => {
      stop();
      if (stopRef.current === stop) stopRef.current = null;
    };
  }, [opId, subscription]);

  // A refused reconnect while the reloaded session still shows the operation
  // running: subscribe again (with a growing delay) rather than leaving a
  // step nobody can follow. Once the attempts are used up, say so, but keep
  // the operation stoppable (see stoppable below).
  useEffect(() => {
    if (closures === 0) return;
    const timer = window.setTimeout(() => {
      const st = recordRef.current?.state;
      if (running(st) && resubscribed.current < MAX_RESUBSCRIBE) {
        resubscribed.current += 1;
        setSubscription((n) => n + 1);
        return;
      }
      // A terminal record is handled by the record effect below.
      if (running(st) || st === undefined) setState((s) => (s.phase === 'reconnecting' ? { ...s, phase: 'error', errorCode: 'ai_unavailable' } : s));
      // Back off between attempts; the last decision only waits for the reloaded session.
    }, RESUBSCRIBE_MS * 2 ** (resubscribed.current < MAX_RESUBSCRIBE ? resubscribed.current : 0));
    return () => window.clearTimeout(timer);
  }, [closures]);

  const recordState = record && record.id === opId ? record.state : undefined;
  const recordError = record?.errorCode;
  const recordRunning = running(recordState);
  const lost = state.phase === 'error' && state.streamClosed === true;

  // The stream cannot be followed but the operation still runs: reload the
  // session now and then, so its end (success, failure, Stop) is shown.
  useEffect(() => {
    if (!lost || !recordRunning) return;
    const timer = window.setInterval(() => settleRef.current(), CLOSED_POLL_MS);
    return () => window.clearInterval(timer);
  }, [lost, recordRunning]);

  useEffect(() => {
    if (recordState !== 'succeeded' && recordState !== 'failed' && recordState !== 'canceled') return;
    // Give the stream a moment to deliver its own final event (with the
    // result summary) before ending the panel from the session view; with
    // no stream left there is nothing to wait for.
    const timer = window.setTimeout(
      () => {
        setState((s) => {
          // A step followed on the stream, or one that could not be followed
          // any more, ends with the state the session reports.
          if (!ACTIVE.includes(s.phase) && !(s.phase === 'error' && s.streamClosed)) return s;
          if (recordState === 'succeeded') return { ...s, phase: 'done', errorCode: undefined };
          return { ...s, phase: 'error', errorCode: recordState === 'canceled' ? 'canceled' : (recordError ?? 'ai_unavailable') };
        });
        stopRef.current?.();
      },
      closedRef.current ? 0 : RECORD_GRACE_MS,
    );
    return () => window.clearTimeout(timer);
  }, [recordState, recordError]);
  const stoppable = isActive(state) || (lost && recordRunning);
  return { ...state, stoppable };
}
