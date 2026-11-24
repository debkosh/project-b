package sync

import (
	"errors"
	"time"

	"github.com/sudiptadeb/synckosh"
)

// The backup code (docs/sync.md, "Backup code"): a second passphrase, random, that the app
// shows once for a person to keep apart from the device. It has a sealed copy of the sync
// key of its own in the bucket, as the passphrase has; the key itself is never shown. With
// it a device joins, and a person who forgot the passphrase gets back in and sets a new one.
// synckosh allows many; this app keeps one at a time.

// Backup is the bucket's backup code as the page shows it: that there is one, and when it
// was made, Unix milliseconds, 0 for one that doesn't say.
type Backup struct {
	Made int64 `json:"made"`
}

// backupLabel is what the server may read of a backup code's copy, as memd's page lists the
// keys that open a bucket: "Backup, 22 Nov".
func backupLabel(now time.Time) string { return "Backup, " + now.Format("2 Jan") }

// backup is the bucket's backup code as the page tells of it, the newest when it has more
// than one, nil for none. A code just made isn't told of until it is kept: nobody has said
// they saved it, and leaving its card takes it away, so until then the page says what was
// there before it.
func (m *Manager) backup() *Backup {
	c := m.conn
	if c == nil {
		return nil
	}
	all, err := c.replica.Passphrases()
	if err != nil {
		return nil
	}
	var out *Backup
	for _, p := range all {
		if !p.BackupCode || p.GUID == m.newBackup {
			continue
		}
		made := int64(0)
		if !p.Created.IsZero() {
			made = p.Created.UnixMilli()
		}
		if out == nil || made > out.Made {
			out = &Backup{Made: made}
		}
	}
	return out
}

// MakeBackupCode makes a backup code and answers with it, to show once: nothing keeps it.
// A code the bucket had before still opens it until KeepBackupCode, which a person's "I've
// saved it" is, takes it away; DropBackupCode takes the new one away instead.
func (m *Manager) MakeBackupCode(done func(code string, err error)) {
	code := ""
	m.do(func(err error) { done(code, err) }, func() error {
		c := m.conn
		if c == nil {
			return errors.New("sync: off")
		}
		// Every copy there is, first.
		if err := m.run(WhyBackupCode); err != nil {
			return err
		}
		made, copy, err := c.replica.AddBackupCode(backupLabel(time.Now()))
		if err != nil {
			return err
		}
		code, m.newBackup = made, copy.GUID
		return m.run(WhyBackupCode)
	})
}

// KeepBackupCode keeps the code MakeBackupCode made as the bucket's one backup code: every
// other one stops opening it.
func (m *Manager) KeepBackupCode(done func(error)) { m.settleBackup(true, done) }

// DropBackupCode takes the code MakeBackupCode made away again, and leaves the bucket's
// backup code, if it had one, as it was.
func (m *Manager) DropBackupCode(done func(error)) { m.settleBackup(false, done) }

func (m *Manager) settleBackup(keep bool, done func(error)) {
	m.do(done, func() error {
		c := m.conn
		if c == nil || m.newBackup == "" {
			return nil
		}
		made := m.newBackup
		m.newBackup = ""
		all, err := c.replica.Passphrases()
		if err != nil {
			return err
		}
		for _, p := range all {
			if p.BackupCode && (p.GUID == made) != keep {
				if err := c.replica.RemovePassphrase(p.GUID); err != nil {
					return err
				}
			}
		}
		// The change is in the profile; a run that can't reach the server now sends it later.
		m.run(WhyBackupCode)
		m.setStatus(func(s *Status) { s.Backup = m.backup() })
		return nil
	})
}

// CheckBackupCode answers whether what a person typed is the bucket's backup code, as of
// the last run. It asks no server.
func (m *Manager) CheckBackupCode(typed string, answer func(is bool, err error)) {
	is := false
	m.do(func(err error) { answer(is, err) }, func() error {
		c := m.conn
		if c == nil {
			return errors.New("sync: off")
		}
		// A passphrase opens a copy too: only what is written as a code is one.
		if _, ok := synckosh.NormalizeCode(typed); !ok {
			return nil
		}
		err := c.replica.CheckPassphrase(typed)
		if errors.Is(err, synckosh.ErrPassphrase) {
			return nil
		}
		is = err == nil
		return err
	})
}
