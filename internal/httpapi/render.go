package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/example/Testovoe-Bazis/internal/domain"
)

const maxBodyBytes = 1 << 20

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		status int
		msg    string
	)

	switch {
	case errors.Is(err, domain.ErrNotFound):
		status, msg = http.StatusNotFound, "not found"
	case errors.Is(err, domain.ErrForbidden):
		status, msg = http.StatusForbidden, "forbidden"
	case errors.Is(err, domain.ErrUnauthorized):
		status, msg = http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, domain.ErrValidation):
		status, msg = http.StatusBadRequest, err.Error()
	case errors.Is(err, domain.ErrConflict):
		status, msg = http.StatusConflict, err.Error()
	default:
		h.logger.Error("internal error",
			"method", r.Method,
			"path", r.URL.Path,
			"error", err,
		)

		status, msg = http.StatusInternalServerError, "internal server error"
	}

	respondJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: invalid request body: %s", domain.ErrValidation, err)
	}

	return nil
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%w: invalid id in path", domain.ErrValidation)
	}

	return id, nil
}

func queryInt(values url.Values, name string) (int, error) {
	raw := values.Get(name)
	if raw == "" {
		return 0, nil
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", domain.ErrValidation, name)
	}

	return v, nil
}

func queryInt64(values url.Values, name string) (int64, error) {
	raw := values.Get(name)
	if raw == "" {
		return 0, nil
	}

	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", domain.ErrValidation, name)
	}

	return v, nil
}
