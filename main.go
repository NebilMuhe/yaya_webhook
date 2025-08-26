package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
)

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	mux := http.NewServeMux()

	logger.InfoContext(ctx, "Initializing configuration...")
	initializeConfig(ctx, logger)
	logger.InfoContext(ctx, "Configuration initialized successfully")

	server := &http.Server{
		Handler: mux,
		Addr:    ":" + viper.GetString("port"),
	}

	logger.InfoContext(ctx, "Intializing yaya webhook handler...")
	handler := NewHandler(viper.GetString("secret_key"), logger)
	logger.InfoContext(ctx, "Yaya webhook handler initialized successfully")

	mux.HandleFunc("GET /healthcheck", handler.HealthCheckHandler)
	mux.HandleFunc("POST /webhook", handler.YayayWebhookHandler)

	startServer(ctx, server, logger)

	stopServer(ctx, server, logger)
}

func initializeConfig(ctx context.Context, logger *slog.Logger) {
	viper.AddConfigPath(".")
	viper.SetConfigName("example_config")
	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	if err != nil {
		logger.ErrorContext(ctx, "Error reading config file", "error", err)
		os.Exit(1)
	}
}

func startServer(ctx context.Context, server *http.Server, log *slog.Logger) {
	go func() {
		log.InfoContext(ctx, "Starting server on port", "port", viper.GetString("port"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.ErrorContext(ctx, "server stopped", "error", err)
			os.Exit(1)
		}
		log.InfoContext(ctx, "Server started")
	}()
}

func stopServer(ctx context.Context, server *http.Server, log *slog.Logger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.InfoContext(ctx, "Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), viper.GetDuration("timeout"))
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.InfoContext(ctx, "server forced to shutdown ", "error", err)
	}
	log.InfoContext(ctx, "Server stopped successfully")
}
