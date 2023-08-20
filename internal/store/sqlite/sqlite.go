// Package sqlite provides an implementation of the store interface using SQLite as the backend.
//
// SQLite is chosen for proof-of-concept purposes due to its simplicity and the ability to run as an
// in-memory database, removing the need for additional setup like you would require with databases such as PostgreSQL.
//
// Limitations:
// One significant limitation of this SQLite implementation is its memory consumption behavior.
// Both when storing CPU profiles (`PutCPUProfile` method) or retrieving them (`GetCPUProfile` method),
// the entire profile is loaded into memory. This can leads to some memory overhead, especially when dealing 
// with large and many profiles. Such behavior is a result of the chosen SQLite package interaction.
// Users looking for a more efficient, production-ready solution might want to consider other database backends
// or an SQLite driver that supports streaming.
//
// It's essential to be cautious when using this package for larger datasets or in memory-constrained environments.

package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"

	types "github.com/suessflorian/pgo/internal/store"

	_ "github.com/mattn/go-sqlite3"
	// NOTE: also https://github.com/tailscale/sqlite as a simple driver
	// And if you want to go all in; there's https://github.com/crawshaw/sqlite
	// which itself provides https://www.sqlite.org/c3ref/blob_open.html allowing
	// this store implementation to stream results directly through to the DB.
	// The repository cobwebs and of disconcerting api kept me from using it.
)

const (
	defaultPath     = "./profiles.db"
	defaultDatabase = "profiles"
	migration       = `
		CREATE TABLE IF NOT EXISTS profiles (
			tag TEXT PRIMARY KEY,
			cpu_profile BLOB,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`
)

func New(ctx context.Context) (*store, error) {
	db, err := sql.Open("sqlite3", defaultPath)
	if err != nil {
		return nil, err
	}

	if _, err := db.ExecContext(ctx, migration); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
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

// PutCPUProfile reads the stream into memory before writing to sqlite, this is due to the
// Limitation of the choosen sqlite package interaction.
func (s *store) PutCPUProfile(ctx context.Context, tag string, profile io.Reader) error {
	blob, err := io.ReadAll(profile)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO profiles (tag, cpu_profile)
		VALUES (?, ?)
		ON CONFLICT(tag) DO UPDATE SET cpu_profile = excluded.cpu_profile;
	`, tag, blob)
	if err != nil {
		return fmt.Errorf("Failed to insert cpu profile blob for %q: %w", tag, err)
	}

	return nil
}

// PutCPUProfile puts the entire set of bytes into memory before establishing a reader to
// return. Limitation of the choosen package interaction.
func (s *store) GetCPUProfile(ctx context.Context, tag string) (io.Reader, error) {
	var cpuProfile []byte
	err := s.db.QueryRowContext(ctx, `SELECT cpu_profile FROM profiles WHERE tag = ?;`, tag).Scan(&cpuProfile)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, types.ErrNoProfile
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve profile for %q: %w", tag, err)
	}

	return bytes.NewReader(cpuProfile), nil
}

func (s *store) Close() error {
	return s.db.Close()
}

type store struct {
	db *sql.DB
}
