package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"rpc-server/example"
	"rpc-server/internal/registry"
	"rpc-server/internal/router"
	"rpc-server/internal/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	selfPort, _ := strconv.Atoi(getEnv("RPC_PORT", "8080"))
	serviceName := getEnv("SERVICE_NAME", "UserService")

	r := router.New()
	r.Register(serviceName, "getUser", example.GetUserHandler)

	regClient := registry.NewClient(
		getEnv("REGISTRY_ADDR", "localhost:9000"),
		getEnv("SELF_IP", "127.0.0.1"),
		selfPort,
		serviceName,
	)

	srv := server.New(
		getEnv("RPC_ADDR", "0.0.0.0:"+getEnv("RPC_PORT", "8080")),
		r,
		regClient,
	)

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("[main] server error: %v", err)
		}
	}()

	log.Printf("[main] started service=%s", serviceName)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("[main] shutting down")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
