package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
)

const defaultAuditRetentionDays = 365

type FileStore struct {
	path string
	mu   sync.RWMutex
	data fileData
}

type fileData struct {
	Accounts       map[string]map[string]domain.AccountUser `json:"accounts"`
	Users          map[string]domain.UserDetails            `json:"users"`
	UserEmailIndex map[string]string                        `json:"user_email_index"`
	Audit          []domain.AuditEntry                      `json:"audit"`
}

func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("%w: data file path is required", domain.ErrInvalidArgument)
	}
	store := &FileStore{path: path}
	store.initLocked()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read data file: %w", err)
	}
	if len(bytes) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(bytes, &store.data); err != nil {
		return nil, fmt.Errorf("decode data file: %w", err)
	}
	store.initLocked()
	return store, nil
}

func (s *FileStore) CreateUser(ctx context.Context, user domain.UserDetails) (domain.UserDetails, error) {
	if err := ctx.Err(); err != nil {
		return domain.UserDetails{}, err
	}
	if err := domain.ValidateEmail(user.Email); err != nil {
		return domain.UserDetails{}, err
	}
	if err := domain.ValidateMobile(user.Mobile); err != nil {
		return domain.UserDetails{}, err
	}
	if strings.TrimSpace(user.Name) == "" {
		return domain.UserDetails{}, fmt.Errorf("%w: name is required", domain.ErrInvalidArgument)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.initLocked()
	user.Email = normalizeEmail(user.Email)
	if _, ok := s.data.UserEmailIndex[user.Email]; ok {
		return domain.UserDetails{}, fmt.Errorf("%w: email already exists", domain.ErrAlreadyExists)
	}
	if user.UserPool == "" {
		user.UserPool = domain.DefaultUserPool
	}
	if user.UserID == "" {
		user.UserID = "USER_POOL_BRAND#" + randomHex(12)
	}
	key := userKey(user.UserPool, user.UserID)
	s.data.Users[key] = user
	s.data.UserEmailIndex[user.Email] = key
	if err := s.saveLocked(); err != nil {
		return domain.UserDetails{}, err
	}
	return user, nil
}

func (s *FileStore) GetUserByEmail(ctx context.Context, email string) (domain.UserDetails, error) {
	if err := ctx.Err(); err != nil {
		return domain.UserDetails{}, err
	}
	email = normalizeEmail(email)

	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.data.UserEmailIndex[email]
	if !ok {
		return domain.UserDetails{}, fmt.Errorf("%w: user not found", domain.ErrNotFound)
	}
	user, ok := s.data.Users[key]
	if !ok {
		return domain.UserDetails{}, fmt.Errorf("%w: user not found", domain.ErrNotFound)
	}
	return user, nil
}

func (s *FileStore) GetUserByID(ctx context.Context, userPool, userID string) (domain.UserDetails, error) {
	if err := ctx.Err(); err != nil {
		return domain.UserDetails{}, err
	}
	if userPool == "" {
		userPool = domain.DefaultUserPool
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.data.Users[userKey(userPool, userID)]
	if !ok {
		return domain.UserDetails{}, fmt.Errorf("%w: user not found", domain.ErrNotFound)
	}
	return user, nil
}

func (s *FileStore) AddAccountUser(ctx context.Context, user domain.AccountUser) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(user.AccountID) == "" {
		return fmt.Errorf("%w: account_id is required", domain.ErrInvalidArgument)
	}
	if strings.TrimSpace(user.UserID) == "" {
		return fmt.Errorf("%w: user_id is required", domain.ErrInvalidArgument)
	}
	personas, err := domain.NormalizePersonas(user.Personas)
	if err != nil {
		return err
	}
	if user.UserPool == "" {
		user.UserPool = domain.DefaultUserPool
	}
	user.Personas = personas

	s.mu.Lock()
	defer s.mu.Unlock()

	s.initLocked()
	if _, ok := s.data.Accounts[user.AccountID]; !ok {
		s.data.Accounts[user.AccountID] = make(map[string]domain.AccountUser)
	}
	key := userKey(user.UserPool, user.UserID)
	if _, ok := s.data.Accounts[user.AccountID][key]; ok {
		return fmt.Errorf("%w: account user already exists", domain.ErrAlreadyExists)
	}
	s.data.Accounts[user.AccountID][key] = cloneAccountUser(user)
	return s.saveLocked()
}

func (s *FileStore) GetAccountUser(ctx context.Context, accountID, userPool, userID string) (domain.AccountUser, error) {
	if err := ctx.Err(); err != nil {
		return domain.AccountUser{}, err
	}
	if userPool == "" {
		userPool = domain.DefaultUserPool
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.data.Accounts[accountID][userKey(userPool, userID)]
	if !ok {
		return domain.AccountUser{}, fmt.Errorf("%w: account user not found", domain.ErrNotFound)
	}
	return cloneAccountUser(user), nil
}

func (s *FileStore) ListAccountUsers(ctx context.Context, accountID string) ([]domain.AccountUser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	accountUsers := s.data.Accounts[accountID]
	users := make([]domain.AccountUser, 0, len(accountUsers))
	for _, user := range accountUsers {
		users = append(users, cloneAccountUser(user))
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].UserID < users[j].UserID
	})
	return users, nil
}

