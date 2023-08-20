package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"pgo/internal/store"
	"pgo/internal/store/sqlite"
	"strings"
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

	s := &server{store: db, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("/profile/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleGet(w, r)
			return
		case http.MethodPost:
			s.handlePost(w, r)
			return
		default:
			w.Write([]byte("Incorrect method, either POST or GET"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
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

type server struct {
	store  store.Store
	logger *slog.Logger
}

var (
	ErrNoTagSupplied = errors.New("No tag supplied")
)

func (s *server) handlePost(w http.ResponseWriter, r *http.Request) {
	var (
		ctx    = r.Context()
		logger = s.logger.With("remote", r.RemoteAddr)
	)

	tag, err := parseRequestTag(r)
	if err != nil {
		if errors.Is(err, ErrNoTagSupplied) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(ErrNoTagSupplied.Error()))
			return
		}
		s.handleUnknownError(w, r, err)
		return
	}
	logger = logger.With("tag", tag)

	reader, err := r.MultipartReader()
	if err != nil {
		s.handleUnknownError(w, r, fmt.Errorf("Failed to read mulipart data: %w", err))
		return
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.handleUnknownError(w, r, fmt.Errorf("Error reading part: %w", err))
			return
		}

		switch name := part.FormName(); name {
		case "cpu_profile":
			{
				part := part
				if err := s.store.PutCPUProfile(ctx, tag, part); err != nil {
					s.handleUnknownError(w, r, fmt.Errorf("Failed to store cpu profile: %w", err))
					return
				}
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Unrecognized file: " + name))
			return
		}
	}

	logger.InfoContext(ctx, "Served request")
}

func (s *server) handleGet(w http.ResponseWriter, r *http.Request) {
	var (
		ctx    = r.Context()
		logger = s.logger.With("remote", r.RemoteAddr)
	)

	tag, err := parseRequestTag(r)
	if err != nil {
		if errors.Is(err, ErrNoTagSupplied) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(ErrNoTagSupplied.Error()))
			return
		}
		s.handleUnknownError(w, r, err)
		return
	}
	logger = logger.With("tag", tag)

	fmt.Println(tag)
	profile, err := s.store.GetCPUProfile(ctx, tag)
	if err != nil { // TODO: handle ErrNoProfile
		s.handleUnknownError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=default.pgo")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, profile); err != nil {
		s.logger.ErrorContext(ctx, "Failed to stream profile", "error", err)
		return
	}

	s.logger.InfoContext(ctx, "Request served")
}

func (s *server) handleUnknownError(w http.ResponseWriter, r *http.Request, err error) {
	defer s.logger.With("error", err.Error()).WarnContext(r.Context(), "responded with an unhandled error")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
	return
}

func parseRequestTag(r *http.Request) (tag string, err error) {
	return strings.TrimPrefix(r.URL.Path, "/profile/"), nil
}
