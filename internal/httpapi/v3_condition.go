package httpapi

import (
	"net/http"
	"time"

	"github.com/YanIanZ/sela/internal/account"
)

// v3ConditionMap returns the person's condition map (S-020). It needs the
// separate, optional condition_map consent, reads only saved records, is
// computed on request and is never stored, logged or sent to a model.
func (s *Server) v3ConditionMap(w http.ResponseWriter, r *http.Request, p account.Principal) {
	consents, err := s.opt.Accounts.Consents(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	granted := false
	for _, c := range consents {
		if c.Purpose == account.PolicyConditionMap && c.Granted && !c.Outdated {
			granted = true
		}
	}
	if !granted {
		writeError(w, r, http.StatusForbidden, "consent_required", account.PolicyConditionMap)
		return
	}
	records, err := s.opt.Accounts.Records(r.Context(), p.UserID)
	if err != nil {
		s.v3Error(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, account.ComputeConditionMap(records, time.Now()))
}

// demoPractitioner is a fictional professional for the product demo
// (S-022). None of them exists, none can be contacted, and nothing a person
// writes is ever sent to them.
type demoPractitioner struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Role      map[string]string `json:"role"`
	Focus     map[string]string `json:"focus"`
	Modes     []string          `json:"modes"`
	City      string            `json:"city"`
	Languages []string          `json:"languages"`
	Stages    []string          `json:"stages"`
	Synthetic bool              `json:"synthetic"`
}

var demoPractitioners = []demoPractitioner{
	{ID: "demo-psy-1", Name: "Psikolog Klinis Demo A", Role: map[string]string{"id": "Psikolog klinis (fiktif)", "en": "Clinical psychologist (fictional)"},
		Focus: map[string]string{"id": "Stres kerja, kecemasan, dan kelelahan", "en": "Work stress, anxiety and exhaustion"},
		Modes: []string{"online", "in_person"}, City: "Surabaya", Languages: []string{"id", "en"}, Stages: []string{account.StageAttention, account.StageSupport}, Synthetic: true},
	{ID: "demo-psy-2", Name: "Konselor Demo B", Role: map[string]string{"id": "Konselor (fiktif)", "en": "Counsellor (fictional)"},
		Focus: map[string]string{"id": "Relasi, keluarga, dan kesepian", "en": "Relationships, family and loneliness"},
		Modes: []string{"online"}, City: "Jakarta", Languages: []string{"id"}, Stages: []string{account.StageEveryday, account.StageAttention}, Synthetic: true},
	{ID: "demo-psy-3", Name: "Psikiater Demo C", Role: map[string]string{"id": "Psikiater (fiktif)", "en": "Psychiatrist (fictional)"},
		Focus: map[string]string{"id": "Penilaian medis menyeluruh bila keluhan berat atau menetap", "en": "Full medical assessment when distress is severe or persistent"},
		Modes: []string{"in_person"}, City: "Bandung", Languages: []string{"id", "en"}, Stages: []string{account.StageSupport, account.StageUrgent}, Synthetic: true},
	{ID: "demo-fac-1", Name: "Puskesmas Demo Sejahtera", Role: map[string]string{"id": "Layanan kesehatan primer (fiktif)", "en": "Primary care clinic (fictional)"},
		Focus: map[string]string{"id": "Pemeriksaan awal dan rujukan ke layanan jiwa", "en": "First check and referral to mental-health services"},
		Modes: []string{"in_person"}, City: "Denpasar", Languages: []string{"id"}, Stages: []string{account.StageAttention, account.StageSupport, account.StageUrgent}, Synthetic: true},
}

// v3CareDemo lists the fictional professionals of the product demo.
func (s *Server) v3CareDemo(w http.ResponseWriter, r *http.Request) {
	writeData(w, http.StatusOK, map[string]any{"practitioners": demoPractitioners, "synthetic": true})
}
