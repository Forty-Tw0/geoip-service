.PHONY: build run test test-grpc proto

TEST_GEOIP_DB_PATH ?= .cache/test/GeoIP2-Country-Test.mmdb

build:
	docker compose build

run:
	mkdir -p "$(dir $(TEST_GEOIP_DB_PATH))"
	test -s "$(TEST_GEOIP_DB_PATH)" || curl -fsSL https://github.com/maxmind/MaxMind-DB/raw/refs/heads/main/test-data/GeoIP2-Country-Test.mmdb -o "$(TEST_GEOIP_DB_PATH)"
	TEST_GEOIP_DB_PATH="$(TEST_GEOIP_DB_PATH)" docker compose up --build

test:
	mkdir -p "$(dir $(TEST_GEOIP_DB_PATH))"
	test -s "$(TEST_GEOIP_DB_PATH)" || curl -fsSL https://github.com/maxmind/MaxMind-DB/raw/refs/heads/main/test-data/GeoIP2-Country-Test.mmdb -o "$(TEST_GEOIP_DB_PATH)"
	GEOIP_DB_PATH="$(TEST_GEOIP_DB_PATH)" go test ./...
	curl -XPOST "localhost:8042/v1/check" -H "content-type: application/json" -d '{"ip_address":"74.209.24.0","allowed_countries":["US","CA"]}'
	curl -XPOST "localhost:8042/v1/check" -H "content-type: application/json" -d '{"ip_address":"74.209.24.0","allowed_countries":["UA","CA"]}'

test-grpc:
	grpcurl -plaintext \
		-import-path proto \
		-proto geoip.proto \
		-d '{"ip_address":"74.209.24.0","allowed_countries":["US","CA"]}' \
		localhost:8842 \
		geoip.GeoIP/Check
	grpcurl -plaintext \
		-import-path proto \
		-proto geoip.proto \
		-d '{"ip_address":"74.209.24.0","allowed_countries":["UA"]}' \
		localhost:8842 \
		geoip.GeoIP/Check

proto:
	docker build --target proto-tools -t geoip-service-proto .
	docker run --rm \
		-u $$(id -u):$$(id -g) \
		-v "$$PWD:/src" \
		-w /src \
		geoip-service-proto \
		sh -lc 'mkdir -p internal/grpcapi && protoc --proto_path=proto --go_out=paths=source_relative:internal/grpcapi --go-grpc_out=paths=source_relative:internal/grpcapi proto/geoip.proto'
