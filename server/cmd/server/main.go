//go:generate go run ./../genproto/
//go:generate go run ./../genrpc/

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"rpc-server/example"
	"rpc-server/internal/config"
	"rpc-server/internal/registry"
	"rpc-server/internal/router"
	"rpc-server/internal/rpc"
	"rpc-server/internal/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Load()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With("service", cfg.ServiceName))

	r := router.New()
	rpc.RegisterUserService(r, &example.UserService{})

	regClient := registry.NewClient(
		cfg.RegistryAddr,
		cfg.Port,
		cfg.ServiceName,
		cfg.HeartbeatInterval(),
		cfg.HeartbeatMaxFail,
		cfg.ReconnectBackoff(),
		cfg.ReconnectMaxBackoff(),
	)

	srv := server.New(
		cfg.ListenAddr(),
		r,
		regClient,
	)

	go func() {
		if err := srv.Start(ctx); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("started", "addr", cfg.ListenAddr())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("shutting down gracefully")
	srv.Stop()
	slog.Info("shutdown complete")
}
