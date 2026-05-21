package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/admin"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/service"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/store"
)

func TestCreateBrandUserEndpoint(t *testing.T) {
	handler, _ := newHTTPTestHandler(t)
	req := jsonRequest(t, http.MethodPost, "/api/v1/account/ACC123/users", map[string]any{
		"email":    "new@example.com",
		"name":     "New User",
		"mobile":   "+919876543210",
		"personas": []string{"ads_admin", "discount_analyst"},
	})
	addSuperAdminHeaders(req)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var response map[string]any
	decodeResponse(t, rr, &response)
	if response["user_id"] == "" {
		t.Fatalf("expected user_id response, got %#v", response)
	}
	if response["message"] != "User created successfully" {
		t.Fatalf("unexpected message %#v", response)
	}
}

func TestCreateBrandUserEndpointRequiresSuperAdmin(t *testing.T) {
	handler, _ := newHTTPTestHandler(t)
	req := jsonRequest(t, http.MethodPost, "/api/v1/account/ACC123/users", map[string]any{
		"email":    "new@example.com",
		"name":     "New User",
		"mobile":   "+919876543210",
		"personas": []string{"ads_admin"},
	})
	req.Header.Set("X-Actor-User-Id", "actor-1")
	req.Header.Set("X-Actor-Email", "actor@example.com")
	req.Header.Set("X-Actor-Personas", "ads_admin")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateUsersOrAccountEndpointUpdatesAndRemoves(t *testing.T) {
	handler, backingStore := newHTTPTestHandler(t)
	userOne := seedHTTPUser(t, backingStore, "ACC123", "one@example.com", []domain.Persona{"ads_admin"})
	userTwo := seedHTTPUser(t, backingStore, "ACC123", "two@example.com", []domain.Persona{"discount_analyst"})
	req := jsonRequest(t, http.MethodPost, "/api/v1/account/ACC123/users:batch", map[string]any{
		"user_operations": []map[string]any{
			{"operation": "UPDATE_PERSONAS", "user_id": userOne.UserID, "personas": []string{"super_admin", "ads_admin"}},
			{"operation": "REMOVE", "user_id": userTwo.UserID},
		},
	})
	addSuperAdminHeaders(req)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Results []domain.OperationResult `json:"results"`
	}
	decodeResponse(t, rr, &response)
	if len(response.Results) != 2 || !response.Results[0].Success || !response.Results[1].Success {
		t.Fatalf("unexpected results: %#v", response.Results)
	}
}

func TestGetUsersOfAccountEndpointReturnsDetails(t *testing.T) {
	handler, backingStore := newHTTPTestHandler(t)
	seedHTTPUser(t, backingStore, "ACC123", "one@example.com", []domain.Persona{"ads_admin"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/ACC123/users", nil)
	addSuperAdminHeaders(req)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Users []domain.AccountUser `json:"users"`
	}
	decodeResponse(t, rr, &response)
	if len(response.Users) != 1 {
		t.Fatalf("expected one user, got %#v", response.Users)
	}
	if response.Users[0].Details == nil || response.Users[0].Details.Email != "one@example.com" {
		t.Fatalf("expected user details, got %#v", response.Users[0].Details)
	}
}

func TestAuditEndpointFiltersByActor(t *testing.T) {
	handler, _ := newHTTPTestHandler(t)
	create := jsonRequest(t, http.MethodPost, "/api/v1/account/ACC123/users", map[string]any{
		"email":    "new@example.com",
		"name":     "New User",
		"mobile":   "+919876543210",
		"personas": []string{"ads_admin"},
	})
	addSuperAdminHeaders(create)
	httptest.NewRecorder().Result()
	rrCreate := httptest.NewRecorder()
	handler.ServeHTTP(rrCreate, create)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("seed create failed: %d %s", rrCreate.Code, rrCreate.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/ACC123/audit?actor_id=actor-1", nil)
	addSuperAdminHeaders(req)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Audit []domain.AuditEntry `json:"audit"`
	}
	decodeResponse(t, rr, &response)
	if len(response.Audit) != 1 || response.Audit[0].Operation != domain.AuditCreate {
		t.Fatalf("expected create audit, got %#v", response.Audit)
	}
}

func TestMalformedJSONReturnsBadRequest(t *testing.T) {
	handler, _ := newHTTPTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/account/ACC123/users", bytes.NewBufferString("{"))
	addSuperAdminHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMethodNotAllowed(t *testing.T) {
	handler, _ := newHTTPTestHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/account/ACC123/users", nil)
	addSuperAdminHeaders(req)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", rr.Code, rr.Body.String())
	}
}

func newHTTPTestHandler(t *testing.T) (http.Handler, *store.FileStore) {
	t.Helper()
	backingStore, err := store.NewFileStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	svc := service.New(backingStore, backingStore, admin.NewFileClient(backingStore), service.WithClock(fixedHTTPClock), service.WithAuditRetentionDays(365))
	return NewRouter(svc, nil), backingStore
}

func seedHTTPUser(t *testing.T, backingStore *store.FileStore, accountID, email string, personas []domain.Persona) domain.UserDetails {
	t.Helper()
	ctx := context.Background()
	user, err := backingStore.CreateUser(ctx, domain.UserDetails{Email: email, Name: "Seed User", Mobile: "+919876543210"})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := backingStore.AddAccountUser(ctx, domain.AccountUser{AccountID: accountID, UserID: user.UserID, UserPool: user.UserPool, Personas: personas}); err != nil {
		t.Fatalf("seed account user: %v", err)
	}
	return user
}

func jsonRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	bytesBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(bytesBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func decodeResponse(t *testing.T, rr *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func addSuperAdminHeaders(req *http.Request) {
	req.Header.Set("X-Actor-User-Id", "actor-1")
	req.Header.Set("X-Actor-Email", "actor@example.com")
	req.Header.Set("X-Actor-Personas", "super_admin,ads_admin")
	req.Header.Set("X-Request-Id", "req-http")
}

func fixedHTTPClock() time.Time {
	return time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
}
