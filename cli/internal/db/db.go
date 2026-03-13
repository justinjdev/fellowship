package db

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// Conn is an alias for sqlite.Conn to simplify consumer imports.
type Conn = sqlite.Conn

// DB manages a SQLite connection pool for fellowship state.
type DB struct {
	pool *sqlitex.Pool
	path string
}

// Open resolves the main repo from fromDir (via git rev-parse --git-common-dir),
// locates <main-repo>/.fellowship/fellowship.db, and opens a connection pool.
func Open(fromDir string) (*DB, error) {
	mainRepo, err := resolveMainRepo(fromDir)
	if err != nil {
		return nil, fmt.Errorf("db: resolve main repo: %w", err)
	}
	dbPath := filepath.Join(mainRepo, ".fellowship", "fellowship.db")
	return OpenPath(dbPath)
}

// OpenPath opens a DB at the given file path.
func OpenPath(dbPath string) (*DB, error) {
	pool, err := sqlitex.NewPool(dbPath, sqlitex.PoolOptions{
		PoolSize: 1,
		Flags:    sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenWAL | sqlite.OpenNoMutex,
	})
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", dbPath, err)
	}

	d := &DB{pool: pool, path: dbPath}

	// Enable foreign keys and apply schema.
	if err := d.WithConn(context.Background(), func(conn *Conn) error {
		if err := sqlitex.ExecuteTransient(conn, "PRAGMA foreign_keys = ON", nil); err != nil {
			return err
		}
		if err := sqlitex.ExecuteTransient(conn, "PRAGMA journal_mode = WAL", nil); err != nil {
			return err
		}
		return applySchema(conn)
	}); err != nil {
		pool.Close()
		return nil, err
	}

	return d, nil
}

// OpenMemory opens an in-memory DB with the full schema applied.
func OpenMemory() (*DB, error) {
	pool, err := sqlitex.NewPool("file::memory:?mode=memory", sqlitex.PoolOptions{
		PoolSize: 1,
		Flags:    sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenMemory | sqlite.OpenNoMutex,
	})
	if err != nil {
		return nil, fmt.Errorf("db: open memory: %w", err)
	}

	d := &DB{pool: pool, path: ":memory:"}

	if err := d.WithConn(context.Background(), func(conn *Conn) error {
		if err := sqlitex.ExecuteTransient(conn, "PRAGMA foreign_keys = ON", nil); err != nil {
			return err
		}
		return applySchema(conn)
	}); err != nil {
		pool.Close()
		return nil, err
	}

	return d, nil
}

// Close releases all connections in the pool.
func (d *DB) Close() error {
	return d.pool.Close()
}

// Path returns the database file path (":memory:" for in-memory DBs).
func (d *DB) Path() string {
	return d.path
}

// WithConn borrows a connection from the pool, calls fn, and returns it.
func (d *DB) WithConn(ctx context.Context, fn func(conn *Conn) error) error {
	conn, err := d.pool.Take(ctx)
	if err != nil {
		return fmt.Errorf("db: take conn: %w", err)
	}
	defer d.pool.Put(conn)

	// Ensure foreign keys are enabled per-connection.
	if err := sqlitex.ExecuteTransient(conn, "PRAGMA foreign_keys = ON", nil); err != nil {
		return err
	}

	return fn(conn)
}

// WithTx runs fn inside an IMMEDIATE transaction. If fn returns an error,
// the transaction is rolled back; otherwise it is committed.
func (d *DB) WithTx(ctx context.Context, fn func(conn *Conn) error) error {
	return d.WithConn(ctx, func(conn *Conn) error {
		endFn, err := sqlitex.ImmediateTransaction(conn)
		if err != nil {
			return fmt.Errorf("db: begin tx: %w", err)
		}
		fnErr := fn(conn)
		endFn(&fnErr)
		return fnErr
	})
}

// resolveMainRepo returns the main repo root from any worktree or the main repo itself.
func resolveMainRepo(fromDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = fromDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	gitCommon := strings.TrimSpace(string(out))

	// --git-common-dir returns absolute or relative path to the shared .git dir.
	if !filepath.IsAbs(gitCommon) {
		gitCommon = filepath.Join(fromDir, gitCommon)
	}
	gitCommon = filepath.Clean(gitCommon)

	// The main repo root is the parent of the .git directory.
	if filepath.Base(gitCommon) == ".git" {
		return filepath.Dir(gitCommon), nil
	}
	// For bare repos or unusual layouts, go up one level.
	return filepath.Dir(gitCommon), nil
}
