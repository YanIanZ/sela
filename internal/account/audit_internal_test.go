package account

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// R1-7: audit detail strings are identifiers only, never free text.
func TestAuditRefusesFreeTextDetail(t *testing.T) {
	s := &Store{}
	for _, v := range []string{"aku depresi, hubungi 0812 3456 7890", "Terms.V1", "", strings.Repeat("a", 65), "-x", "a b"} {
		if err := s.audit(context.Background(), nil, UserID{}, nil, "consent.granted", actorUser, map[string]any{"policyVersion": v}); err == nil {
			t.Errorf("audit accepted detail %q", v)
		}
	}
}

// The failure statement writes its audit events in SQL; they must stay on
// the allow-lists the audit helper enforces.
func TestFailureAuditEventsAreAllowListed(t *testing.T) {
	for _, typ := range []string{auditLoginFailed, auditLoginDisabled} {
		if !slices.Contains(AuditTypes, typ) {
			t.Errorf("%s is not an audit type", typ)
		}
	}
	if !slices.Contains(AuditDetailKeys, auditFailureCountKey) {
		t.Errorf("%s is not an audit detail key", auditFailureCountKey)
	}
}

// R1-7: a configured policy version must have the same safe shape.
func TestNewRefusesMalformedPolicyVersion(t *testing.T) {
	cfg := DefaultConfig("test")
	cfg.PolicyVersions = map[string]string{PolicyTerms: "Terms V1", PolicyGuest: "g.1", PolicyGuardian: "gi.1"}
	if err := checkPolicyVersions(cfg); err == nil {
		t.Fatal("malformed policy version accepted")
	}
	cfg.PolicyVersions[PolicyTerms] = "terms-privacy-2026-11-01.1"
	if err := checkPolicyVersions(cfg); err != nil {
		t.Fatal(err)
	}
}
