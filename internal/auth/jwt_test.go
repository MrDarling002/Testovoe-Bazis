package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testSecret = "test-secret-0123456789abcdef-32chars"

func TestJWT_GenerateParseRoundtrip(t *testing.T) {
	m := NewJWTManager(testSecret, time.Hour)

	token, err := m.Generate(42, "alice")
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if claims.UserID != 42 || claims.Username != "alice" {
		t.Fatalf("claims = %+v, want user 42 / alice", claims)
	}
}

func TestJWT_WrongSecretRejected(t *testing.T) {
	token, err := NewJWTManager(testSecret, time.Hour).Generate(1, "alice")
	if err != nil {
		t.Fatal(err)
	}

	other := NewJWTManager("another-secret-0123456789abcdef-32ch", time.Hour)
	if _, err := other.Parse(token); err == nil {
		t.Fatal("token signed with a different secret must be rejected")
	}
}

func TestJWT_ExpiredRejected(t *testing.T) {
	m := NewJWTManager(testSecret, -time.Minute)

	token, err := m.Generate(1, "alice")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := m.Parse(token); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

func TestAuthMiddleware(t *testing.T) {
	m := NewJWTManager(testSecret, time.Hour)

	var gotUserID int64

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("user id missing from context")
		}

		gotUserID = id
		w.WriteHeader(http.StatusOK)
	})

	handler := AuthMiddleware(m)(next)

	t.Run("missing header", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}

		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", ct)
		}
	})

	t.Run("malformed header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Basic abc")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		token, err := m.Generate(7, "bob")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}

		if gotUserID != 7 {
			t.Fatalf("user id in context = %d, want 7", gotUserID)
		}
	})
}
