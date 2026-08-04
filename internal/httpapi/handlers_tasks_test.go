package httpapi

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

func TestUpdateTaskRequestToPatch(t *testing.T) {
	t.Run("absent assignee means no change", func(t *testing.T) {
		var req updateTaskRequest
		if err := json.Unmarshal([]byte(`{"title":"x"}`), &req); err != nil {
			t.Fatal(err)
		}

		patch, err := req.toPatch()
		if err != nil {
			t.Fatal(err)
		}

		if patch.Assignee.Set {
			t.Fatal("absent assignee_id must not be marked as set")
		}
	})

	t.Run("explicit null unassigns", func(t *testing.T) {
		var req updateTaskRequest
		if err := json.Unmarshal([]byte(`{"assignee_id":null}`), &req); err != nil {
			t.Fatal(err)
		}

		patch, err := req.toPatch()
		if err != nil {
			t.Fatal(err)
		}

		if !patch.Assignee.Set || patch.Assignee.Value != nil {
			t.Fatalf("explicit null must set unassign, got %+v", patch.Assignee)
		}
	})

	t.Run("concrete id reassigns", func(t *testing.T) {
		var req updateTaskRequest
		if err := json.Unmarshal([]byte(`{"assignee_id":7}`), &req); err != nil {
			t.Fatal(err)
		}

		patch, err := req.toPatch()
		if err != nil {
			t.Fatal(err)
		}

		if !patch.Assignee.Set || patch.Assignee.Value == nil || *patch.Assignee.Value != 7 {
			t.Fatalf("assignee = %+v, want set to 7", patch.Assignee)
		}
	})

	t.Run("non-integer rejected", func(t *testing.T) {
		var req updateTaskRequest
		if err := json.Unmarshal([]byte(`{"assignee_id":"abc"}`), &req); err != nil {
			t.Fatal(err)
		}

		if _, err := req.toPatch(); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("toPatch() = %v, want ErrValidation", err)
		}
	})
}

func TestDecodeJSON(t *testing.T) {
	t.Run("unknown fields rejected", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader(`{"unknown_field":1}`))
		rec := httptest.NewRecorder()

		var dst createTaskRequest

		if err := decodeJSON(rec, req, &dst); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("decodeJSON() = %v, want ErrValidation", err)
		}
	})

	t.Run("valid body decoded", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader(`{"team_id":1,"title":"x"}`))
		rec := httptest.NewRecorder()

		var dst createTaskRequest

		if err := decodeJSON(rec, req, &dst); err != nil {
			t.Fatalf("decodeJSON() unexpected error: %v", err)
		}

		if dst.TeamID != 1 || dst.Title != "x" {
			t.Fatalf("decoded = %+v", dst)
		}
	})

	t.Run("oversized body rejected", func(t *testing.T) {
		big := `{"title":"` + strings.Repeat("x", maxBodyBytes+1) + `"}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(big))
		rec := httptest.NewRecorder()

		var dst createTaskRequest

		if err := decodeJSON(rec, req, &dst); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("decodeJSON() = %v, want ErrValidation", err)
		}
	})
}
