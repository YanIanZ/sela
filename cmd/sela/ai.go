package main

import (
	"context"
	"errors"
	"github.com/YanIanZ/sela/internal/account"
	"github.com/YanIanZ/sela/internal/ai"
	"strconv"
)

type accountAIConsent struct{ store *account.Store }

func (a accountAIConsent) Consent(ctx context.Context, subject ai.Subject, purpose ai.Purpose) (ai.Consent, error) {
	id, err := account.ParseUUID(string(subject))
	if err != nil {
		return ai.Consent{}, err
	}
	cs, err := a.store.Consents(ctx, account.UserID(id))
	if err != nil {
		return ai.Consent{}, err
	}
	for _, c := range cs {
		if c.Purpose == string(purpose) {
			return ai.Consent{Granted: c.Granted && !c.Outdated, Revision: strconv.Itoa(c.Revision)}, nil
		}
	}
	return ai.Consent{}, nil
}
func (a accountAIConsent) AgeBand(ctx context.Context, subject ai.Subject) (ai.AgeBand, error) {
	id, err := account.ParseUUID(string(subject))
	if err != nil {
		return "", err
	}
	p, err := a.store.Profile(ctx, account.UserID(id))
	if err != nil {
		return "", err
	}
	if p.GuardianConsent != nil && p.GuardianConsent.Required && p.GuardianConsent.Status != "active" {
		return "", errors.New("guardian permission required")
	}
	return ai.AgeBand(p.AgeBand), nil
}
