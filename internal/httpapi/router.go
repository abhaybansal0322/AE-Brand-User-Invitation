package httpapi

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/service"
)

type API struct {
	service *service.Service
	logger  *slog.Logger
}

func NewRouter(service *service.Service, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	api := &API{service: service, logger: logger}
	return recoverer(requestLogger(api, logger), logger)
}

func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz", "/readyz":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	accountID, suffix, ok := parseAccountPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Route not found")
		return
	}

	switch suffix {
	case "/users":
		switch r.Method {
		case http.MethodPost:
			api.handleCreateBrandUser(w, r, accountID)
		case http.MethodGet:
			api.handleGetUsersOfAccount(w, r, accountID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		}
	case "/users:batch":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		api.handleUpdateUsersOrAccount(w, r, accountID)
	case "/audit":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		api.handleQueryAudit(w, r, accountID)
	default:
		writeError(w, http.StatusNotFound, "not_found", "Route not found")
	}
}

func parseAccountPath(path string) (string, string, bool) {
	const prefix = "/api/v1/account/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", false
	}
	return parts[0], "/" + parts[1], true
}
