package authorize

import (
	"errors"
	"net"
	"testing"

	"geoip-service/internal/geoip"
)

func TestCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		lookupCountry    func(net.IP) (string, error)
		ipAddress        string
		allowedCountries []string
		wantAllowed      bool
		wantCountry      string
		wantErr          error
	}{
		{
			name:             "allows when resolved country is in whitelist",
			lookupCountry:    func(net.IP) (string, error) { return "us", nil },
			ipAddress:        "8.8.8.8",
			allowedCountries: []string{"ca", "US"},
			wantAllowed:      true,
			wantCountry:      "US",
		},
		{
			name:             "denies when resolved country is outside whitelist",
			lookupCountry:    func(net.IP) (string, error) { return "DE", nil },
			ipAddress:        "8.8.8.8",
			allowedCountries: []string{"US", "CA"},
			wantAllowed:      false,
			wantCountry:      "DE",
		},
		{
			name:             "rejects invalid ip",
			lookupCountry:    func(net.IP) (string, error) { return "US", nil },
			ipAddress:        "not-an-ip",
			allowedCountries: []string{"US"},
			wantErr:          ErrInvalidIPAddress,
		},
		{
			name:             "rejects invalid country code",
			lookupCountry:    func(net.IP) (string, error) { return "US", nil },
			ipAddress:        "8.8.8.8",
			allowedCountries: []string{"USA"},
			wantErr:          ErrInvalidCountryCode,
		},
		{
			name:             "returns unresolved country when database has no result",
			lookupCountry:    func(net.IP) (string, error) { return "", geoip.ErrCountryNotFound },
			ipAddress:        "8.8.8.8",
			allowedCountries: []string{"US"},
			wantErr:          ErrCountryNotResolved,
		},
		{
			name:             "propagates resolver failure",
			lookupCountry:    func(net.IP) (string, error) { return "", errors.New("db failure") },
			ipAddress:        "8.8.8.8",
			allowedCountries: []string{"US"},
			wantErr:          errors.New("resolve country: db failure"),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decision, err := Check(test.lookupCountry, test.ipAddress, test.allowedCountries)
			if test.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", test.wantErr)
				}
				if !errors.Is(err, test.wantErr) && err.Error() != test.wantErr.Error() {
					t.Fatalf("expected error %v, got %v", test.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if decision.Allowed != test.wantAllowed {
				t.Fatalf("expected allowed=%v, got %v", test.wantAllowed, decision.Allowed)
			}
			if decision.ResolvedCountry != test.wantCountry {
				t.Fatalf("expected resolved country %q, got %q", test.wantCountry, decision.ResolvedCountry)
			}
		})
	}
}
