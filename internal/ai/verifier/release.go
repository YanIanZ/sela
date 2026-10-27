package verifier

// Disposition is what VC-8 does with one claim's AI wording.
type Disposition string

// Dispositions.
const (
	// Release: the claim is supported and carries no issue, so its AI
	// wording is released.
	Release Disposition = "release"
	// QuoteFallback: the user's own quote is shown instead, with Sela's
	// fixed explanation for the verdict or issue.
	QuoteFallback Disposition = "user_quote"
	// Drop: there is no quote to fall back to (an opening line, a reply
	// sentence), so the sentence is dropped.
	Drop Disposition = "drop"
)

// Explanation codes. The UI maps each code to Sela's reviewed bilingual
// text (VC-9); the verifier's own reason is shown only under "Detail
// pemeriksaan / Check details".
const (
	ExplainSupported = "verified.supported"
)

// ExplainVerdict returns the explanation code for a non-supported verdict.
func ExplainVerdict(v Verdict) string { return "verdict." + string(v) }

// ExplainIssue returns the explanation code for an issue category.
func ExplainIssue(c Category) string { return "issue." + string(c) }

// ReleaseOptions configure Decide for a task.
type ReleaseOptions struct {
	// WithholdOnAnyNotReleased: any claim that is not released withholds
	// the whole output (AT-1: one non-supported sentence withholds the
	// whole reply).
	WithholdOnAnyNotReleased bool
	// HasQuote reports whether a claim has the user's own quote to fall
	// back to (nil: none has).
	HasQuote func(claimID string) bool
}

// ClaimDecision is the VC-8 outcome for one claim.
type ClaimDecision struct {
	ClaimID     string
	Verdict     Verdict
	Disposition Disposition
	Issues      []Category
	Explanation string
	// Reason is the verifier's short reason, for "Check details" only.
	Reason string
}

// Decision is the VC-8 outcome for one output.
type Decision struct {
	Claims []ClaimDecision
	// WithholdAll: nothing of the output is released.
	WithholdAll bool
	// Withheld lists the output-level issue categories that withheld it.
	Withheld []Category
	// SafetySignal raises the route: the inline help card is shown and the
	// output is withheld (CK-9 can only raise).
	SafetySignal bool
}

// Released reports how many claims keep their AI wording.
func (d Decision) Released() int {
	if d.WithholdAll {
		return 0
	}
	n := 0
	for _, c := range d.Claims {
		if c.Disposition == Release {
			n++
		}
	}
	return n
}

// OutputLevel reports whether an issue of category c withholds the whole
// output wherever the verifier attached it (VC-8: medical_language,
// manipulation, age_inappropriate, information_leak). A claim-scoped
// issue of another category (lost_context, citation_invalid) concerns
// that claim only.
func OutputLevel(c Category) bool {
	switch c {
	case MedicalLanguage, Manipulation, AgeInappropriate, InformationLeak:
		return true
	}
	return false
}

// Decide applies the release rules of VC-8 to a validated report:
//   - AI wording is released only for claims that are supported and carry
//     no issue;
//   - other claims fall back to the user's quote where one exists, and are
//     dropped otherwise;
//   - an output-level issue (medical_language, manipulation,
//     age_inappropriate, information_leak; see OutputLevel) withholds the
//     whole output whatever its claim_id, and an issue that concerns the
//     whole output (claim_id "") withholds it for every category, which is
//     stricter than VC-8's list of four and fails closed;
//   - safety_signal anywhere raises the route and withholds the output;
//   - with WithholdOnAnyNotReleased, one claim that is not released
//     withholds the whole output.
//
// A verdict never overrides a deterministic rejection, a user review,
// consent, approval or sharing: Decide can only withhold.
func Decide(req Request, rep Report, o ReleaseOptions) Decision {
	byClaim := map[string][]Category{}
	var d Decision
	for _, is := range rep.Issues {
		if is.Category == SafetySignal {
			d.SafetySignal = true
		}
		if is.ClaimID == "" {
			d.Withheld = appendUnique(d.Withheld, is.Category)
			continue
		}
		if OutputLevel(is.Category) {
			// The claim keeps the issue in its record, and the whole
			// output is withheld.
			d.Withheld = appendUnique(d.Withheld, is.Category)
		}
		byClaim[is.ClaimID] = appendUnique(byClaim[is.ClaimID], is.Category)
	}
	verdicts := make(map[string]ClaimVerdict, len(rep.Claims))
	for _, c := range rep.Claims {
		verdicts[c.ClaimID] = c
	}
	notReleased := false
	for _, c := range req.Claims {
		cv, ok := verdicts[c.ID]
		cd := ClaimDecision{ClaimID: c.ID, Verdict: cv.Verdict, Issues: byClaim[c.ID], Reason: cv.Reason}
		switch {
		case ok && cv.Verdict == Supported && len(cd.Issues) == 0:
			cd.Disposition, cd.Explanation = Release, ExplainSupported
		default:
			notReleased = true
			cd.Explanation = ExplainVerdict(cv.Verdict)
			if cv.Verdict == Supported || !ok {
				if len(cd.Issues) > 0 {
					cd.Explanation = ExplainIssue(cd.Issues[0])
				} else {
					cd.Explanation = ExplainVerdict(InsufficientEvidence)
				}
			}
			cd.Disposition = Drop
			if o.HasQuote != nil && o.HasQuote(c.ID) {
				cd.Disposition = QuoteFallback
			}
		}
		d.Claims = append(d.Claims, cd)
	}
	if d.SafetySignal || len(d.Withheld) > 0 || (o.WithholdOnAnyNotReleased && notReleased) {
		d.WithholdAll = true
	}
	return d
}

func appendUnique(list []Category, c Category) []Category {
	for _, x := range list {
		if x == c {
			return list
		}
	}
	return append(list, c)
}
