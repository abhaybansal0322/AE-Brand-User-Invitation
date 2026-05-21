package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/admin"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/store"
)

type Service struct {
	accounts      store.AccountRepository
	audits        store.AuditRepository
	admin         admin.Client
	clock         func() time.Time
	retentionDays int
	detailTimeout time.Duration
}

type CreateBrandUserResult struct {
	User    domain.UserDetails `json:"user"`
	Message string             `json:"message"`
}

type Option func(*Service)

func New(accounts store.AccountRepository, audits store.AuditRepository, adminClient admin.Client, opts ...Option) *Service {
	s := &Service{
		accounts:      accounts,
		audits:        audits,
		admin:         adminClient,
		clock:         func() time.Time { return time.Now().UTC() },
		retentionDays: 365,
		detailTimeout: time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithClock(clock func() time.Time) Option {
	return func(s *Service) {
		if clock != nil {
			s.clock = clock
		}
	}
}

func WithAuditRetentionDays(days int) Option {
	return func(s *Service) {
		if days > 0 {
			s.retentionDays = days
		}
	}
}

func WithDetailTimeout(timeout time.Duration) Option {
	return func(s *Service) {
		if timeout > 0 {
			s.detailTimeout = timeout
		}
	}
}

func (s *Service) CreateBrandUser(ctx context.Context, input domain.CreateBrandUserInput) (CreateBrandUserResult, error) {
	if err := requireSuperAdmin(input.Actor); err != nil {
		return CreateBrandUserResult{}, err
	}
	if err := domain.ValidateCreateBrandUserInput(input); err != nil {
		return CreateBrandUserResult{}, err
	}
	personas, err := domain.NormalizePersonas(input.Personas)
	if err != nil {
		return CreateBrandUserResult{}, err
	}

	user, err := s.admin.CreateUser(ctx, domain.UserDetails{
		Email:  strings.ToLower(strings.TrimSpace(input.Email)),
		Name:   strings.TrimSpace(input.Name),
		Mobile: strings.TrimSpace(input.Mobile),
	})
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return s.handleExistingEmail(ctx, input, personas)
		}
		return CreateBrandUserResult{}, fmt.Errorf("%w: failed to create user in admin api", err)
	}

	accountUser := domain.AccountUser{
		AccountID: input.AccountID,
		UserID:    user.UserID,
		UserPool:  user.UserPool,
		Personas:  personas,
	}
	if err := s.accounts.AddAccountUser(ctx, accountUser); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return CreateBrandUserResult{User: user, Message: "User already exists in account"}, nil
		}
		return CreateBrandUserResult{}, err
	}
	if err := s.appendAudit(ctx, domain.AuditEntry{
		AccountID:     input.AccountID,
		ActorID:       input.Actor.UserID,
		ActorEmail:    input.Actor.Email,
		TargetUserID:  user.UserID,
		TargetEmail:   user.Email,
		TargetName:    user.Name,
		TargetMobile:  user.Mobile,
		Operation:     domain.AuditCreate,
		NewPersonas:   personas,
		RequestID:     input.RequestID,
		RetentionDays: s.retentionDays,
	}); err != nil {
		return CreateBrandUserResult{}, err
	}

	return CreateBrandUserResult{User: user, Message: "User created successfully"}, nil
}

func (s *Service) ApplyUserOperations(ctx context.Context, accountID string, actor domain.Actor, operations []domain.UserOperation, requestID string) ([]domain.OperationResult, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return nil, err
	}
	if strings.TrimSpace(accountID) == "" {
		return nil, fmt.Errorf("%w: account_id is required", domain.ErrInvalidArgument)
	}
	if len(operations) == 0 {
		return nil, fmt.Errorf("%w: user_operations is required", domain.ErrInvalidArgument)
	}

	results := make([]domain.OperationResult, 0, len(operations))
	for _, operation := range operations {
		switch operation.Operation {
		case domain.OperationUpdatePersonas:
			results = append(results, s.updateUserPersonas(ctx, accountID, actor, operation, requestID))
		case domain.OperationRemove:
			results = append(results, s.removeUser(ctx, accountID, actor, operation, requestID))
		default:
			results = append(results, domain.OperationResult{UserID: operation.UserID, Success: false, Message: "Unsupported operation"})
		}
	}
	return results, nil
}

func (s *Service) ListUsers(ctx context.Context, accountID string, actor domain.Actor) ([]domain.AccountUser, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return nil, err
	}
	if strings.TrimSpace(accountID) == "" {
		return nil, fmt.Errorf("%w: account_id is required", domain.ErrInvalidArgument)
	}

	users, err := s.accounts.ListAccountUsers(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return users, nil
	}

	detailCtx, cancel := context.WithTimeout(ctx, s.detailTimeout)
	defer cancel()

	var wg sync.WaitGroup
	for i := range users {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			details, err := s.admin.GetUserByID(detailCtx, users[index].UserPool, users[index].UserID)
			if err != nil {
				return
			}
			users[index].Details = &details
		}(i)
	}
	wg.Wait()
	return users, nil
}

