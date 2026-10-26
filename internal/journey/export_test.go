package journey

// Test-only accessors for the external journey_test package.
var (
	InputOpen  = InputOpenMarker
	InputClose = InputCloseMarker
)

func (s *Service) Policy(key string) string { return s.policy(key) }

func (s *Service) MaxOpsPerWindow() int { return s.cfg.MaxOpsPerWindow }
