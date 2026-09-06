package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"

	"github.com/justinjdev/fellowship/cli/internal/datadir"
	"github.com/justinjdev/fellowship/cli/internal/gitutil"
)

// Conn is an alias for sqlite.Conn to simplify consumer imports.
type Conn = sqlite.Conn

// DB manages a SQLite connection pool for fellowship state.
type DB struct {
	pool *sqlitex.Pool
	path string
}

// ErrNoStore reports that the repo containing the starting directory has no
// fellowship store. It is not a failure: an ordinary repo that has never run
// `fellowship init` has no store, and hooks must allow in that case rather
// than block (and must not bring a store into existence by looking).
var ErrNoStore = errors.New("db: no fellowship store")

// ErrEmptyStore reports that the store file exists but is zero bytes.
//
// This is what deleting the store looks like when something recreates the file
// — and it used to be indistinguishable from a brand-new one: ensureSchema saw
// user_version 0, wrote the whole schema, and the hook that opened it reported
// "no quest here" for a store it had just built itself. An empty store is a
// destroyed store, never a fresh one; only `init` and `state init` may build a
// schema.
var ErrEmptyStore = errors.New("db: fellowship store is empty")

// StorePath returns the fellowship database path for the repo containing
// fromDir, without touching the filesystem.
//
// The directory name comes from the repo's own configuration (datadir.Resolve),
// not the ".fellowship" default: a project that sets dataDir kept its store in
// .fellowship while every guard, marker, and coordination write went to the
// configured directory, so half the fellowship looked at the wrong place.
func StorePath(fromDir string) (string, error) {
	mainRepo, err := resolveMainRepo(fromDir)
	if err != nil {
		// Outside any git repository there is no main repo to key a store
		// to, so there is no store: the same answer as a repo without one,
		// and one the hooks allow. It used to surface as a generic failure,
		// which gate hooks read as "the store cannot be read" and blocked —
		// every Bash call in a plain directory, cleanup included.
		if errors.Is(err, gitutil.ErrNotARepository) {
			return "", fmt.Errorf("%w: %s is not inside a git repository", ErrNoStore, fromDir)
		}
		return "", fmt.Errorf("db: resolve main repo: %w", err)
	}
	return filepath.Join(mainRepo, datadir.Resolve(mainRepo), datadir.StoreFileName), nil
}

// Open resolves the main repo from fromDir (via git rev-parse --git-common-dir),
// locates <main-repo>/.fellowship/fellowship.db, and opens a connection pool,
// CREATING the store if it does not exist. Only store-creating commands
// (`fellowship init`, `fellowship state init`, `fellowship migrate`) may call
// this; everything else — hooks above all — must use OpenExisting so that
// merely running the binary in an unrelated repo leaves no .fellowship behind.
func Open(fromDir string) (*DB, error) {
	dbPath, err := StorePath(fromDir)
	if err != nil {
		return nil, err
	}
	return OpenPath(dbPath)
}

// OpenExisting opens the store for the repo containing fromDir and never
// creates one. It returns an error wrapping ErrNoStore when no database file
// exists, so callers can tell "no fellowship here" (allow) apart from "the
// store is present but unreadable" (fail closed).
func OpenExisting(fromDir string) (*DB, error) {
	dbPath, err := StorePath(fromDir)
	if err != nil {
		return nil, err
	}
	return openExistingPath(dbPath)
}

// openExistingPath is OpenExisting once the store path is known.
func openExistingPath(dbPath string) (*DB, error) {
	if err := statStore(dbPath); err != nil {
		return nil, err
	}
	return openPath(dbPath, openOptions{migrate: true})
}

// statStore reports whether a store file is there to be opened: ErrNoStore when
// it is absent, ErrEmptyStore when it is zero bytes (a destroyed store, which
// must never be mistaken for a fresh one).
func statStore(dbPath string) error {
	info, err := os.Stat(dbPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w at %s", ErrNoStore, dbPath)
		}
		return fmt.Errorf("db: stat %s: %w", dbPath, err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%w at %s", ErrEmptyStore, dbPath)
	}
	return nil
}

