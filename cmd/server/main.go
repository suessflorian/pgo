package main

import (
	"context"
	"errors"
	"github.com/suessflorian/pgo/internal/server"
	"github.com/suessflorian/pgo/internal/store/sqlite"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/exp/slog"
)

const (
	// SHUTDOWN_TIME represents the upper limit time allowance for a graceful shutdown
	SHUTDOWN_TIME = 5 * time.Second
)

func main() {
	var (
		logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		ctx    = context.Background()
	)

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := sqlite.New(ctx)
	if err != nil {
		logger.With("error", err).Error("failed to setup store")
		os.Exit(1)
	}
	defer db.Close()

	server := server.New(db, logger)
	go func() {
		logger.Info("Listening for requests")
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			logger.ErrorContext(ctx, err.Error())
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), SHUTDOWN_TIME)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(shutdownCtx, "Error during server shutdown: %s", err.Error())
		os.Exit(1)
	}

	logger.Info("Successfully shutdown gracefully")
	os.Exit(0)
}
