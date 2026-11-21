//go:build linux && !android

package keys

import (
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// ErrNoSecretPortal is the device key's answer on a desktop that can't give one: no portal,
// or a portal with no Secret backend, or one that refused. What needs the key, sync and saved
// passwords, says so and doesn't start; no key is ever kept in a file instead.
var ErrNoSecretPortal = errors.New("keys: the desktop's Secret portal gave no key")

// secretPortalWait is how long the portal may take, which includes the desktop asking the
// person to unlock their keyring.
const secretPortalWait = 2 * time.Minute

// SecretPortal is the device key on Linux (docs/sync.md): the desktop's Secret portal,
// org.freedesktop.portal.Secret, gives each app a secret of its own from the desktop's
// keyring (GNOME Keyring, KWallet), the same at every call, made at the first; the key is
// drawn from it with HKDF-SHA256. So create changes nothing.
func SecretPortal() Device {
	return func(bool) ([]byte, error) {
		secret, err := retrieveSecret()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrNoSecretPortal, err)
		}
		if len(secret) < 16 {
			return nil, fmt.Errorf("%w: a secret of %d bytes", ErrNoSecretPortal, len(secret))
		}
		return hkdf.Key(sha256.New, secret, nil, "project-b device key", 32)
	}
}

// retrieveSecret asks the portal for the app's secret: RetrieveSecret writes it to a pipe
// we hand it, and its Request answers with Response once it has (0 is done).
func retrieveSecret() ([]byte, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	token := make([]byte, 8)
	rand.Read(token)
	handle := "pb_" + hex.EncodeToString(token)
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	request := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + handle)
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(request),
		dbus.WithMatchInterface("org.freedesktop.portal.Request"), dbus.WithMatchMember("Response")); err != nil {
		return nil, err
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	read := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(r)
		read <- b
	}()

	portal := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	var got dbus.ObjectPath
	err = portal.Call("org.freedesktop.portal.Secret.RetrieveSecret", 0, dbus.UnixFD(w.Fd()),
		map[string]dbus.Variant{"handle_token": dbus.MakeVariant(handle)}).Store(&got)
	w.Close() // The portal has its own copy; the pipe ends when it closes that.
	if err != nil {
		return nil, err
	}
	if got != request {
		return nil, fmt.Errorf("the portal answers at %s, not %s", got, request)
	}

	timeout := time.After(secretPortalWait)
	for {
		select {
		case s := <-signals:
			if s.Path != request || s.Name != "org.freedesktop.portal.Request.Response" || len(s.Body) == 0 {
				continue
			}
			if code, _ := s.Body[0].(uint32); code != 0 {
				return nil, fmt.Errorf("the portal answered %d", code)
			}
			select {
			case b := <-read:
				return b, nil
			case <-timeout:
				return nil, errors.New("the portal never wrote the secret")
			}
		case <-timeout:
			return nil, errors.New("the portal didn't answer")
		}
	}
}