// OpenForHook opens the store for a hook: it never creates one, and it never
// migrates. A hook is a decision, not an upgrade path — running the schema
// ladder from inside one would have every tool call racing to rewrite the store
// it is reading, and it is how a zero-byte store used to be rebuilt into a
// brand-new one. An out-of-date store is reported (ErrSchemaOutOfDate) so the
// caller can fail closed and tell the user to run `fellowship init`.
//
// readOnly opens the connection read-only as well, for the hooks that only
// decide. A read-only connection cannot create the -shm file a WAL database
// wants when one is missing, so that open falls back to read-write rather than
// turning a recoverable state into a block.
func OpenForHook(fromDir string, readOnly bool) (*DB, error) {
	dbPath, err := StorePath(fromDir)
	if err != nil {
		return nil, err
	}
	return openForHookPath(dbPath, readOnly)
}

// openForHookPath is OpenForHook once the store path is known.
func openForHookPath(dbPath string, readOnly bool) (*DB, error) {
	if err := statStore(dbPath); err != nil {
		return nil, err
	}
	if readOnly {
		d, err := openPath(dbPath, openOptions{readOnly: true})
		if err == nil {
			return d, nil
		}
		// A schema verdict — out of date, or newer than this binary — is the
		// answer, not an obstacle to work around; reopening read-write would
		// only reach it again. Anything else (a WAL database whose -shm a
		// read-only connection cannot create) falls back to the writable open.
		if errors.Is(err, ErrSchemaOutOfDate) || errors.Is(err, ErrSchemaNewer) {
			return nil, err
		}
	}
	return openPath(dbPath, openOptions{})
}

// OpenPath opens a DB at the given file path, creating it if needed.
func OpenPath(dbPath string) (*DB, error) {
	return openPath(dbPath, openOptions{create: true, migrate: true})
}

// openOptions selects how openPath treats the file: whether it may bring one
// into existence, whether the connection may write, and whether it may run the
// schema ladder.
type openOptions struct {
	create   bool
	readOnly bool
	migrate  bool
}

func openPath(dbPath string, o openOptions) (*DB, error) {
	flags := sqlite.OpenReadWrite | sqlite.OpenWAL
	if o.readOnly {
		flags = sqlite.OpenReadOnly | sqlite.OpenWAL
	}
	if o.create {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, fmt.Errorf("db: mkdir %s: %w", filepath.Dir(dbPath), err)
		}
		flags |= sqlite.OpenCreate
	}

	pool, err := sqlitex.NewPool(dbPath, sqlitex.PoolOptions{
		PoolSize: 1,
		Flags:    flags,
	})
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", dbPath, err)
	}

	d := &DB{pool: pool, path: dbPath}

	// Enable foreign keys and settle the schema. Only the store-creating and
	// -upgrading commands migrate; everything else checks the version and
	// refuses to touch a store it would have to rewrite.
	if err := d.WithConn(context.Background(), func(conn *Conn) error {
		if err := sqlitex.ExecuteTransient(conn, "PRAGMA foreign_keys = ON", nil); err != nil {
			return err
		}
		if o.migrate {
			return ensureSchema(conn)
		}
		return checkSchemaCurrent(conn)
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
		Flags:    sqlite.OpenReadWrite | sqlite.OpenCreate | sqlite.OpenMemory,
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

// WithTx runs fn inside an IMMEDIATE transaction. If fn returns an error, the
// transaction is rolled back; otherwise it is committed.
//
// The transaction is ended from a defer, so a panic inside fn cannot leave it
// open on a connection that goes straight back into the pool. The error is
// seeded non-nil for exactly that case: endFn commits only when the error it is
// handed is nil, so an fn that never returns rolls back. A named return keeps a
// commit failure visible to the caller.
func (d *DB) WithTx(ctx context.Context, fn func(conn *Conn) error) error {
	return d.WithConn(ctx, func(conn *Conn) (err error) {
		endFn, txErr := sqlitex.ImmediateTransaction(conn)
		if txErr != nil {
			return fmt.Errorf("db: begin tx: %w", txErr)
		}
		err = errors.New("db: transaction abandoned")
		defer func() { endFn(&err) }()
		err = fn(conn)
		return err
	})
}

// resolveMainRepo returns the main repo root from any worktree or the main repo
// itself. It is gitutil.MainRepoRoot, which every other caller shares, so the
// store, the data directory and the guards cannot disagree about the root.
func resolveMainRepo(fromDir string) (string, error) {
	return gitutil.MainRepoRoot(fromDir)
}
