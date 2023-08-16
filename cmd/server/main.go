package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"pgo/internal/store/sqlite"
	"syscall"
	"time"

	"golang.org/x/exp/slog"
)

func main() {
	var (
		logger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
		ctx    = context.Background()
	)

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := sqlite.New()
	if err != nil {
		logger.With("error", err).Error("failed to setup store")
		os.Exit(1)
	}
	defer db.Close()

	server := &http.Server{
		Addr: ":8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			logger := logger.WithGroup("request").With("remote", r.RemoteAddr)
			defer logger.InfoContext(ctx, "Request served")

			w.WriteHeader(http.StatusOK)
		}),
	}

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			logger.ErrorContext(ctx, err.Error())
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(shutdownCtx, "Error during server shutdown: %s", err.Error())
		os.Exit(1)
	}

	logger.Info("Successfully shutdown gracefully")
	os.Exit(0)
}
