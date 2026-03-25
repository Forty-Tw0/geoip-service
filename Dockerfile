# reproducible build env for generation of protobuff code
FROM golang:1.25 AS proto-tools

WORKDIR /src

RUN apt-get update && apt-get install -y protobuf-compiler && rm -rf /var/lib/apt/lists/*
RUN GOBIN=/usr/local/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
RUN GOBIN=/usr/local/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

# actual builds start here
FROM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY proto/ ./proto/

# disable cgo for static linking on a minimal runtime image, we dont need C bindings anyway
RUN CGO_ENABLED=0 go build -trimpath -o /out/geoip-service ./cmd/geoip-service

# runtime on a very minimal deb12
FROM gcr.io/distroless/static-debian12:nonroot AS run

WORKDIR /app

COPY --from=build /out/geoip-service /app/geoip-service

ENV LISTEN_ADDRESS=0.0.0.0:8042
ENV GRPC_LISTEN_ADDRESS=0.0.0.0:8842
EXPOSE 8042
EXPOSE 8842

ENTRYPOINT ["/app/geoip-service"]
