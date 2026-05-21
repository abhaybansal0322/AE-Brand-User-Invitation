package admin

import (
	"context"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/store"
)

type FileClient struct {
	users store.UserRepository
}

func NewFileClient(users store.UserRepository) *FileClient {
	return &FileClient{users: users}
}

func (c *FileClient) CreateUser(ctx context.Context, user domain.UserDetails) (domain.UserDetails, error) {
	return c.users.CreateUser(ctx, user)
}

func (c *FileClient) GetUserByEmail(ctx context.Context, email string) (domain.UserDetails, error) {
	return c.users.GetUserByEmail(ctx, email)
}

func (c *FileClient) GetUserByID(ctx context.Context, userPool, userID string) (domain.UserDetails, error) {
	return c.users.GetUserByID(ctx, userPool, userID)
}
