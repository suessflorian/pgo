package store

import (
	"context"
	"io"
)

// Store represents the bare minimum required for profile persistance
type Store interface {
	PutCPUProfile(ctx context.Context, tag string, profile io.Reader) error
	GetCPUProfile(ctx context.Context, tag string) (io.Reader, error)
	Close() error
}
