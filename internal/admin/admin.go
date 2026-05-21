package admin

import (
	"context"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
)

type Client interface {
	CreateUser(ctx context.Context, user domain.UserDetails) (domain.UserDetails, error)
	GetUserByEmail(ctx context.Context, email string) (domain.UserDetails, error)
	GetUserByID(ctx context.Context, userPool, userID string) (domain.UserDetails, error)
}
