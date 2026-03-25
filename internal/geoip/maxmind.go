package geoip

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

var (
	ErrCountryNotFound = errors.New("country not found")
	ErrResolverClosed  = errors.New("resolver is closed")
)

type MaxMindResolver struct {
	mu           sync.RWMutex
	databasePath string
	reader       *geoip2.Reader
}

func NewMaxMindResolver(databasePath string) (*MaxMindResolver, error) {
	reader, err := geoip2.Open(databasePath)
	if err != nil {
		return nil, fmt.Errorf("open maxmind database: %w", err)
	}

	return &MaxMindResolver{
		databasePath: databasePath,
		reader:       reader,
	}, nil
}

func (r *MaxMindResolver) LookupCountry(ip net.IP) (string, error) {
	r.mu.RLock()
	reader := r.reader
	r.mu.RUnlock()
	if reader == nil {
		return "", ErrResolverClosed
	}

	record, err := reader.Country(ip)
	if err != nil {
		return "", fmt.Errorf("lookup country: %w", err)
	}

	code := record.Country.IsoCode
	if code == "" {
		return "", ErrCountryNotFound
	}

	return code, nil
}

func (r *MaxMindResolver) Reload() error {
	reader, err := geoip2.Open(r.databasePath)
	if err != nil {
		return fmt.Errorf("open maxmind database: %w", err)
	}

	r.mu.Lock()
	previous := r.reader
	r.reader = reader
	r.mu.Unlock()

	if previous != nil {
		return previous.Close()
	}

	return nil
}

func (r *MaxMindResolver) Close() error {
	r.mu.Lock()
	reader := r.reader
	r.reader = nil
	r.mu.Unlock()

	if reader == nil {
		return nil
	}

	return reader.Close()
}
