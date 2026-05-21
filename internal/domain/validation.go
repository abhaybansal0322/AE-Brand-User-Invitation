package domain

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
)

var (
	personaPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
	mobilePattern  = regexp.MustCompile(`^\+?[0-9][0-9\s-]{6,18}$`)
)

func ValidateCreateBrandUserInput(input CreateBrandUserInput) error {
	if strings.TrimSpace(input.AccountID) == "" {
		return fmt.Errorf("%w: account_id is required", ErrInvalidArgument)
	}
	if err := ValidateEmail(input.Email); err != nil {
		return err
	}
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidArgument)
	}
	if err := ValidateMobile(input.Mobile); err != nil {
		return err
	}
	if _, err := NormalizePersonas(input.Personas); err != nil {
		return err
	}
	return nil
}

func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("%w: email is required", ErrInvalidArgument)
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return fmt.Errorf("%w: invalid email", ErrInvalidArgument)
	}
	return nil
}

func ValidateMobile(mobile string) error {
	mobile = strings.TrimSpace(mobile)
	if mobile == "" {
		return fmt.Errorf("%w: mobile is required", ErrInvalidArgument)
	}
	if !mobilePattern.MatchString(mobile) {
		return fmt.Errorf("%w: invalid mobile", ErrInvalidArgument)
	}
	return nil
}

func NormalizePersonas(personas []Persona) ([]Persona, error) {
	if len(personas) == 0 {
		return nil, fmt.Errorf("%w: at least one persona required", ErrInvalidArgument)
	}

	seen := make(map[Persona]bool, len(personas))
	normalized := make([]Persona, 0, len(personas))
	for _, persona := range personas {
		value := Persona(strings.ToLower(strings.TrimSpace(string(persona))))
		if value == "" {
			return nil, fmt.Errorf("%w: persona cannot be blank", ErrInvalidArgument)
		}
		if !personaPattern.MatchString(string(value)) {
			return nil, fmt.Errorf("%w: invalid persona %q", ErrInvalidArgument, value)
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized, nil
}
