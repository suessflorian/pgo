package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"

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
	fmt.Println(blob)

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

var ErrNoProfile = errors.New("no profile for given tag")

// PutCPUProfile puts the entire set of bytes into memory before establishing a reader to
// return. Limitation of the choosen package interaction.
func (s *store) GetCPUProfile(ctx context.Context, tag string) (io.Reader, error) {
	var cpuProfile []byte
	err := s.db.QueryRowContext(ctx, `SELECT cpu_profile FROM profiles WHERE tag = ?;`, tag).Scan(&cpuProfile)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoProfile
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
