// This is an example client application that demonstrates the use of how
// absence of explicit shutdown logic would be used with this profiler client package.
// Instead just using a simpler ctx listening approach. This'd represent the
// majority use case.

package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	pcap "pgo/pkg/pcapture"
	"syscall"
	"time"

	"golang.org/x/exp/slog"
)

func main() {
	var (
		logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		ctx    = context.Background()
	)

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	emit, _ := pcap.Capture("example", pcap.WithLogger(logger)) // optionally handle setup err
	defer func() {
		if err := emit(nil); err != nil {
			logger.WarnContext(ctx, "Failed to emit profile: "+err.Error())
		}
	}() // optionally handle emit err

	server := &http.Server{
		Addr: ":8081",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			reqLogger := logger.WithGroup("request").With("remote", r.RemoteAddr)
			defer reqLogger.InfoContext(ctx, "Request served")

			w.WriteHeader(http.StatusOK)
		}),
	}

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
			logger.ErrorContext(ctx, err.Error())
			os.Exit(1)
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
}
