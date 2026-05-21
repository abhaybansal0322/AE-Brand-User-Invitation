package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/admin"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/store"
)

func TestCreateBrandUserCreatesAccountUserAndAudit(t *testing.T) {
	ctx := context.Background()
	svc, backingStore := newTestService(t)

	result, err := svc.CreateBrandUser(ctx, domain.CreateBrandUserInput{
		AccountID: "ACC123",
		Email:     "new@example.com",
		Name:      "New User",
		Mobile:    "+919876543210",
		Personas:  []domain.Persona{"ads_admin", "discount_analyst"},
		Actor:     superAdminActor(),
		RequestID: "req-1",
	})
	if err != nil {
		t.Fatalf("create brand user: %v", err)
	}
	if result.User.UserID == "" {
		t.Fatal("expected user id")
	}
	if result.Message != "User created successfully" {
		t.Fatalf("unexpected message %q", result.Message)
	}

	accountUsers, err := backingStore.ListAccountUsers(ctx, "ACC123")
	if err != nil {
		t.Fatalf("list account users: %v", err)
	}
	if len(accountUsers) != 1 {
		t.Fatalf("expected one account user, got %#v", accountUsers)
	}
	if len(accountUsers[0].Personas) != 2 {
		t.Fatalf("expected two personas, got %#v", accountUsers[0].Personas)
	}

	audit, err := backingStore.QueryAudit(ctx, domain.AuditFilter{AccountID: "ACC123", ReferenceClock: fixedClock(), RetentionDays: 365})
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(audit) != 1 || audit[0].Operation != domain.AuditCreate {
		t.Fatalf("expected create audit entry, got %#v", audit)
	}
}

