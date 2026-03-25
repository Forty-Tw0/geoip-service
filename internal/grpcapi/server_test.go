package grpcapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"geoip-service/internal/authorize"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestServerCheckReturnsDecision(t *testing.T) {
	t.Parallel()

	server := NewServer(nil, func(context.Context, string, []string) (authorize.Decision, error) {
		return authorize.Decision{
			IP:               "8.8.8.8",
			Allowed:          true,
			ResolvedCountry:  "US",
			AllowedCountries: []string{"CA", "US"},
		}, nil
	})

	resp, err := server.Check(context.Background(), &CheckRequest{
		IpAddress:        "8.8.8.8",
		AllowedCountries: []string{"US", "CA"},
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if got, want := resp.IpAddress, "8.8.8.8"; got != want {
		t.Fatalf("expected ip address %q, got %q", want, got)
	}
	if !resp.Allowed {
		t.Fatal("expected allowed response")
	}
	if got, want := resp.ResolvedCountry, "US"; got != want {
		t.Fatalf("expected resolved country %q, got %q", want, got)
	}
}

func TestServerCheckMapsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
		wantMsg  string
	}{
		{
			name:     "invalid argument",
			err:      authorize.ErrInvalidIPAddress,
			wantCode: codes.InvalidArgument,
			wantMsg:  "invalid ip address",
		},
		{
			name:     "not found",
			err:      authorize.ErrCountryNotResolved,
			wantCode: codes.NotFound,
			wantMsg:  "country could not be resolved for ip address",
		},
		{
			name:     "internal",
			err:      errors.New("boom"),
			wantCode: codes.Internal,
			wantMsg:  "internal server error",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := NewServer(nil, func(context.Context, string, []string) (authorize.Decision, error) {
				return authorize.Decision{}, test.err
			})

			_, err := server.Check(context.Background(), &CheckRequest{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			got := status.Convert(err)
			if got.Code() != test.wantCode {
				t.Fatalf("expected code %s, got %s", test.wantCode, got.Code())
			}
			if got.Message() != test.wantMsg {
				t.Fatalf("expected message %q, got %q", test.wantMsg, got.Message())
			}
		})
	}
}

func TestServerCheckLogsUnexpectedErrors(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	server := NewServer(logger, func(context.Context, string, []string) (authorize.Decision, error) {
		return authorize.Decision{}, errors.New("boom")
	})

	_, err := server.Check(context.Background(), &CheckRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(logs.String(), "authorization check failed") {
		t.Fatalf("expected log output to contain failure message, got %q", logs.String())
	}
}
