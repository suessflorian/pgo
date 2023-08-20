package pcapture

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"runtime/pprof"
	"sync"
	"time"

	"golang.org/x/exp/slog"
)

const server = "http://localhost:8080/profile"

type Option func(cfg *config)

// WithLogger returns an Option that sets a given logger to be used during the profile capture.
// By default uses slog.Default().
func WithLogger(logger *slog.Logger) Option {
	return func(cfg *config) {
		cfg.logger = logger
	}
}

// Capture begins a pprof CPU profile capture using an open file as the medium of the
// profiling run. The profile is emitted when the returned emit function is invoked.
// This emit optionally accepts a context, allowing the emitting to adhere to any shutdown
// specific lifecycle rules.
func Capture(tag string, opts ...Option) (emit func(ctx context.Context) error, err error) {
	var cfg = &config{
		logger: slog.Default(),
	}

	for _, withOpt := range opts {
		withOpt(cfg)
	}

	f, err := os.Create(tag + ".profile")
	if err != nil {
		return nil, fmt.Errorf("Failed to create cpu profiling file: %w", err)
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		return nil, fmt.Errorf("Error while creating CPU profile: %w", err)
	}
	cfg.logger.Info("CPU profiling via pprof")

	var (
		client = &http.Client{Timeout: 5 * time.Second}
		once   sync.Once
	)

	emit = func(ctx context.Context) error {
		var err error

		once.Do(func() {
			defer f.Close()
			pprof.StopCPUProfile()

			// Reset the file descriptor's position to the beginning of the file
			_, err := f.Seek(0, 0)
			if err != nil {
				return
			}

			pr, pw := io.Pipe()
			w := multipart.NewWriter(pw)

			go func() {
				defer pw.Close()
				defer w.Close()

				part, err := w.CreateFormFile("cpu_profile", "cpu_profile")
				if err != nil {
					cfg.logger.With("error", err.Error()).ErrorContext(ctx, "Failed to create a form file for cpu profile")
					return
				}

				_, err = io.Copy(part, f)
				if err != nil {
					cfg.logger.With("error", err.Error()).ErrorContext(ctx, "Failed to write `cpu_profile` to form file")
					return
				}
			}()

			var req *http.Request
			req, err = http.NewRequest(http.MethodPost, server+"/"+tag, pr)
			if err != nil {
				err = fmt.Errorf("Failed to assemble profile dump request: %w", err)
				return
			}

			if ctx != nil {
				req = req.WithContext(ctx)
			}

			// TODO: Check that we're actually sending all the right header.
			req.Header.Set("Content-Type", w.FormDataContentType())

			res, err := client.Do(req)
			if err != nil {
				err = fmt.Errorf("Failure during the sending of the profile dump: %w", err)
				return
			}
			defer res.Body.Close()

			cfg.logger.DebugContext(ctx, "Request sent to: "+req.URL.String())

			if res.StatusCode != http.StatusOK {
				err = fmt.Errorf("Unexpected status code response from profiling server: %d", res.StatusCode)
			}

			cfg.logger.InfoContext(ctx, "Successfully published profile")
		})

		return err
	}

	return emit, nil
}

type config struct {
	logger *slog.Logger
}
