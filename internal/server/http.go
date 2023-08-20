package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"github.com/suessflorian/pgo/internal/store"
	"strings"

	"golang.org/x/exp/slog"
)
type server struct {
	*http.Server

	store  store.Store
	logger *slog.Logger
}

func New(db store.Store, logger *slog.Logger) *server {
	s := &server{
		store:  db,
		logger: logger,
	}

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

	s.Server = &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	return s
}

var (
	errNoTagSupplied = errors.New("No tag supplied")
)

func (s *server) handlePost(w http.ResponseWriter, r *http.Request) {
	var (
		ctx    = r.Context()
		logger = s.logger.With("remote", r.RemoteAddr)
	)

	tag, err := parseRequestTag(r)
	if err != nil {
		if errors.Is(err, errNoTagSupplied) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(errNoTagSupplied.Error()))
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
		if errors.Is(err, errNoTagSupplied) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(errNoTagSupplied.Error()))
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
