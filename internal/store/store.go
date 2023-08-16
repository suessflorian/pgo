package store

import (
	"context"
	"pgo/internal/profile"
)

// Store represents the bare minimum required for profile persistance
type Store interface {
	Put(ctx context.Context, profile profile.Profile) error
	Get(ctx context.Context, tag string) (*profile.Profile, error)
}
