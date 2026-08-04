package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 72 // bcrypt input limit
	minUsernameLength = 3
	maxUsernameLength = 100
	maxEmailLength    = 255
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type UserStore interface {
	Create(ctx context.Context, email, username, passwordHash string) (domain.User, error)
	GetByID(ctx context.Context, id int64) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
}

type TokenIssuer interface {
	Generate(userID int64, username string) (string, error)
}

type AuthService struct {
	users  UserStore
	tokens TokenIssuer
}

func NewAuthService(users UserStore, tokens TokenIssuer) *AuthService {
	return &AuthService{
		users:  users,
		tokens: tokens,
	}
}

func (s *AuthService) Register(ctx context.Context, email, username, password string) (domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.TrimSpace(username)

	if err := validateCredentials(email, username, password); err != nil {
		return domain.User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	return s.users.Create(ctx, email, username, string(hash))
}

// dummyHash keeps Login runtime roughly constant whether or not the user
// exists, mitigating account enumeration via response timing.
var dummyHash = func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcrypt.DefaultCost)
	if err != nil {
		panic(fmt.Sprintf("generate dummy bcrypt hash: %v", err))
	}

	return h
}()

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return "", domain.ErrUnauthorized
		}

		return "", fmt.Errorf("get user by email: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", domain.ErrUnauthorized
	}

	token, err := s.tokens.Generate(user.ID, user.Username)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return token, nil
}

func validateCredentials(email, username, password string) error {
	if email == "" || len(email) > maxEmailLength || !emailPattern.MatchString(email) {
		return fmt.Errorf("%w: invalid email", domain.ErrValidation)
	}

	if len(username) < minUsernameLength || len(username) > maxUsernameLength {
		return fmt.Errorf(
			"%w: username must be %d-%d characters",
			domain.ErrValidation, minUsernameLength, maxUsernameLength,
		)
	}

	if len(password) < minPasswordLength || len(password) > maxPasswordLength {
		return fmt.Errorf(
			"%w: password must be %d-%d characters",
			domain.ErrValidation, minPasswordLength, maxPasswordLength,
		)
	}

	return nil
}
