// Package store is a profile's database, store.db in its folder: SQLite, in which each
// feature keeps its own tables (docs/store.md). One worker runs every read and write, in the
// order asked, each a transaction of its own, so the main thread never waits on the database
// and a read sees every write asked before it.
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"projectb/timeline"
)

// Store is one profile's database and its worker.
type Store struct {
	db   *sql.DB
	jobs chan func()
	done chan struct{}
}

// Tx is a job's transaction.
type Tx = sql.Tx

// Open opens dir/store.db, making it when it isn't there, and starts its worker.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "store.db")
	// The driver keeps each statement prepared once, so a query asked again isn't parsed again.
	db, err := sql.Open("sqlite3", "file:"+(&url.URL{Path: path}).EscapedPath()+"?_journal_mode=WAL&_synchronous=NORMAL&_stmt_cache_size=64")
	if err != nil {
		return nil, err
	}
	// One connection, the worker's: nothing else reads or writes.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema (feature TEXT PRIMARY KEY, step INTEGER NOT NULL) WITHOUT ROWID`); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: %s: %w", path, err)
	}
	s := &Store{db: db, jobs: make(chan func(), 4096), done: make(chan struct{})}
	go s.work()
	return s, nil
}

// Do runs job on the worker, in a transaction that commits when it returns nil and rolls back
// when it returns an error, which goes to the timeline. A job's answer goes back to the main
// thread by whatever the feature posts it with.
func (s *Store) Do(job func(*Tx) error) {
	s.jobs <- func() { s.run(job) }
}

// Wait is Do, and returns once the job has run, with its error. Only for what is small and
// needed at start: the main thread waits meanwhile.
func (s *Store) Wait(job func(*Tx) error) error {
	got := make(chan error, 1)
	s.jobs <- func() { got <- s.run(job) }
	return <-got
}

// Schema brings a feature's tables up to date, on the worker, before any job asked after
// it: the steps the profile has not had run in order, in one transaction that ends by
// keeping the number of the last. A step, once released, never changes; a change is a step
// added at the end.
func (s *Store) Schema(feature string, steps ...func(*Tx) error) {
	s.Do(func(tx *Tx) error {
		had := 0
		err := tx.QueryRow(`SELECT step FROM schema WHERE feature = ?`, feature).Scan(&had)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if had >= len(steps) {
			return nil
		}
		for i := had; i < len(steps); i++ {
			if err := steps[i](tx); err != nil {
				return fmt.Errorf("%s, step %d: %w", feature, i+1, err)
			}
		}
		_, err = tx.Exec(`INSERT INTO schema (feature, step) VALUES (?, ?) ON CONFLICT (feature) DO UPDATE SET step = excluded.step`, feature, len(steps))
		if err == nil {
			timeline.Infof("store: %s from step %d to %d", feature, had, len(steps))
		}
		return err
	})
}

// Close runs the jobs already asked for, then closes the database.
func (s *Store) Close() {
	close(s.jobs)
	<-s.done
	if err := s.db.Close(); err != nil {
		timeline.Errorf("store: %v", err)
	}
}

func (s *Store) work() {
	defer close(s.done)
	for job := range s.jobs {
		job()
	}
}

func (s *Store) run(job func(*Tx) error) error {
	tx, err := s.db.Begin()
	if err == nil {
		if err = job(tx); err == nil {
			err = tx.Commit()
		} else {
			tx.Rollback()
		}
	}
	if err != nil {
		timeline.Errorf("store: %v", err)
	}
	return err
}

// NewID is a UUIDv7 as text: the time in milliseconds, then random bits, so an id made later
// sorts later. Every row that may sync one day has one, made once.
func NewID() string {
	var b [16]byte
	rand.Read(b[:])
	ms := time.Now().UnixMilli()
	for i := range 6 {
		b[i] = byte(ms >> (40 - 8*i))
	}
	b[6] = b[6]&0x0f | 0x70
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// Now is the time a row was edited: Unix milliseconds.
func Now() int64 { return time.Now().UnixMilli() }
