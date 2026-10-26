package journey

// Test-only accessors for the external journey_test package.
var (
	InputOpen  = InputOpenMarker
	InputClose = InputCloseMarker
)

func (s *Service) Policy(key string) string { return s.policy(key) }

func (s *Service) MaxOpsPerWindow() int { return s.cfg.MaxOpsPerWindow }

// RuntimeEvents returns the events an operation runtime still holds in
// process memory, and whether the runtime exists at all.
func (s *Service) RuntimeEvents(opID string) ([]OpEvent, bool) {
	s.opsMu.Lock()
	rt := s.ops[opID]
	s.opsMu.Unlock()
	if rt == nil {
		return nil, false
	}
	ev, _, _ := rt.Events(0)
	return ev, true
}
