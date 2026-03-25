package authorize

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"geoip-service/internal/geoip"
)

var (
	ErrInvalidIPAddress   = errors.New("invalid ip address")
	ErrNoAllowedCountries = errors.New("allowed countries list must not be empty")
	ErrInvalidCountryCode = errors.New("invalid country code")
	ErrCountryNotResolved = errors.New("country could not be resolved for ip address")
)

type ErrorKind int

const (
	ErrorKindInvalidArgument ErrorKind = iota + 1
	ErrorKindNotFound
	ErrorKindInternal
)

type Decision struct {
	IP               string   `json:"ip_address"`
	Allowed          bool     `json:"allowed"`
	ResolvedCountry  string   `json:"resolved_country,omitempty"`
	AllowedCountries []string `json:"allowed_countries"`
}

func Check(lookupCountry func(net.IP) (string, error), ipAddress string, allowedCountries []string) (Decision, error) {
	normalizedCountries, err := normalizeCountryCodes(allowedCountries)
	if err != nil {
		return Decision{}, err
	}

	ip := net.ParseIP(strings.TrimSpace(ipAddress))
	if ip == nil {
		return Decision{}, ErrInvalidIPAddress
	}

	resolvedCountry, err := lookupCountry(ip)
	if err != nil {
		if errors.Is(err, geoip.ErrCountryNotFound) {
			return Decision{}, ErrCountryNotResolved
		}
		return Decision{}, fmt.Errorf("resolve country: %w", err)
	}

	resolvedCountry = strings.ToUpper(resolvedCountry)

	return Decision{
		IP:               ip.String(),
		Allowed:          slices.Contains(normalizedCountries, resolvedCountry),
		ResolvedCountry:  resolvedCountry,
		AllowedCountries: normalizedCountries,
	}, nil
}

func normalizeCountryCodes(countryCodes []string) ([]string, error) {
	if len(countryCodes) == 0 {
		return nil, ErrNoAllowedCountries
	}

	seen := make(map[string]struct{}, len(countryCodes))
	normalized := make([]string, 0, len(countryCodes))

	for _, code := range countryCodes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if len(code) != 2 || !isAlpha(code) {
			return nil, ErrInvalidCountryCode
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		normalized = append(normalized, code)
	}

	slices.Sort(normalized)
	return normalized, nil
}

func isAlpha(value string) bool {
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func ClassifyError(err error) (ErrorKind, string) {
	switch {
	case errors.Is(err, ErrInvalidIPAddress),
		errors.Is(err, ErrNoAllowedCountries),
		errors.Is(err, ErrInvalidCountryCode):
		return ErrorKindInvalidArgument, err.Error()
	case errors.Is(err, ErrCountryNotResolved):
		return ErrorKindNotFound, err.Error()
	default:
		return ErrorKindInternal, "internal server error"
	}
}
