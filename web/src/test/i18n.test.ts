import { id } from '../i18n/id';
import { en } from '../i18n/en';

const placeholders = (s: string) => (s.match(/\{[a-zA-Z]+\}/g) ?? []).sort().join(',');

describe('i18n dictionaries', () => {
  it('have identical key sets', () => {
    expect(Object.keys(en).sort()).toEqual(Object.keys(id).sort());
  });
  it('have no empty strings', () => {
    for (const [k, v] of Object.entries(id)) expect(v.trim(), `id:${k}`).not.toBe('');
    for (const [k, v] of Object.entries(en)) expect(v.trim(), `en:${k}`).not.toBe('');
  });
  it('use the same interpolation placeholders in both languages', () => {
    for (const k of Object.keys(id) as (keyof typeof id)[]) {
      expect(placeholders(en[k]), k).toBe(placeholders(id[k]));
    }
  });
  it('cover every server error code used by the API contract', () => {
    const codes = [
      'invalid_input', 'session_missing', 'stale_version', 'consent_required', 'crisis_interrupted', 'approval_required',
      'review_incomplete', 'not_found', 'forbidden', 'rate_limited', 'ai_disabled', 'ai_unavailable', 'ai_timeout', 'ai_auth',
      'ai_busy', 'ai_model_mismatch', 'ai_invalid_output', 'ai_ungrounded', 'stale_result', 'canceled', 'operation_in_progress',
      'invalid_transition', 'idempotency_conflict', 'idempotency_key_required', 'capacity', 'contact_details_detected',
      'policy_outdated', 'access_ended', 'handoff_exists', 'slot_unavailable', 'story_required', 'adult_attestation_required',
      'too_large', 'cross_origin', 'network', 'internal', 'research_unavailable', 'limit_reached',
    ];
    for (const c of codes) expect(Object.keys(id), c).toContain(`error.${c}`);
  });
});
