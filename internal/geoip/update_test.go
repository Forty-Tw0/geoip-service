package geoip

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDownloadDatabaseUsesBasicAuth(t *testing.T) {
	originalBaseURL := maxMindDownloadBaseURL
	originalClient := maxMindHTTPClient
	t.Cleanup(func() {
		maxMindDownloadBaseURL = originalBaseURL
		maxMindHTTPClient = originalClient
	})

	maxMindDownloadBaseURL = "https://example.test"
	maxMindHTTPClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got, want := r.URL.Path, "/geoip/databases/GeoLite2-Country/download"; got != want {
				t.Fatalf("expected path %q, got %q", want, got)
			}
			if got, want := r.URL.RawQuery, "suffix=tar.gz"; got != want {
				t.Fatalf("expected query %q, got %q", want, got)
			}

			username, password, ok := r.BasicAuth()
			if !ok {
				t.Fatal("expected basic auth credentials")
			}
			if username != "account-id" || password != "license-key" {
				t.Fatalf("unexpected credentials %q / %q", username, password)
			}

			var body bytes.Buffer
			if err := writeArchive(&body, "GeoLite2-Country.mmdb", "mmdb-data"); err != nil {
				t.Fatalf("write archive: %v", err)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(body.Bytes())),
				Request:    r,
			}, nil
		}),
	}

	var dst bytes.Buffer
	err := downloadDatabase(context.Background(), &dst, "account-id", "license-key", "")
	if err != nil {
		t.Fatalf("downloadDatabase returned error: %v", err)
	}
	if got, want := dst.String(), "mmdb-data"; got != want {
		t.Fatalf("expected body %q, got %q", want, got)
	}
}

func TestDownloadDatabaseRequiresCredentials(t *testing.T) {
	tests := []struct {
		name       string
		accountID  string
		licenseKey string
		wantErr    string
	}{
		{
			name:       "missing account id",
			licenseKey: "license-key",
			wantErr:    "account id is required",
		},
		{
			name:      "missing license key",
			accountID: "account-id",
			wantErr:   "license key is required",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			err := downloadDatabase(context.Background(), io.Discard, test.accountID, test.licenseKey, "")
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected error containing %q, got %v", test.wantErr, err)
			}
		})
	}
}

func writeArchive(w io.Writer, filename, contents string) error {
	gzw := gzip.NewWriter(w)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	header := &tar.Header{
		Name: filename,
		Mode: 0o600,
		Size: int64(len(contents)),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(contents)); err != nil {
		return err
	}

	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
