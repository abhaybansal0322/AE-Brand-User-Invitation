package domain

import (
	"errors"
	"testing"
)

func TestValidateCreateBrandUserInputAcceptsValidRequest(t *testing.T) {
	input := CreateBrandUserInput{
		AccountID: "SWIGGY_IN#IM#ACC123",
		Email:     "admin@example.com",
		Name:      "Asha Brand",
		Mobile:    "+919876543210",
		Personas:  []Persona{"ads_admin", "discount_analyst"},
	}

	if err := ValidateCreateBrandUserInput(input); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
}

func TestValidateCreateBrandUserInputRejectsInvalidEmail(t *testing.T) {
	input := CreateBrandUserInput{
		AccountID: "SWIGGY_IN#IM#ACC123",
		Email:     "not-an-email",
		Name:      "Asha Brand",
		Mobile:    "+919876543210",
		Personas:  []Persona{"ads_admin"},
	}

	err := ValidateCreateBrandUserInput(input)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestValidateCreateBrandUserInputRejectsBlankAccount(t *testing.T) {
	input := CreateBrandUserInput{
		Email:    "admin@example.com",
		Name:     "Asha Brand",
		Mobile:   "+919876543210",
		Personas: []Persona{"ads_admin"},
	}

	err := ValidateCreateBrandUserInput(input)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestNormalizePersonasRejectsBlankPersona(t *testing.T) {
	_, err := NormalizePersonas([]Persona{"ads_admin", " "})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestNormalizePersonasDeduplicatesAndLowercases(t *testing.T) {
	got, err := NormalizePersonas([]Persona{"ADS_ADMIN", "ads_admin", "discount_analyst"})
	if err != nil {
		t.Fatalf("expected personas to normalize, got %v", err)
	}

	want := []Persona{"ads_admin", "discount_analyst"}
	if len(got) != len(want) {
		t.Fatalf("expected %d personas, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("persona[%d]: want %q got %q", i, want[i], got[i])
		}
	}
}

func TestActorIsSuperAdmin(t *testing.T) {
	actor := Actor{Personas: []Persona{"ads_admin", "super_admin"}}
	if !actor.IsSuperAdmin() {
		t.Fatal("expected actor with super_admin persona to pass")
	}
}