func (s *Service) QueryAudit(ctx context.Context, filter domain.AuditFilter, actor domain.Actor) ([]domain.AuditEntry, error) {
	if err := requireSuperAdmin(actor); err != nil {
		return nil, err
	}
	if strings.TrimSpace(filter.AccountID) == "" {
		return nil, fmt.Errorf("%w: account_id is required", domain.ErrInvalidArgument)
	}
	if filter.RetentionDays == 0 {
		filter.RetentionDays = s.retentionDays
	}
	if filter.ReferenceClock.IsZero() {
		filter.ReferenceClock = s.clock()
	}
	return s.audits.QueryAudit(ctx, filter)
}

func (s *Service) handleExistingEmail(ctx context.Context, input domain.CreateBrandUserInput, personas []domain.Persona) (CreateBrandUserResult, error) {
	existing, err := s.admin.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return CreateBrandUserResult{}, fmt.Errorf("%w: user with this email already exists", domain.ErrAlreadyExists)
	}
	if _, err := s.accounts.GetAccountUser(ctx, input.AccountID, existing.UserPool, existing.UserID); err == nil {
		return CreateBrandUserResult{User: existing, Message: "User already exists in account"}, nil
	}

	_ = personas
	return CreateBrandUserResult{}, fmt.Errorf("%w: user with this email already exists", domain.ErrAlreadyExists)
}

func (s *Service) updateUserPersonas(ctx context.Context, accountID string, actor domain.Actor, operation domain.UserOperation, requestID string) domain.OperationResult {
	if strings.TrimSpace(operation.UserID) == "" {
		return domain.OperationResult{Success: false, Message: "user_id is required"}
	}
	oldUser, updatedUser, err := s.accounts.UpdateAccountUserPersonas(ctx, accountID, operation.UserPool, operation.UserID, operation.Personas)
	if err != nil {
		return domain.OperationResult{UserID: operation.UserID, Success: false, Message: publicMessage(err)}
	}

	details, _ := s.admin.GetUserByID(ctx, updatedUser.UserPool, updatedUser.UserID)
	if err := s.appendAudit(ctx, domain.AuditEntry{
		AccountID:     accountID,
		ActorID:       actor.UserID,
		ActorEmail:    actor.Email,
		TargetUserID:  updatedUser.UserID,
		TargetEmail:   details.Email,
		TargetName:    details.Name,
		TargetMobile:  details.Mobile,
		Operation:     domain.AuditUpdatePersonas,
		OldPersonas:   oldUser.Personas,
		NewPersonas:   updatedUser.Personas,
		RequestID:     requestID,
		RetentionDays: s.retentionDays,
	}); err != nil {
		return domain.OperationResult{UserID: operation.UserID, Success: false, Message: publicMessage(err)}
	}
	return domain.OperationResult{UserID: operation.UserID, Success: true, Message: "Personas updated"}
}

func (s *Service) removeUser(ctx context.Context, accountID string, actor domain.Actor, operation domain.UserOperation, requestID string) domain.OperationResult {
	if strings.TrimSpace(operation.UserID) == "" {
		return domain.OperationResult{Success: false, Message: "user_id is required"}
	}
	removedUser, err := s.accounts.RemoveAccountUser(ctx, accountID, operation.UserPool, operation.UserID)
	if err != nil {
		return domain.OperationResult{UserID: operation.UserID, Success: false, Message: publicMessage(err)}
	}

	details, _ := s.admin.GetUserByID(ctx, removedUser.UserPool, removedUser.UserID)
	if err := s.appendAudit(ctx, domain.AuditEntry{
		AccountID:     accountID,
		ActorID:       actor.UserID,
		ActorEmail:    actor.Email,
		TargetUserID:  removedUser.UserID,
		TargetEmail:   details.Email,
		TargetName:    details.Name,
		TargetMobile:  details.Mobile,
		Operation:     domain.AuditRemove,
		OldPersonas:   removedUser.Personas,
		RequestID:     requestID,
		RetentionDays: s.retentionDays,
	}); err != nil {
		return domain.OperationResult{UserID: operation.UserID, Success: false, Message: publicMessage(err)}
	}
	return domain.OperationResult{UserID: operation.UserID, Success: true, Message: "User removed from account"}
}

func (s *Service) appendAudit(ctx context.Context, entry domain.AuditEntry) error {
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = s.clock()
	}
	return s.audits.AppendAudit(ctx, entry)
}

func requireSuperAdmin(actor domain.Actor) error {
	if !actor.IsSuperAdmin() {
		return fmt.Errorf("%w: super_admin role required", domain.ErrPermissionDenied)
	}
	return nil
}

func publicMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidArgument):
		return "Invalid request"
	case errors.Is(err, domain.ErrNotFound):
		return "User not found"
	case errors.Is(err, domain.ErrAlreadyExists):
		return "User already exists"
	case errors.Is(err, domain.ErrPermissionDenied):
		return "Access denied"
	case errors.Is(err, domain.ErrUnavailable):
		return "Dependency unavailable"
	default:
		return "Operation failed"
	}
}
