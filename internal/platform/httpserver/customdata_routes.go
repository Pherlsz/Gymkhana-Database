package httpserver

import (
	"log/slog"
	"net/http"
)

func registerCustomDataRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service customDataService) {
	registerCustomDataDefinitionRoutes(mux, logger, authentication, service)
	registerCustomDataValueRoutes(mux, logger, authentication, service)
}