func (s *FileStore) UpdateAccountUserPersonas(ctx context.Context, accountID, userPool, userID string, personas []domain.Persona) (domain.AccountUser, domain.AccountUser, error) {
	if err := ctx.Err(); err != nil {
		return domain.AccountUser{}, domain.AccountUser{}, err
	}
	normalized, err := domain.NormalizePersonas(personas)
	if err != nil {
		return domain.AccountUser{}, domain.AccountUser{}, err
	}
	if userPool == "" {
		userPool = domain.DefaultUserPool
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := userKey(userPool, userID)
	user, ok := s.data.Accounts[accountID][key]
	if !ok {
		return domain.AccountUser{}, domain.AccountUser{}, fmt.Errorf("%w: account user not found", domain.ErrNotFound)
	}
	oldUser := cloneAccountUser(user)
	user.Personas = normalized
	s.data.Accounts[accountID][key] = user
	if err := s.saveLocked(); err != nil {
		return domain.AccountUser{}, domain.AccountUser{}, err
	}
	return oldUser, cloneAccountUser(user), nil
}

func (s *FileStore) RemoveAccountUser(ctx context.Context, accountID, userPool, userID string) (domain.AccountUser, error) {
	if err := ctx.Err(); err != nil {
		return domain.AccountUser{}, err
	}
	if userPool == "" {
		userPool = domain.DefaultUserPool
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := userKey(userPool, userID)
	user, ok := s.data.Accounts[accountID][key]
	if !ok {
		return domain.AccountUser{}, fmt.Errorf("%w: account user not found", domain.ErrNotFound)
	}
	delete(s.data.Accounts[accountID], key)
	if len(s.data.Accounts[accountID]) == 0 {
		delete(s.data.Accounts, accountID)
	}
	if err := s.saveLocked(); err != nil {
		return domain.AccountUser{}, err
	}
	return cloneAccountUser(user), nil
}

func (s *FileStore) AppendAudit(ctx context.Context, entry domain.AuditEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(entry.AccountID) == "" {
		return fmt.Errorf("%w: audit account_id is required", domain.ErrInvalidArgument)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		entry.ID = "audit_" + randomHex(12)
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if entry.RetentionDays == 0 {
		entry.RetentionDays = defaultAuditRetentionDays
	}
	s.data.Audit = append(s.data.Audit, cloneAuditEntry(entry))
	return s.saveLocked()
}

func (s *FileStore) QueryAudit(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	retentionDays := filter.RetentionDays
	if retentionDays == 0 {
		retentionDays = defaultAuditRetentionDays
	}
	referenceClock := filter.ReferenceClock
	if referenceClock.IsZero() {
		referenceClock = time.Now().UTC()
	}
	cutoff := referenceClock.AddDate(0, 0, -retentionDays)

	entries := make([]domain.AuditEntry, 0, len(s.data.Audit))
	for _, entry := range s.data.Audit {
		if filter.AccountID != "" && entry.AccountID != filter.AccountID {
			continue
		}
		if filter.ActorID != "" && entry.ActorID != filter.ActorID {
			continue
		}
		if filter.TargetUserID != "" && entry.TargetUserID != filter.TargetUserID {
			continue
		}
		if filter.Operation != "" && entry.Operation != filter.Operation {
			continue
		}
		if !filter.From.IsZero() && entry.CreatedAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && entry.CreatedAt.After(filter.To) {
			continue
		}
		if entry.CreatedAt.Before(cutoff) {
			continue
		}
		entries = append(entries, cloneAuditEntry(entry))
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.After(entries[j].CreatedAt)
	})
	return entries, nil
}

func (s *FileStore) initLocked() {
	if s.data.Accounts == nil {
		s.data.Accounts = make(map[string]map[string]domain.AccountUser)
	}
	if s.data.Users == nil {
		s.data.Users = make(map[string]domain.UserDetails)
	}
	if s.data.UserEmailIndex == nil {
		s.data.UserEmailIndex = make(map[string]string)
	}
}

func (s *FileStore) saveLocked() error {
	bytes, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode data file: %w", err)
	}
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, bytes, 0o600); err != nil {
		return fmt.Errorf("write data file: %w", err)
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replace data file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace data file: %w", err)
	}
	return nil
}

func userKey(userPool, userID string) string {
	if userPool == "" {
		userPool = domain.DefaultUserPool
	}
	return userPool + "|" + userID
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func randomHex(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func cloneAccountUser(user domain.AccountUser) domain.AccountUser {
	user.Personas = append([]domain.Persona(nil), user.Personas...)
	if user.Details != nil {
		details := *user.Details
		user.Details = &details
	}
	return user
}

func cloneAuditEntry(entry domain.AuditEntry) domain.AuditEntry {
	entry.OldPersonas = append([]domain.Persona(nil), entry.OldPersonas...)
	entry.NewPersonas = append([]domain.Persona(nil), entry.NewPersonas...)
	return entry
}
