package domain

import "time"

const DefaultUserPool = "USER_POOL_BRAND"

type Persona string

const PersonaSuperAdmin Persona = "super_admin"

type Actor struct {
	UserID   string    `json:"user_id"`
	Email    string    `json:"email"`
	Personas []Persona `json:"personas"`
}

func (a Actor) IsSuperAdmin() bool {
	for _, persona := range a.Personas {
		if persona == PersonaSuperAdmin {
			return true
		}
	}
	return false
}

type UserDetails struct {
	UserID   string `json:"user_id"`
	UserPool string `json:"user_pool"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Mobile   string `json:"mobile"`
}

type AccountUser struct {
	AccountID string       `json:"account_id"`
	UserID    string       `json:"user_id"`
	UserPool  string       `json:"user_pool"`
	Personas  []Persona    `json:"personas"`
	Details   *UserDetails `json:"user_details,omitempty"`
}

type CreateBrandUserInput struct {
	AccountID string
	Email     string
	Name      string
	Mobile    string
	Personas  []Persona
	Actor     Actor
	RequestID string
}

type UserOperationType string

const (
	OperationUpdatePersonas UserOperationType = "UPDATE_PERSONAS"
	OperationRemove         UserOperationType = "REMOVE"
)

type UserOperation struct {
	Operation UserOperationType
	UserID    string
	UserPool  string
	Personas  []Persona
}

type OperationResult struct {
	UserID  string `json:"user_id"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type AuditOperation string

const (
	AuditCreate         AuditOperation = "CREATE"
	AuditUpdatePersonas AuditOperation = "UPDATE_PERSONAS"
	AuditRemove         AuditOperation = "REMOVE"
)

type AuditEntry struct {
	ID            string         `json:"id"`
	AccountID     string         `json:"account_id"`
	ActorID       string         `json:"actor_id"`
	ActorEmail    string         `json:"actor_email"`
	TargetUserID  string         `json:"target_user_id"`
	TargetEmail   string         `json:"target_email"`
	TargetName    string         `json:"target_name,omitempty"`
	TargetMobile  string         `json:"target_mobile,omitempty"`
	Operation     AuditOperation `json:"operation"`
	OldPersonas   []Persona      `json:"old_personas,omitempty"`
	NewPersonas   []Persona      `json:"new_personas,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	RetentionDays int            `json:"retention_days"`
}

type AuditFilter struct {
	AccountID      string
	ActorID        string
	TargetUserID   string
	Operation      AuditOperation
	From           time.Time
	To             time.Time
	RetentionDays  int
	ReferenceClock time.Time
}
