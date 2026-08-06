package httpserver

import (
	"log/slog"
	"net/http"
)

func registerCustomDataRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, checker capabilityChecker, service customDataService) {
	registerCustomDataDefinitionRoutes(mux, logger, authentication, checker, service)
	registerCustomDataValueRoutes(mux, logger, authentication, checker, service)
}
