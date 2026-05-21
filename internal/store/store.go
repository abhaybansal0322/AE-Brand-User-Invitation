package store

import (
	"context"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
)

type AccountRepository interface {
	AddAccountUser(ctx context.Context, user domain.AccountUser) error
	GetAccountUser(ctx context.Context, accountID, userPool, userID string) (domain.AccountUser, error)
	ListAccountUsers(ctx context.Context, accountID string) ([]domain.AccountUser, error)
	UpdateAccountUserPersonas(ctx context.Context, accountID, userPool, userID string, personas []domain.Persona) (domain.AccountUser, domain.AccountUser, error)
	RemoveAccountUser(ctx context.Context, accountID, userPool, userID string) (domain.AccountUser, error)
}

type UserRepository interface {
	CreateUser(ctx context.Context, user domain.UserDetails) (domain.UserDetails, error)
	GetUserByEmail(ctx context.Context, email string) (domain.UserDetails, error)
	GetUserByID(ctx context.Context, userPool, userID string) (domain.UserDetails, error)
}

type AuditRepository interface {
	AppendAudit(ctx context.Context, entry domain.AuditEntry) error
	QueryAudit(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, error)
}
