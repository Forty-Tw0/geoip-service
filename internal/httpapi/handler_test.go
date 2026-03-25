package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"geoip-service/internal/authorize"
)

func TestHandleCheckAuthorization(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name       string
		body       any
		rawBody    string
		check      func(context.Context, string, []string) (authorize.Decision, error)
		wantStatus int
		wantBody   string
	}{
		{
			name: "returns decision",
			body: map[string]any{
				"ip_address":        "8.8.8.8",
				"allowed_countries": []string{"US", "CA"},
			},
			check: func(context.Context, string, []string) (authorize.Decision, error) {
				return authorize.Decision{
					IP:               "8.8.8.8",
					Allowed:          true,
					ResolvedCountry:  "US",
					AllowedCountries: []string{"CA", "US"},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `"allowed":true`,
		},
		{
			name:       "rejects malformed json",
			rawBody:    `{"ip_address":`,
			check:      func(context.Context, string, []string) (authorize.Decision, error) { return authorize.Decision{}, nil },
			wantStatus: http.StatusBadRequest,
			wantBody:   `"error":"invalid request body"`,
		},
		{
			name:       "rejects unknown fields",
			rawBody:    `{"ip_address":"8.8.8.8","allowed_countries":["US"],"country":"CA"}`,
			check:      func(context.Context, string, []string) (authorize.Decision, error) { return authorize.Decision{}, nil },
			wantStatus: http.StatusBadRequest,
			wantBody:   `"error":"invalid request body"`,
		},
		{
			name: "maps validation errors to bad request",
			body: map[string]any{
				"ip_address":        "bad-ip",
				"allowed_countries": []string{"US"},
			},
			check: func(context.Context, string, []string) (authorize.Decision, error) {
				return authorize.Decision{}, authorize.ErrInvalidIPAddress
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   `"error":"invalid ip address"`,
		},
		{
			name: "maps unresolved country errors",
			body: map[string]any{
				"ip_address":        "8.8.8.8",
				"allowed_countries": []string{"US"},
			},
			check: func(context.Context, string, []string) (authorize.Decision, error) {
				return authorize.Decision{}, authorize.ErrCountryNotResolved
			},
			wantStatus: http.StatusNotFound,
			wantBody:   `"error":"country could not be resolved for ip address"`,
		},
		{
			name: "maps unexpected failures to internal server error",
			body: map[string]any{
				"ip_address":        "8.8.8.8",
				"allowed_countries": []string{"US"},
			},
			check: func(context.Context, string, []string) (authorize.Decision, error) {
				return authorize.Decision{}, errors.New("boom")
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `"error":"internal server error"`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(logger, test.check, func(context.Context) error { return nil })
			rr := sendRequest(t, handler, http.MethodPost, "/v1/check", requestBody(t, test.body, test.rawBody))

			if rr.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d", test.wantStatus, rr.Code)
			}
			if !bytes.Contains(rr.Body.Bytes(), []byte(test.wantBody)) {
				t.Fatalf("expected response body to contain %q, got %q", test.wantBody, rr.Body.String())
			}
		})
	}
}

func TestHandleUpdate(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name       string
		update     func(context.Context) error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "updates database",
			update:     func(context.Context) error { return nil },
			wantStatus: http.StatusOK,
			wantBody:   `"status":"updated"`,
		},
		{
			name:       "returns service unavailable when update is not configured",
			update:     nil,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   `"error":"maxmind credentials are not configured"`,
		},
		{
			name:       "returns internal server error on update failure",
			update:     func(context.Context) error { return errors.New("boom") },
			wantStatus: http.StatusInternalServerError,
			wantBody:   `"error":"database update failed"`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(
				logger,
				func(context.Context, string, []string) (authorize.Decision, error) { return authorize.Decision{}, nil },
				test.update,
			)
			rr := sendRequest(t, handler, http.MethodPost, "/update", nil)

			if rr.Code != test.wantStatus {
				t.Fatalf("expected status %d, got %d", test.wantStatus, rr.Code)
			}
			if !bytes.Contains(rr.Body.Bytes(), []byte(test.wantBody)) {
				t.Fatalf("expected response body to contain %q, got %q", test.wantBody, rr.Body.String())
			}
		})
	}
}

func requestBody(t *testing.T, body any, raw string) io.Reader {
	t.Helper()

	if raw != "" {
		return bytes.NewBufferString(raw)
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	return bytes.NewReader(encoded)
}

func sendRequest(t *testing.T, handler http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, body)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}
