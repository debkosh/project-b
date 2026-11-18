package store

import (
	"errors"
	"strings"
	"testing"
)

func open(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// count is how many rows a table has, or -1 when it isn't there.
func count(t *testing.T, s *Store, table string) int {
	n := -1
	s.Wait(func(tx *Tx) error {
		if tx.QueryRow(`SELECT count(*) FROM `+table).Scan(&n) != nil {
			n = -1
		}
		return nil
	})
	return n
}

// A feature's steps run once each, in order; a later start runs only a step added since.
func TestSchemaStepsRunOnce(t *testing.T) {
	dir := t.TempDir()
	var ran []string
	step := func(name, sql string) func(*Tx) error {
		return func(tx *Tx) error {
			ran = append(ran, name)
			_, err := tx.Exec(sql)
			return err
		}
	}
	one := step("one", `CREATE TABLE notes (text TEXT)`)
	two := step("two", `INSERT INTO notes VALUES ('kept')`)
	s := open(t, dir)
	s.Schema("notes", one)
	s.Schema("notes", one)
	s.Close()

	s = open(t, dir)
	s.Schema("notes", one, two)
	if n := count(t, s, "notes"); n != 1 {
		t.Fatalf("notes = %d", n)
	}
	s.Close()
	s = open(t, dir)
	defer s.Close()
	s.Schema("notes", one, two)
	s.Wait(func(*Tx) error { return nil })
	if strings.Join(ran, " ") != "one two" {
		t.Fatalf("ran %v", ran)
	}
}

// A step that fails leaves nothing of the feature's steps, and the next start tries them again.
func TestAFailedStepLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	s := open(t, dir)
	made := func(tx *Tx) error {
		_, err := tx.Exec(`CREATE TABLE pages (url TEXT)`)
		return err
	}
	s.Schema("pages", made, func(*Tx) error { return errors.New("no") })
	if n := count(t, s, "pages"); n != -1 {
		t.Fatalf("pages is there after a failed step: %d", n)
	}
	s.Close()
	s = open(t, dir)
	defer s.Close()
	s.Schema("pages", made, func(*Tx) error { return nil })
	if n := count(t, s, "pages"); n != 0 {
		t.Fatalf("pages = %d", n)
	}
}

// Jobs run in the order asked, and one that fails takes back what it wrote.
func TestJobsInOrderEachATransaction(t *testing.T) {
	s := open(t, t.TempDir())
	defer s.Close()
	s.Schema("log", func(tx *Tx) error {
		_, err := tx.Exec(`CREATE TABLE log (n INTEGER)`)
		return err
	})
	for i := range 100 {
		s.Do(func(tx *Tx) error {
			_, err := tx.Exec(`INSERT INTO log VALUES (?)`, i)
			return err
		})
	}
	s.Do(func(tx *Tx) error {
		tx.Exec(`INSERT INTO log VALUES (-1)`)
		return errors.New("taken back")
	})
	var sum, n int
	s.Wait(func(tx *Tx) error { return tx.QueryRow(`SELECT sum(n), count(*) FROM log`).Scan(&sum, &n) })
	if n != 100 || sum != 4950 {
		t.Fatalf("%d rows adding up to %d", n, sum)
	}
}

// The build has FTS5 and its trigram tokenizer, which the history's search needs.
func TestTrigramIndex(t *testing.T) {
	s := open(t, t.TempDir())
	defer s.Close()
	var got string
	err := s.Wait(func(tx *Tx) error {
		if _, err := tx.Exec(`CREATE VIRTUAL TABLE t USING fts5(text, tokenize = 'trigram')`); err != nil {
			return err
		}
		tx.Exec(`INSERT INTO t VALUES ('github.com'), ('gitlab.com')`)
		return tx.QueryRow(`SELECT text FROM t WHERE t MATCH '"thu"'`).Scan(&got)
	})
	if err != nil || got != "github.com" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestNewID(t *testing.T) {
	a, b := NewID(), NewID()
	if len(a) != 36 || a[14] != '7' || a == b || !strings.ContainsAny(a[19:20], "89ab") {
		t.Fatalf("ids %s %s", a, b)
	}
}
