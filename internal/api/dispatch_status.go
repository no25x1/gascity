package api

import "context"

func init() {
	RegisterSupervisorVoidAction("cities.list", ActionDef{
		Description: "List managed cities (supervisor)",
		ServerRoles: actionServerRoleSupervisor,
	}, func(_ context.Context, sm *SupervisorMux) (listResponse, error) {
		return sm.citiesList(), nil
	})

	RegisterVoidAction("status.get", ActionDef{
		Description:       "City status snapshot",
		RequiresCityScope: true,
		SupportsWatch:     true,
	}, func(_ context.Context, s *Server) (any, error) {
		return s.statusSnapshot(), nil
	})
}
