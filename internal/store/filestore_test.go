package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
)

func TestFileStoreCreatesUsersAndRejectsDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore(t)

	created, err := store.CreateUser(ctx, domain.UserDetails{
		Email:  "Admin@Example.com",
		Name:   "Asha Brand",
		Mobile: "+919876543210",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if created.UserID == "" {
		t.Fatal("expected generated user id")
	}
	if created.UserPool != domain.DefaultUserPool {
		t.Fatalf("expected default user pool, got %q", created.UserPool)
	}
	if created.Email != "admin@example.com" {
		t.Fatalf("expected normalized email, got %q", created.Email)
	}

	_, err = store.CreateUser(ctx, domain.UserDetails{
		Email:  "admin@example.com",
		Name:   "Duplicate",
		Mobile: "+919876543211",
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestFileStoreAccountUserLifecyclePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	user := domain.AccountUser{
		AccountID: "ACC123",
		UserID:    "USER_POOL_BRAND#123",
		UserPool:  domain.DefaultUserPool,
		Personas:  []domain.Persona{"ads_admin"},
	}
	if err := store.AddAccountUser(ctx, user); err != nil {
		t.Fatalf("add account user: %v", err)
	}

	old, updated, err := store.UpdateAccountUserPersonas(ctx, "ACC123", domain.DefaultUserPool, "USER_POOL_BRAND#123", []domain.Persona{"super_admin", "ads_admin"})
	if err != nil {
		t.Fatalf("update personas: %v", err)
	}
	if old.Personas[0] != "ads_admin" {
		t.Fatalf("expected old personas to be returned, got %#v", old.Personas)
	}
	if len(updated.Personas) != 2 {
		t.Fatalf("expected two personas, got %#v", updated.Personas)
	}

	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	users, err := reopened.ListAccountUsers(ctx, "ACC123")
	if err != nil {
		t.Fatalf("list account users: %v", err)
	}
	if len(users) != 1 || len(users[0].Personas) != 2 {
		t.Fatalf("unexpected users after reopen: %#v", users)
	}

	removed, err := reopened.RemoveAccountUser(ctx, "ACC123", domain.DefaultUserPool, "USER_POOL_BRAND#123")
	if err != nil {
		t.Fatalf("remove account user: %v", err)
	}
	if removed.UserID != "USER_POOL_BRAND#123" {
		t.Fatalf("expected removed user id, got %q", removed.UserID)
	}
	users, err = reopened.ListAccountUsers(ctx, "ACC123")
	if err != nil {
		t.Fatalf("list account users after remove: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty account after remove, got %#v", users)
	}
}

func TestFileStoreQueriesAuditWithFiltersAndRetention(t *testing.T) {
	ctx := context.Background()
	store := newTestFileStore(t)
	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)

	entries := []domain.AuditEntry{
		{ID: "old", AccountID: "ACC123", ActorID: "actor-1", TargetUserID: "user-1", Operation: domain.AuditCreate, CreatedAt: now.AddDate(-2, 0, 0)},
		{ID: "keep", AccountID: "ACC123", ActorID: "actor-1", TargetUserID: "user-2", Operation: domain.AuditUpdatePersonas, CreatedAt: now.Add(-time.Hour)},
		{ID: "other-account", AccountID: "ACC999", ActorID: "actor-1", TargetUserID: "user-2", Operation: domain.AuditUpdatePersonas, CreatedAt: now.Add(-time.Hour)},
	}
	for _, entry := range entries {
		if err := store.AppendAudit(ctx, entry); err != nil {
			t.Fatalf("append audit %s: %v", entry.ID, err)
		}
	}

	got, err := store.QueryAudit(ctx, domain.AuditFilter{
		AccountID:      "ACC123",
		ActorID:        "actor-1",
		RetentionDays:  365,
		ReferenceClock: now,
	})
	if err != nil {
		t.Fatalf("query audit: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one retained audit entry, got %#v", got)
	}
	if got[0].ID != "keep" {
		t.Fatalf("expected retained entry, got %q", got[0].ID)
	}
}

func newTestFileStore(t *testing.T) *FileStore {
	t.Helper()
	store, err := NewFileStore(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	return store
}
