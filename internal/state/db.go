package state

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type ImageState struct {
	Registry            string
	Repository          string
	Reference           string
	PlatformOS          string
	PlatformArch        string
	PlatformVariant     string
	IndexDigest         string
	ImageManifestDigest string
}

type BlobState struct {
	Digest    string
	Size      int64
	MediaType string
	LocalPath string
	Kind      string
	Verified  bool
}

func Open(path string) (*Store, error) {
	if err := ensureParent(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set wal mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS image_state (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			registry TEXT NOT NULL,
			repository TEXT NOT NULL,
			reference TEXT NOT NULL,
			platform_os TEXT NOT NULL,
			platform_arch TEXT NOT NULL,
			platform_variant TEXT NOT NULL DEFAULT '',
			index_digest TEXT,
			image_manifest_digest TEXT,
			last_checked_at INTEGER,
			last_changed_at INTEGER,
			UNIQUE(registry, repository, reference, platform_os, platform_arch, platform_variant)
		);`,
		`CREATE TABLE IF NOT EXISTS blob_state (
			digest TEXT PRIMARY KEY,
			size INTEGER,
			media_type TEXT,
			local_path TEXT,
			kind TEXT,
			verified INTEGER DEFAULT 0,
			created_at INTEGER,
			updated_at INTEGER
		);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetImageState(ctx context.Context, key ImageState) (ImageState, bool, error) {
	const query = `SELECT registry, repository, reference, platform_os, platform_arch, platform_variant, index_digest, image_manifest_digest
	FROM image_state WHERE registry=? AND repository=? AND reference=? AND platform_os=? AND platform_arch=? AND platform_variant=?`
	row := s.db.QueryRowContext(ctx, query, key.Registry, key.Repository, key.Reference, key.PlatformOS, key.PlatformArch, key.PlatformVariant)
	var state ImageState
	if err := row.Scan(&state.Registry, &state.Repository, &state.Reference, &state.PlatformOS, &state.PlatformArch, &state.PlatformVariant, &state.IndexDigest, &state.ImageManifestDigest); err != nil {
		if err == sql.ErrNoRows {
			return ImageState{}, false, nil
		}
		return ImageState{}, false, err
	}
	return state, true, nil
}

func (s *Store) UpsertImageState(ctx context.Context, state ImageState) error {
	now := time.Now().Unix()
	const stmt = `INSERT INTO image_state (registry, repository, reference, platform_os, platform_arch, platform_variant, index_digest, image_manifest_digest, last_checked_at, last_changed_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(registry, repository, reference, platform_os, platform_arch, platform_variant)
	DO UPDATE SET index_digest=excluded.index_digest, image_manifest_digest=excluded.image_manifest_digest, last_checked_at=excluded.last_checked_at, last_changed_at=excluded.last_changed_at`
	_, err := s.db.ExecContext(ctx, stmt,
		state.Registry, state.Repository, state.Reference,
		state.PlatformOS, state.PlatformArch, state.PlatformVariant,
		state.IndexDigest, state.ImageManifestDigest, now, now,
	)
	return err
}

func (s *Store) UpsertBlobState(ctx context.Context, blob BlobState) error {
	now := time.Now().Unix()
	const stmt = `INSERT INTO blob_state (digest, size, media_type, local_path, kind, verified, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(digest)
	DO UPDATE SET size=excluded.size, media_type=excluded.media_type, local_path=excluded.local_path, kind=excluded.kind, verified=excluded.verified, updated_at=excluded.updated_at`
	verified := 0
	if blob.Verified {
		verified = 1
	}
	_, err := s.db.ExecContext(ctx, stmt, blob.Digest, blob.Size, blob.MediaType, blob.LocalPath, blob.Kind, verified, now, now)
	return err
}

func ensureParent(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	return nil
}
