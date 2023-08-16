package sqlite

import (
	"context"
	"database/sql"
	"embed"
	_ "embed"
	"errors"
	"fmt"
	"pgo/internal/profile"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
)

const (
	defaultPath     = "./profiles.db"
	defaultDatabase = "profiles"
)

//go:embed migrations
var migrations embed.FS

func New() (*store, error) {
	db, err := sql.Open("sqlite3", defaultPath)
	if err != nil {
		return nil, err
	}

	driver, err := sqlite.WithInstance(db, &sqlite.Config{DatabaseName: defaultDatabase})
	if err != nil {
		return nil, err
	}
	source, err := iofs.New(migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("Failed to setup migrations source: %w", err)
	}
	defer source.Close()

	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		return nil, fmt.Errorf("Failed to create migrator instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		db.Close()
		return nil, fmt.Errorf("Failed to perform migrations: %w", err)
	}

	return &store{db: db}, nil
}

func NewWithPath(path string) (*store, error) {
	db, err := sql.Open("sqlite3", defaultPath)
	if err != nil {
		return nil, err
	}
	return &store{db: db}, nil
}

func (s *store) Put(ctx context.Context, profile profile.Profile) error {
	return nil
}

func (s *store) Get(ctx context.Context, tag string) (*profile.Profile, error) {
	return nil, nil
}

func (s *store) Close() error {
	return s.db.Close()
}

type store struct {
	db *sql.DB
}
