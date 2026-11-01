import { useCallback, useEffect, useState } from "react";
import { api, V3Error, type Consent, type Policy } from "./api";

/**
 * useConsent reads one optional consent and lets the page grant or
 * withdraw it against the current policy version.
 */
export function useConsent(purpose: string, policy?: Policy) {
  const [consent, setConsent] = useState<Consent | null | undefined>(undefined);
  const [error, setError] = useState<unknown>(null);

  const load = useCallback(() => {
    setError(null);
    api<{ consents: Consent[] }>("/me/consents")
      .then((d) => setConsent(d.consents.find((c) => c.purpose === purpose) ?? null))
      .catch((e) => {
        setError(e);
        setConsent(null);
      });
  }, [purpose]);

  useEffect(load, [load]);

  const set = useCallback(
    async (granted: boolean) => {
      const version = consent?.currentPolicyVersion ?? policy?.version;
      // a translated, retryable error instead of the bare word "policy"
      if (!version) throw new V3Error("policy_unavailable", "policy_unavailable");
      await api(`/me/consents/${purpose}`, { granted, policyVersion: version }, "PUT");
      load();
    },
    [consent, policy, purpose, load],
  );

  return {
    loading: consent === undefined,
    granted: Boolean(consent?.granted && !consent.outdated),
    outdated: Boolean(consent?.granted && consent.outdated),
    error,
    set,
    reload: load,
  };
}