func TestCreateBrandUserReturnsConflictWhenEmailExistsOutsideAccount(t *testing.T) {
	ctx := context.Background()
	svc, backingStore := newTestService(t)
	_, err := backingStore.CreateUser(ctx, domain.UserDetails{
		Email:  "existing@example.com",
		Name:   "Existing User",
		Mobile: "+919876543210",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	_, err = svc.CreateBrandUser(ctx, domain.CreateBrandUserInput{
		AccountID: "ACC123",
		Email:     "existing@example.com",
		Name:      "Existing User",
		Mobile:    "+919876543210",
		Personas:  []domain.Persona{"ads_admin"},
		Actor:     superAdminActor(),
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestCreateBrandUserIsIdempotentWhenExistingEmailAlreadyBelongsToAccount(t *testing.T) {
	ctx := context.Background()
	svc, backingStore := newTestService(t)
	user, err := backingStore.CreateUser(ctx, domain.UserDetails{
		Email:  "existing@example.com",
		Name:   "Existing User",
		Mobile: "+919876543210",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := backingStore.AddAccountUser(ctx, domain.AccountUser{
		AccountID: "ACC123",
		UserID:    user.UserID,
		UserPool:  user.UserPool,
		Personas:  []domain.Persona{"ads_admin"},
	}); err != nil {
		t.Fatalf("seed account user: %v", err)
	}

	result, err := svc.CreateBrandUser(ctx, domain.CreateBrandUserInput{
		AccountID: "ACC123",
		Email:     "existing@example.com",
		Name:      "Existing User",
		Mobile:    "+919876543210",
		Personas:  []domain.Persona{"ads_admin"},
		Actor:     superAdminActor(),
	})
	if err != nil {
		t.Fatalf("create existing account user should be idempotent: %v", err)
	}
	if result.User.UserID != user.UserID {
		t.Fatalf("expected existing user id %q, got %q", user.UserID, result.User.UserID)
	}
	if result.Message != "User already exists in account" {
		t.Fatalf("unexpected message %q", result.Message)
	}
}

func TestCreateBrandUserRequiresSuperAdmin(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestService(t)

	_, err := svc.CreateBrandUser(ctx, domain.CreateBrandUserInput{
		AccountID: "ACC123",
		Email:     "new@example.com",
		Name:      "New User",
		Mobile:    "+919876543210",
		Personas:  []domain.Persona{"ads_admin"},
		Actor:     domain.Actor{UserID: "actor-1", Email: "actor@example.com", Personas: []domain.Persona{"ads_admin"}},
	})
	if !errors.Is(err, domain.ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestApplyUserOperationsUpdatesPersonasAndRemovesUsersWithAudit(t *testing.T) {
	ctx := context.Background()
	svc, backingStore := newTestService(t)
	userOne := seedAccountUser(t, ctx, backingStore, "ACC123", "one@example.com", []domain.Persona{"ads_admin"})
	userTwo := seedAccountUser(t, ctx, backingStore, "ACC123", "two@example.com", []domain.Persona{"discount_analyst"})

	results, err := svc.ApplyUserOperations(ctx, "ACC123", superAdminActor(), []domain.UserOperation{
		{Operation: domain.OperationUpdatePersonas, UserID: userOne.UserID, UserPool: userOne.UserPool, Personas: []domain.Persona{"super_admin", "ads_admin"}},
		{Operation: domain.OperationRemove, UserID: userTwo.UserID, UserPool: userTwo.UserPool},
	}, "req-ops")
	if err != nil {
		t.Fatalf("apply operations: %v", err)
	}
	if len(results) != 2 || !results[0].Success || !results[1].Success {
		t.Fatalf("unexpected operation results: %#v", results)
	}

	updated, err := backingStore.GetAccountUser(ctx, "ACC123", userOne.UserPool, userOne.UserID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if len(updated.Personas) != 2 || updated.Personas[0] != "super_admin" {
		t.Fatalf("expected updated personas, got %#v", updated.Personas)
	}
	_, err = backingStore.GetAccountUser(ctx, "ACC123", userTwo.UserPool, userTwo.UserID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected removed user to be missing, got %v", err)
	}

	audit, err := backingStore.QueryAudit(ctx, domain.AuditFilter{AccountID: "ACC123", ReferenceClock: fixedClock(), RetentionDays: 365})
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(audit) != 2 {
		t.Fatalf("expected two audit entries, got %#v", audit)
	}
}

func TestListUsersReturnsAccountUsersWhenAdminDetailsFail(t *testing.T) {
	ctx := context.Background()
	backingStore := newTestStore(t)
	if err := backingStore.AddAccountUser(ctx, domain.AccountUser{
		AccountID: "ACC123",
		UserID:    "USER_POOL_BRAND#missing",
		UserPool:  domain.DefaultUserPool,
		Personas:  []domain.Persona{"ads_admin"},
	}); err != nil {
		t.Fatalf("seed account user: %v", err)
	}
	svc := New(backingStore, backingStore, failingAdminClient{}, WithClock(fixedClock), WithAuditRetentionDays(365))

	users, err := svc.ListUsers(ctx, "ACC123", superAdminActor())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected one user, got %#v", users)
	}
	if users[0].Details != nil {
		t.Fatalf("expected missing details fallback, got %#v", users[0].Details)
	}
}

type failingAdminClient struct{}

func (f failingAdminClient) CreateUser(context.Context, domain.UserDetails) (domain.UserDetails, error) {
	return domain.UserDetails{}, domain.ErrUnavailable
}

func (f failingAdminClient) GetUserByEmail(context.Context, string) (domain.UserDetails, error) {
	return domain.UserDetails{}, domain.ErrUnavailable
}

func (f failingAdminClient) GetUserByID(context.Context, string, string) (domain.UserDetails, error) {
	return domain.UserDetails{}, domain.ErrUnavailable
}

func newTestService(t *testing.T) (*Service, *store.FileStore) {
	t.Helper()
	backingStore := newTestStore(t)
	return New(backingStore, backingStore, admin.NewFileClient(backingStore), WithClock(fixedClock), WithAuditRetentionDays(365)), backingStore
}

func newTestStore(t *testing.T) *store.FileStore {
	t.Helper()
	backingStore, err := store.NewFileStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("new file store: %v", err)
	}
	return backingStore
}

func seedAccountUser(t *testing.T, ctx context.Context, backingStore *store.FileStore, accountID, email string, personas []domain.Persona) domain.UserDetails {
	t.Helper()
	user, err := backingStore.CreateUser(ctx, domain.UserDetails{
		Email:  email,
		Name:   "Seed User",
		Mobile: "+919876543210",
	})
	if err != nil {
		t.Fatalf("seed admin user: %v", err)
	}
	if err := backingStore.AddAccountUser(ctx, domain.AccountUser{
		AccountID: accountID,
		UserID:    user.UserID,
		UserPool:  user.UserPool,
		Personas:  personas,
	}); err != nil {
		t.Fatalf("seed account user: %v", err)
	}
	return user
}

func superAdminActor() domain.Actor {
	return domain.Actor{UserID: "actor-1", Email: "actor@example.com", Personas: []domain.Persona{"super_admin"}}
}

func fixedClock() time.Time {
	return time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
}
