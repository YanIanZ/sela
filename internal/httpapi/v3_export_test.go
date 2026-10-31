package httpapi

// Test-only access to the /api/v3 route table and status table.

// RouteInfo describes one registered /api/v3 route.
type RouteInfo struct {
	Method, Path     string
	Mutation, Public bool
	GuardianGate     bool
}

// V3RouteTable lists the routes the server registered.
func V3RouteTable(s *Server) []RouteInfo {
	out := make([]RouteInfo, 0, len(s.v3.routes))
	for _, rt := range s.v3.routes {
		out = append(out, RouteInfo{Method: rt.method, Path: rt.path, Mutation: rt.mutation, Public: rt.public,
			GuardianGate: rt.guardianGate})
	}
	return out
}

// V3Status is the documented status of a code (0 when unknown).
func V3Status(code string) int { return v3Status[code] }

// Message returns the Indonesian and English message of a code.
func Message(code string) (id, en string, ok bool) {
	m, ok := messages[code]
	return m.id, m.en, ok
}
