package main

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("GEOIP_DB_PATH", "/tmp/GeoLite2-Country.mmdb")
	t.Setenv("MAXMIND_ACCOUNT_ID", "account-id")
	t.Setenv("MAXMIND_LICENSE_KEY", "license-key")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if got, want := cfg.accountID, "account-id"; got != want {
		t.Fatalf("expected account id %q, got %q", want, got)
	}
	if got, want := cfg.licenseKey, "license-key"; got != want {
		t.Fatalf("expected license key %q, got %q", want, got)
	}
	if got, want := cfg.httpAddress, "0.0.0.0:8042"; got != want {
		t.Fatalf("expected listen address %q, got %q", want, got)
	}
	if got, want := cfg.grpcAddress, "0.0.0.0:8842"; got != want {
		t.Fatalf("expected grpc listen address %q, got %q", want, got)
	}
	if got, want := cfg.editionID, "GeoLite2-Country"; got != want {
		t.Fatalf("expected edition id %q, got %q", want, got)
	}
}

func TestLoadConfigRequiresDatabasePath(t *testing.T) {
	t.Setenv("GEOIP_DB_PATH", "")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "GEOIP_DB_PATH is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}
