package httpapi

import (
	"net/http"
	"strings"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
)

func actorFromHeaders(r *http.Request) domain.Actor {
	personaParts := strings.Split(r.Header.Get("X-Actor-Personas"), ",")
	personas := make([]domain.Persona, 0, len(personaParts))
	for _, part := range personaParts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}
		personas = append(personas, domain.Persona(part))
	}
	return domain.Actor{
		UserID:   strings.TrimSpace(r.Header.Get("X-Actor-User-Id")),
		Email:    strings.TrimSpace(r.Header.Get("X-Actor-Email")),
		Personas: personas,
	}
}

func requestIDFromHeaders(r *http.Request) string {
	if requestID := strings.TrimSpace(r.Header.Get("X-Request-Id")); requestID != "" {
		return requestID
	}
	return strings.TrimSpace(r.Header.Get("X-Correlation-Id"))
}
