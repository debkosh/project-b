//go:build !android

package secret

import (
	"context"
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/sudiptadeb/oskit/internal/portal"
)

// ErrNoPortal is a desktop that gave no key: no Secret portal, one with no backend (GNOME
// Keyring, KWallet), or one that refused. Nothing is kept in a file instead.
var ErrNoPortal = errors.New("secret: the desktop's Secret portal gave no key")

// linux is the Secret portal, org.freedesktop.portal.Secret: it gives each app a secret of its
// own from the desktop's keyring, the same at every call, made at the first. The key is drawn
// from it with HKDF-SHA256, its info named after name ("Project B": "project-b device key"),
// so one app keeps a key per name.
//
// The desktop may ask the person to unlock their keyring first, and the call waits for as
// long as that question is up. ctx isn't waited on, as on the Mac: the question can't be
// taken back. Closing our Request while GNOME's question is up leaves the question there,
// and once it is answered gnome-keyring answers every later request, from any app, "The
// operation was cancelled" (the portal's code 2, at once), with the keyring unlocked, until
// it starts again at the next login (gnome-keyring 46, 2026-11-24). A process that ends
// while the question is up sets off the same, as the portal closes its requests then.
type linux struct{}

// New is this OS's Store.
func New() Store { return linux{} }

func (linux) DeviceKey(_ context.Context, name string, _ bool) ([]byte, error) {
	s, err := retrieve()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoPortal, err)
	}
	if len(s) < 16 {
		return nil, fmt.Errorf("%w: a secret of %d bytes", ErrNoPortal, len(s))
	}
	return hkdf.Key(sha256.New, s, nil, info(name), 32)
}

// info is the HKDF info for name: "Project B" is "project-b device key".
func info(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-")) + " device key"
}

// retrieve asks the portal for the app's secret: RetrieveSecret writes it to a pipe we hand
// it, and its Request answers once it has (0 is done).
func retrieve() ([]byte, error) {
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
	code, _, err := portal.Request(context.Background(), func(_ *dbus.Conn, p dbus.BusObject, token string) (dbus.ObjectPath, error) {
		var request dbus.ObjectPath
		err := p.Call("org.freedesktop.portal.Secret.RetrieveSecret", 0, dbus.UnixFD(w.Fd()),
			map[string]dbus.Variant{"handle_token": dbus.MakeVariant(token)}).Store(&request)
		w.Close() // The portal has its own copy; the pipe ends when it closes that.
		return request, err
	})
	if err != nil {
		w.Close()
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("the portal answered %d", code)
	}
	return <-read, nil
}
