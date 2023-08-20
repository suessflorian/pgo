package store

import (
	"context"
	"errors"
	"io"
)

// Store represents the bare minimum required for profile persistance
type Store interface {
	PutCPUProfile(ctx context.Context, tag string, profile io.Reader) error
	GetCPUProfile(ctx context.Context, tag string) (io.Reader, error)
	Close() error
}

var ErrNoProfile = errors.New("no profile for given tag")
