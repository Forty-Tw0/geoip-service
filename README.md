# GeoIP Country Check Service

This is an internal HTTP/gRPC microservice that provides an IP country lookup endpoint, which returns a boolean that is true when the given IP is within the given list of countries.
IP lookups are backed by MaxMind provided .mmdb files, using an unofficial 3rd party library, oschwald/geoip2-golang.

## Layout

- `cmd/geoip-service`: entrypoint
- `internal/authorize`: input validation and allow/deny logic
- `internal/geoip`: MaxMind lookup, reload, and download logic
- `internal/httpapi`: HTTP handlers
- `internal/grpcapi`: gRPC server and protoc generated .pb.go stubs

## HTTP API

`POST /v1/check`

```json
{
  "ip_address": "8.8.8.8",
  "allowed_countries": ["US", "CA"]
}
```

```json
{
  "ip_address": "8.8.8.8",
  "allowed": true,
  "resolved_country": "US",
  "allowed_countries": ["CA", "US"]
}
```

`POST /update`

Refreshes the local MaxMind database when `MAXMIND_ACCOUNT_ID` and `MAXMIND_LICENSE_KEY` are configured, does nothing otherwise.

`GET /healthz`

Returns `{"status":"ok"}` for Kubernetes health checks.

Request rules

- `ip_address` must be a valid IPv4 or IPv6 address
- `allowed_countries` must contain at least one `ISO 3166-1 alpha-2` country code

Response codes:

- `400 Bad Request` for malformed input or validation failures
- `404 Not Found` when the IP is valid but no country can be resolved
- `500 Internal Server Error` for unexpected lookup failures
- `503 Service Unavailable` from `/update` when MaxMind credentials are not configured

## gRPC API

Service: `geoip.GeoIP/Check`

Proto: `proto/geoip.proto`

Protoc generated .pb.go files are at `internal/grpcapi`, rebuild them with `make proto`.

## Configuration

- `GEOIP_DB_PATH` (required): path to the MaxMind `.mmdb` file
- `LISTEN_ADDRESS` (optional, default `0.0.0.0:8042`): HTTP listen address
- `GRPC_LISTEN_ADDRESS` (optional, default `0.0.0.0:8842`): gRPC listen address
- `MAXMIND_ACCOUNT_ID` (optional): used by `/update` together with `MAXMIND_LICENSE_KEY`
- `MAXMIND_LICENSE_KEY` (optional): used by `/update` together with `MAXMIND_ACCOUNT_ID`
- `MAXMIND_EDITION_ID` (optional, default `GeoLite2-Country`): MaxMind edition to download

## Running

If you already have a GeoLite2 Country database locally:

```bash
GEOIP_DB_PATH=./GeoLite2-Country.mmdb go run ./cmd/geoip-service
```

Example request:

```bash
curl -s http://localhost:8042/v1/check \
  -H 'Content-Type: application/json' \
  -d '{"ip_address":"8.8.8.8","allowed_countries":["US","CA"]}'
```

## Development

If `proto/geoip.proto` changes, regenerate the checked-in gRPC files:

```bash
make proto
```

Build:

```bash
make build
```

Test:

```bash
make test
```

`make test` downloads MaxMind's `GeoIP2-Country-Test.mmdb` on first run, caches it under `.cache/test/`, and sets `GEOIP_DB_PATH` for the test process.

Run locally with the cached mmdb:

```bash
make run
```

## Production Deployment

TODO:
- Use paid GeoIP2 edition, which requires MAXMIND_ACCOUNT_ID and MAXMING_LICENSE_KEY.
- Bootstrap .mmdb at service start if it is missing, or at least manually create it before starting the service.
- Hook up /update to a daily job, or internalize the update code within the service.

