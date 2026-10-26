// Package generationfallback implements an opt-in, transparently attributed
// second-provider fallback for Sela's synthetic Meta-based drafting prototype.
// This is not a clinical service or an integrated Sela v2 API.
package generationfallback

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrBadInput    = errors.New("generationfallback: invalid demo input or consent")
	ErrUnavailable = errors.New("generationfallback: provider unavailable")
	ErrUnsafe      = errors.New("generationfallback: unsupported provider output")
)

const MetaProvider = "meta"
const DeepSeekProvider = "deepseek"
const DeepSeekModel = "deepseek-flash"
const DeepSeekVersion = "DeepSeek-V4.1-Flash-1017"

type Evidence struct {
	ID    string `json:"id"`
	Quote string `json:"quote"`
}

type Claim struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type Input struct {
	Synthetic        bool       `json:"synthetic"`
	CrisisGatePassed bool       `json:"-"` // Server-asserted; never accept this bit from a browser.
	MetaConsent      bool       `json:"-"`
	FallbackConsent  bool       `json:"-"`
	EnableFallback   bool       `json:"-"`
	FixtureID        string     `json:"fixture_id"`
	FixtureVersion   string     `json:"fixture_version"`
	BriefVersion     int        `json:"brief_version"`
	Language         string     `json:"language"`
	Evidence         []Evidence `json:"evidence"`
}

type Output struct {
	Claims []Claim `json:"claims"`
}

type Result struct {
	Output         Output `json:"output"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	Origin         string `json:"origin"` // live_meta or live_deepseek_fallback
	FallbackUsed   bool   `json:"fallback_used"`
	FixtureVersion string `json:"fixture_version"`
	BriefVersion   int    `json:"brief_version"`
}

type Generator interface {
	Generate(context.Context, Input) (Output, error)
}

type Router struct {
	Primary      Generator
	Secondary    Generator
	PrimaryModel string
}

func validateInput(in Input) error {
	if !in.Synthetic || !in.CrisisGatePassed || !in.MetaConsent ||
		in.FixtureID == "" || len(in.FixtureID) > 64 ||
		in.FixtureVersion == "" || len(in.FixtureVersion) > 64 ||
		in.BriefVersion < 1 || (in.Language != "id" && in.Language != "en") ||
		len(in.Evidence) == 0 || len(in.Evidence) > 12 {
		return ErrBadInput
	}
	ids := make(map[string]bool, len(in.Evidence))
	for _, e := range in.Evidence {
		if e.ID == "" || len(e.ID) > 48 || ids[e.ID] ||
			strings.TrimSpace(e.Quote) == "" || len(e.Quote) > 1400 {
			return ErrBadInput
		}
		ids[e.ID] = true
	}
	return nil
}

func validateOutput(in Input, out Output) error {
	if len(out.Claims) == 0 || len(out.Claims) > 12 {
		return ErrUnsafe
	}
	sourceIDs := make(map[string]bool, len(in.Evidence))
	for _, e := range in.Evidence {
		sourceIDs[e.ID] = true
	}
	claimIDs := map[string]bool{}
	for _, c := range out.Claims {
		if c.ID == "" || len(c.ID) > 48 || claimIDs[c.ID] ||
			len(c.Text) > 900 || strings.TrimSpace(c.Text) == "" ||
			len(c.EvidenceIDs) == 0 || len(c.EvidenceIDs) > 5 {
			return ErrUnsafe
		}
		claimIDs[c.ID] = true
		used := map[string]bool{}
		for _, id := range c.EvidenceIDs {
			if !sourceIDs[id] || used[id] {
				return ErrUnsafe
			}
			used[id] = true
		}
	}
	// These schema/ID checks do NOT prove semantic grounding. The real Go v2
	// application still needs deterministic evidence and user-review gates.
	return nil
}

func (r Router) Generate(ctx context.Context, in Input) (Result, error) {
	if err := validateInput(in); err != nil {
		return Result{}, err
	}
	if r.Primary == nil || strings.TrimSpace(r.PrimaryModel) == "" {
		return Result{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	output, err := r.Primary.Generate(ctx, in)
	if err == nil {
		if verr := validateOutput(in, output); verr != nil {
			// Unsafe output must NEVER trigger a silent fallback.
			return Result{}, verr
		}
		return Result{
			Output: output, Provider: MetaProvider, Model: r.PrimaryModel,
			Origin: "live_meta", FixtureVersion: in.FixtureVersion,
			BriefVersion: in.BriefVersion,
		}, nil
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	// Retryable transport/upstream failures only, never unsafe output,
	// policy refusal, invalid consent, or caller cancellation.
	if !errors.Is(err, ErrUnavailable) {
		return Result{}, err
	}
	if !in.EnableFallback || !in.FallbackConsent || r.Secondary == nil {
		return Result{}, ErrUnavailable
	}
	secondary, fallbackErr := r.Secondary.Generate(ctx, in)
	if fallbackErr != nil {
		return Result{}, ErrUnavailable
	}
	if err := validateOutput(in, secondary); err != nil {
		return Result{}, err
	}
	return Result{
		Output: secondary, Provider: DeepSeekProvider, Model: DeepSeekModel,
		Origin: "live_deepseek_fallback", FallbackUsed: true,
		FixtureVersion: in.FixtureVersion, BriefVersion: in.BriefVersion,
	}, nil
}
