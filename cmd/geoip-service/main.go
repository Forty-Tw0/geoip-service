package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"geoip-service/internal/authorize"
	"geoip-service/internal/geoip"
	"geoip-service/internal/grpcapi"
	"geoip-service/internal/httpapi"

	"google.golang.org/grpc"
)

const shutdownTimeout = 10 * time.Second

type config struct {
	httpAddress string
	grpcAddress string
	dbPath      string
	accountID   string
	licenseKey  string
	editionID   string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	resolver, err := geoip.NewMaxMindResolver(cfg.dbPath)
	if err != nil {
		return fmt.Errorf("initialize geoip resolver: %w", err)
	}
	defer func() {
		if err := resolver.Close(); err != nil {
			logger.Error("failed to close geoip resolver", "error", err)
		}
	}()

	check := func(_ context.Context, ipAddress string, allowedCountries []string) (authorize.Decision, error) {
		return authorize.Check(resolver.LookupCountry, ipAddress, allowedCountries)
	}

	var update httpapi.UpdateFunc
	if cfg.accountID != "" && cfg.licenseKey != "" {
		update = func(ctx context.Context) error {
			return resolver.Update(ctx, cfg.accountID, cfg.licenseKey, cfg.editionID)
		}
	}

	grpcServer := grpc.NewServer()
	grpcapi.RegisterGeoIPServer(
		grpcServer,
		grpcapi.NewServer(logger, check),
	)
	grpcListener, err := net.Listen("tcp", cfg.grpcAddress)
	if err != nil {
		return fmt.Errorf("listen for grpc on %s: %w", cfg.grpcAddress, err)
	}
	defer grpcListener.Close()

	handler := httpapi.NewHandler(logger, check, update)
	httpServer := &http.Server{
		Addr:    cfg.httpAddress,
		Handler: handler,
	}

	serverErrs := make(chan error, 2)

	go func() {
		logger.Info("starting grpc server", "listen_address", cfg.grpcAddress)
		serverErrs <- serveGRPC(grpcServer, grpcListener)
	}()

	go func() {
		logger.Info("starting http server", "listen_address", cfg.httpAddress)
		serverErrs <- serveHTTP(httpServer)
	}()

	var runErr error
	remaining := 2

	select {
	case runErr = <-serverErrs:
		remaining--
	case <-ctx.Done():
		logger.Info("shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := shutdownServers(shutdownCtx, httpServer, grpcServer); err != nil && runErr == nil {
		runErr = err
	}

	for i := 0; i < remaining; i++ {
		if err := <-serverErrs; err != nil && runErr == nil {
			runErr = err
		}
	}

	if runErr != nil {
		return runErr
	}

	logger.Info("servers stopped")
	return nil
}

func serveGRPC(server *grpc.Server, listener net.Listener) error {
	if err := server.Serve(listener); err != nil {
		return fmt.Errorf("serve grpc on %s: %w", listener.Addr(), err)
	}

	return nil
}

func serveHTTP(server *http.Server) error {
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http on %s: %w", server.Addr, err)
	}

	return nil
}

func shutdownServers(ctx context.Context, httpServer *http.Server, grpcServer *grpc.Server) error {
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		grpcServer.Stop()
		return fmt.Errorf("shutdown grpc server: %w", ctx.Err())
	}

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}

func loadConfig() (config, error) {
	dbPath := os.Getenv("GEOIP_DB_PATH")
	if dbPath == "" {
		return config{}, fmt.Errorf("GEOIP_DB_PATH is required")
	}

	return config{
		httpAddress: env("LISTEN_ADDRESS", "0.0.0.0:8042"),
		grpcAddress: env("GRPC_LISTEN_ADDRESS", "0.0.0.0:8842"),
		dbPath:      dbPath,
		accountID:   os.Getenv("MAXMIND_ACCOUNT_ID"),
		licenseKey:  os.Getenv("MAXMIND_LICENSE_KEY"),
		editionID:   env("MAXMIND_EDITION_ID", "GeoLite2-Country"),
	}, nil
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
