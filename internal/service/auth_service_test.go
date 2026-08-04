package service

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

type stubUserStore struct {
	createdEmail string
	createdHash  string
	createErr    error

	userByEmail domain.User
	getErr      error
}

func (s *stubUserStore) Create(_ context.Context, email, username, passwordHash string) (domain.User, error) {
	if s.createErr != nil {
		return domain.User{}, s.createErr
	}

	s.createdEmail = email
	s.createdHash = passwordHash

	return domain.User{ID: 1, Email: email, Username: username}, nil
}

func (s *stubUserStore) GetByID(context.Context, int64) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (s *stubUserStore) GetByEmail(context.Context, string) (domain.User, error) {
	if s.getErr != nil {
		return domain.User{}, s.getErr
	}

	return s.userByEmail, nil
}

type stubTokens struct {
	token string
	err   error
}

func (s *stubTokens) Generate(int64, string) (string, error) {
	return s.token, s.err
}

func TestRegister_Validation(t *testing.T) {
	svc := NewAuthService(&stubUserStore{}, &stubTokens{})

	tests := []struct {
		name     string
		email    string
		username string
		password string
	}{
		{"empty email", "", "user", "password123"},
		{"invalid email", "not-an-email", "user", "password123"},
		{"short username", "a@b.io", "ab", "password123"},
		{"short password", "a@b.io", "user", "1234567"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(context.Background(), tt.email, tt.username, tt.password)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("Register() = %v, want ErrValidation", err)
			}
		})
	}
}

func TestRegister_HashesPasswordAndNormalizesEmail(t *testing.T) {
	store := &stubUserStore{}
	svc := NewAuthService(store, &stubTokens{})

	user, err := svc.Register(context.Background(), "  Alice@Example.COM ", "alice", "password123")
	if err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	if user.Email != "alice@example.com" || store.createdEmail != "alice@example.com" {
		t.Fatalf("email = %q, want normalized lowercase", store.createdEmail)
	}

	if store.createdHash == "password123" {
		t.Fatal("password must never be stored in plain text")
	}

	if bcrypt.CompareHashAndPassword([]byte(store.createdHash), []byte("password123")) != nil {
		t.Fatal("stored hash must verify against the original password")
	}
}

func TestRegister_DuplicatePropagatesConflict(t *testing.T) {
	store := &stubUserStore{createErr: domain.ErrConflict}
	svc := NewAuthService(store, &stubTokens{})

	_, err := svc.Register(context.Background(), "a@b.io", "user", "password123")
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Register() = %v, want ErrConflict", err)
	}
}

func TestLogin_UnknownUserIsUnauthorized(t *testing.T) {
	store := &stubUserStore{getErr: domain.ErrNotFound}
	svc := NewAuthService(store, &stubTokens{})

	_, err := svc.Login(context.Background(), "a@b.io", "password123")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Login() = %v, want ErrUnauthorized", err)
	}
}

func TestLogin_WrongPasswordIsUnauthorized(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	store := &stubUserStore{userByEmail: domain.User{ID: 1, PasswordHash: string(hash)}}
	svc := NewAuthService(store, &stubTokens{})

	_, err = svc.Login(context.Background(), "a@b.io", "wrong-password")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Login() = %v, want ErrUnauthorized", err)
	}
}

func TestLogin_InfrastructureErrorIsNotUnauthorized(t *testing.T) {
	dbErr := errors.New("connection refused")
	store := &stubUserStore{getErr: dbErr}
	svc := NewAuthService(store, &stubTokens{})

	_, err := svc.Login(context.Background(), "a@b.io", "password123")
	if errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("infrastructure error must not be reported as unauthorized, got %v", err)
	}

	if !errors.Is(err, dbErr) {
		t.Fatalf("Login() = %v, want wrapped %v", err, dbErr)
	}
}

func TestLogin_Success(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	store := &stubUserStore{userByEmail: domain.User{ID: 1, Username: "alice", PasswordHash: string(hash)}}
	svc := NewAuthService(store, &stubTokens{token: "jwt-token"})

	token, err := svc.Login(context.Background(), "a@b.io", "correct-password")
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}

	if token != "jwt-token" {
		t.Fatalf("token = %q, want jwt-token", token)
	}
}
