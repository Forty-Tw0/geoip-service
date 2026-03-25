package grpcapi

import (
	"context"
	"log/slog"

	"geoip-service/internal/authorize"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CheckFunc func(context.Context, string, []string) (authorize.Decision, error)

type Server struct {
	UnimplementedGeoIPServer
	logger *slog.Logger
	check  CheckFunc
}

func NewServer(logger *slog.Logger, check CheckFunc) *Server {
	return &Server{
		logger: logger,
		check:  check,
	}
}

func (s *Server) Check(ctx context.Context, req *CheckRequest) (*CheckResponse, error) {
	decision, err := s.check(ctx, req.IpAddress, req.AllowedCountries)
	if err != nil {
		return nil, mapError(s.logger, err)
	}

	return &CheckResponse{
		IpAddress:        decision.IP,
		Allowed:          decision.Allowed,
		ResolvedCountry:  decision.ResolvedCountry,
		AllowedCountries: decision.AllowedCountries,
	}, nil
}

func mapError(logger *slog.Logger, err error) error {
	kind, message := authorize.ClassifyError(err)

	switch kind {
	case authorize.ErrorKindInvalidArgument:
		return status.Error(codes.InvalidArgument, message)
	case authorize.ErrorKindNotFound:
		return status.Error(codes.NotFound, message)
	default:
		if logger != nil {
			logger.Error("authorization check failed", "error", err)
		}
		return status.Error(codes.Internal, message)
	}
}
