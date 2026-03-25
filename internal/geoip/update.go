package geoip

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultEditionID = "GeoLite2-Country"
const maxMindDownloadHost = "https://download.maxmind.com"

var maxMindDownloadBaseURL = maxMindDownloadHost
var maxMindHTTPClient = &http.Client{Timeout: 2 * time.Minute}

func (r *MaxMindResolver) Update(ctx context.Context, accountID, licenseKey, editionID string) error {
	if editionID == "" {
		editionID = defaultEditionID
	}

	dir := filepath.Dir(r.databasePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "*.mmdb")
	if err != nil {
		return fmt.Errorf("create temp database file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if err := downloadDatabase(ctx, tmpFile, accountID, licenseKey, editionID); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp database file: %w", err)
	}
	if err := os.Rename(tmpPath, r.databasePath); err != nil {
		return fmt.Errorf("replace database file: %w", err)
	}

	return r.Reload()
}

func downloadDatabase(ctx context.Context, dst io.Writer, accountID, licenseKey, editionID string) error {
	if editionID == "" {
		editionID = defaultEditionID
	}
	if accountID == "" {
		return errors.New("download database: account id is required")
	}
	if licenseKey == "" {
		return errors.New("download database: license key is required")
	}

	downloadURL := fmt.Sprintf(
		"%s/geoip/databases/%s/download?suffix=tar.gz",
		maxMindDownloadBaseURL,
		url.PathEscape(editionID),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.SetBasicAuth(accountID, licenseKey)

	resp, err := maxMindHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download database: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download database: unexpected status %s", resp.Status)
	}

	return extractDatabase(dst, resp.Body)
}
func extractDatabase(dst io.Writer, src io.Reader) error {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("read gzip archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("download did not contain an mmdb file")
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}
		if header.FileInfo().IsDir() || !strings.HasSuffix(header.Name, ".mmdb") {
			continue
		}
		if _, err := io.Copy(dst, tr); err != nil {
			return fmt.Errorf("write database file: %w", err)
		}
		return nil
	}
}
