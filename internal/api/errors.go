package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/panttony/kbase/internal/storage"
)

type cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	InvalidateAll(ctx context.Context, prefix string) error
}

type errBody struct {
	Error errDetail `json:"error"`
}

type errDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type appError struct {
	status  int
	code    string
	message string
}

func (e *appError) Error() string { return e.message }

const (
	codeInvalidRequest = "invalid_request"
	codeNotFound       = "not_found"
	codeBadRequest     = "bad_request"
	codeInternal       = "internal"
)

func writeError(w http.ResponseWriter, err error) {
	var app *appError
	if errors.As(err, &app) {
		writeJSON(w, app.status, errBody{Error: errDetail{Code: app.code, Message: app.message}})
		return
	}
	var nf storage.ErrNotFound
	if errors.As(err, &nf) {
		writeJSON(w, http.StatusNotFound, errBody{Error: errDetail{Code: codeNotFound, Message: err.Error()}})
		return
	}
	slog.Error("internal server error", "error", err)
	writeJSON(w, http.StatusInternalServerError, errBody{Error: errDetail{Code: codeInternal, Message: "internal server error"}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

func mapRepoError(err error) error {
	var nf storage.ErrNotFound
	if errors.As(err, &nf) {
		return &appError{status: http.StatusNotFound, code: codeNotFound, message: err.Error()}
	}
	return err
}

func invalidRequest(msg string) error {
	return &appError{status: http.StatusBadRequest, code: codeInvalidRequest, message: msg}
}
