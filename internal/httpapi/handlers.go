package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
)

const maxRequestBodyBytes = 1 << 20

type createBrandUserRequest struct {
	RequestContext map[string]any    `json:"request_context,omitempty"`
	Email          string            `json:"email"`
	Name           string            `json:"name"`
	Mobile         string            `json:"mobile"`
	Personas       []domain.Persona  `json:"personas"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type updateUsersOrAccountRequest struct {
	RequestContext map[string]any         `json:"request_context,omitempty"`
	UserOperations []userOperationRequest `json:"user_operations"`
	Metadata       map[string]string      `json:"metadata,omitempty"`
}

type userOperationRequest struct {
	Operation string           `json:"operation"`
	UserID    string           `json:"user_id"`
	UserPool  string           `json:"user_pool,omitempty"`
	Personas  []domain.Persona `json:"personas,omitempty"`
}

func (api *API) handleCreateBrandUser(w http.ResponseWriter, r *http.Request, accountID string) {
	var req createBrandUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := api.service.CreateBrandUser(r.Context(), domain.CreateBrandUserInput{
		AccountID: accountID,
		Email:     req.Email,
		Name:      req.Name,
		Mobile:    req.Mobile,
		Personas:  req.Personas,
		Actor:     actorFromHeaders(r),
		RequestID: requestIDFromHeaders(r),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"user_id": result.User.UserID,
		"message": result.Message,
	})
}

func (api *API) handleUpdateUsersOrAccount(w http.ResponseWriter, r *http.Request, accountID string) {
	var req updateUsersOrAccountRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	operations := make([]domain.UserOperation, 0, len(req.UserOperations))
	for _, operation := range req.UserOperations {
		operations = append(operations, domain.UserOperation{
			Operation: domain.UserOperationType(strings.ToUpper(strings.TrimSpace(operation.Operation))),
			UserID:    strings.TrimSpace(operation.UserID),
			UserPool:  strings.TrimSpace(operation.UserPool),
			Personas:  operation.Personas,
		})
	}

	results, err := api.service.ApplyUserOperations(r.Context(), accountID, actorFromHeaders(r), operations, requestIDFromHeaders(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]domain.OperationResult{"results": results})
}

func (api *API) handleGetUsersOfAccount(w http.ResponseWriter, r *http.Request, accountID string) {
	users, err := api.service.ListUsers(r.Context(), accountID, actorFromHeaders(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]domain.AccountUser{"users": users})
}

func (api *API) handleQueryAudit(w http.ResponseWriter, r *http.Request, accountID string) {
	filter := domain.AuditFilter{
		AccountID:     accountID,
		ActorID:       strings.TrimSpace(r.URL.Query().Get("actor_id")),
		TargetUserID:  strings.TrimSpace(r.URL.Query().Get("target_user_id")),
		Operation:     domain.AuditOperation(strings.TrimSpace(r.URL.Query().Get("operation"))),
		RetentionDays: parsePositiveInt(r.URL.Query().Get("retention_days")),
	}

	var err error
	if from := strings.TrimSpace(r.URL.Query().Get("from")); from != "" {
		filter.From, err = time.Parse(time.RFC3339, from)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "from must be RFC3339")
			return
		}
	}
	if to := strings.TrimSpace(r.URL.Query().Get("to")); to != "" {
		filter.To, err = time.Parse(time.RFC3339, to)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "to must be RFC3339")
			return
		}
	}

	entries, err := api.service.QueryAudit(r.Context(), filter, actorFromHeaders(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]domain.AuditEntry{"audit": entries})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Malformed JSON body")
		return false
	}
	return true
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		writeError(w, http.StatusBadRequest, "invalid_argument", publicHTTPMessage(err))
	case errors.Is(err, domain.ErrPermissionDenied):
		writeError(w, http.StatusForbidden, "permission_denied", "Access denied")
	case errors.Is(err, domain.ErrAlreadyExists):
		writeError(w, http.StatusConflict, "already_exists", "User already exists")
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource not found")
	case errors.Is(err, domain.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Dependency unavailable")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "Internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]map[string]string{
		"error": {
			"code":    code,
			"message": message,
		},
	})
}

func publicHTTPMessage(err error) string {
	message := err.Error()
	if message == "" {
		return "Invalid request"
	}
	if i := strings.Index(message, ": "); i >= 0 && i+2 < len(message) {
		return strings.TrimSpace(message[i+2:])
	}
	return message
}

func parsePositiveInt(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed < 0 {
		return 0
	}
	return parsed
}
