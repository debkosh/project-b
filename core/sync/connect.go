package sync

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/sudiptadeb/synckosh"
	"github.com/sudiptadeb/synckosh/client"
)

// ErrPassphrase is Connect's answer for a passphrase that opens none of the bucket's sealed
// copies of its key.
var ErrPassphrase = synckosh.ErrPassphrase

// passphraseLabel is the label of the sealed copies of the sync key Connect and
// ChangePassphrase make.
const passphraseLabel = "Passphrase"

// Proposal is what an address would connect to.
type Proposal struct {
	// Address is the address as it is connected to.
	Address string `json:"address"`
	// Host is all of it that is shown.
	Host string `json:"host"`
	// NeedsToken says the address holds no token, so one is asked for.
	NeedsToken bool `json:"needsToken"`
}

// Link is the address in a deep link, project-b://sync?address=<the address, percent-encoded>.
// It only reads it: the address still goes to Propose.
func Link(link string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil || u.Scheme != "project-b" || u.Host != "sync" {
		return "", errors.New("sync: not a project-b://sync link")
	}
	address := u.Query().Get("address")
	if address == "" {
		return "", fmt.Errorf("%w: the link has none", ErrAddress)
	}
	return address, nil
}

// Propose checks an address and says what it would connect to, changing nothing: https, with a
// host, and no user, query or fragment, else ErrAddress. One whose path ends in /sync needs its token given;
// one that goes on past it carries its token, as https://<host>/sync/<token> does.
func Propose(address string) (Proposal, error) {
	u, err := url.Parse(strings.TrimSpace(address))
	switch {
	case err != nil || u.Scheme != "https":
		return Proposal{}, fmt.Errorf("%w: it must start with https://", ErrAddress)
	case u.Host == "":
		return Proposal{}, fmt.Errorf("%w: it has no host", ErrAddress)
	case u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "":
		return Proposal{}, fmt.Errorf("%w: it can't have a user, a query or a fragment", ErrAddress)
	}
	path := strings.TrimRight(u.EscapedPath(), "/")
	return Proposal{Address: "https://" + u.Host + path, Host: u.Host, NeedsToken: path == "/sync" || strings.HasSuffix(path, "/sync")}, nil
}

// check is the address and token a bucket is reached with: a token only for an address that
// needs one, and then always one, else ErrNeedsToken.
func check(address, token string) (string, string, error) {
	p, err := Propose(address)
	if err != nil {
		return "", "", err
	}
	token = strings.TrimSpace(token)
	switch {
	case p.NeedsToken && token == "":
		return "", "", ErrNeedsToken
	case !p.NeedsToken:
		token = ""
	}
	return p.Address, token, nil
}

// Inspect answers whether the bucket is new, empty, so a new passphrase is asked for twice, or
// holds items, so its passphrase is asked for once. It changes nothing.
func (m *Manager) Inspect(address, token string, answer func(isNew bool, err error)) {
	address, token, err := check(address, token)
	if err != nil {
		answer(false, err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(m.ctx, time.Minute)
		defer cancel()
		p, err := (&client.Client{URL: address, Token: token, HTTP: m.http}).Pull(ctx, "", 0, 1)
		m.post(func() { answer(err == nil && len(p.Items) == 0, err) })
	}()
}

// Connect turns sync on with the bucket: it makes the bucket's sync key when the bucket is
// empty, with a copy sealed under the passphrase, or opens the copy with it when the bucket
// holds items. A wrong passphrase is ErrPassphrase and changes nothing. It keeps the address,
// the token and the sync key sealed, and hands every row to synckosh once; done hears once it
// is on, and the first run follows, which the status shows.
func (m *Manager) Connect(address, token, passphrase string, done func(error)) {
	address, token, err := check(address, token)
	if err == nil && passphrase == "" {
		err = errors.New("sync: a passphrase is needed")
	}
	if err != nil {
		done(err)
		return
	}
	m.ops <- func() {
		err := m.connect(address, token, passphrase)
		m.post(func() { done(err) })
		if err == nil {
			m.run(WhyOn)
		}
	}
}

// connect is Connect's work, on the sync goroutine, up to its first run.
func (m *Manager) connect(address, token, passphrase string) error {
	if m.conn != nil {
		return errors.New("sync: on already; disconnect first")
	}
	if err := m.keys.Check(); err != nil {
		return lockedError(err)
	}
	ctx, cancel := context.WithTimeout(m.ctx, time.Minute)
	defer cancel()
	c := &client.Client{URL: address, Token: token, HTTP: m.http}
	key, err := client.Create(ctx, c, passphrase, passphraseLabel)
	if errors.Is(err, client.ErrHasKey) {
		key, err = client.Join(ctx, c, passphrase)
	}
	if err != nil {
		return err
	}
	return m.begin(address, token, key)
}

// ChangePassphrase puts a copy of the sync key sealed under passphrase and deletes the
// others. It needs no old passphrase: the device holds the sync key.
func (m *Manager) ChangePassphrase(passphrase string, done func(error)) {
	if passphrase == "" {
		done(errors.New("sync: a passphrase is needed"))
		return
	}
	m.do(done, func() error {
		c := m.conn
		if c == nil {
			return errors.New("sync: off")
		}
		// Every copy there is, first.
		if err := m.run(WhyPassphrase); err != nil {
			return err
		}
		copy, err := c.key.SealedCopy(passphrase, passphraseLabel)
		if err != nil {
			return err
		}
		put, err := c.replica.Put(copy)
		if err != nil {
			return err
		}
		copies, err := c.replica.Items(synckosh.KeysCollection)
		if err != nil {
			return err
		}
		for _, it := range copies {
			if it.GUID != put[0].GUID {
				if err := c.replica.Delete(synckosh.KeysCollection, it.GUID); err != nil {
					return err
				}
			}
		}
		return m.run(WhyPassphrase)
	})
}

// Disconnect stops syncing and forgets the address, the token and the sync key. Every row
// stays.
func (m *Manager) Disconnect(done func(error)) { m.do(done, m.end) }

// StartFresh is for a profile whose key is locked: keys.StartFresh deletes what was sealed,
// so sync is forgotten and off, and makes a new key. History and bookmarks stay.
func (m *Manager) StartFresh(done func(error)) {
	m.do(done, func() error {
		if err := m.keys.StartFresh(); err != nil {
			return err
		}
		return m.end()
	})
}
