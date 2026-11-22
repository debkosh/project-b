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
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/sudiptadeb/oskit/internal/portal"
)

// ErrNoPortal is a desktop that gave no key: no Secret portal, one with no backend (GNOME
// Keyring, KWallet), or one that refused. Nothing is kept in a file instead.
var ErrNoPortal = errors.New("secret: the desktop's Secret portal gave no key")

// wait is the most the portal may take when ctx says nothing: the desktop may ask the person to
// unlock their keyring first.
const wait = 2 * time.Minute

// linux is the Secret portal, org.freedesktop.portal.Secret: it gives each app a secret of its
// own from the desktop's keyring, the same at every call, made at the first. The key is drawn
// from it with HKDF-SHA256, its info named after name ("Project B": "project-b device key"),
// so one app keeps a key per name.
type linux struct{}

// New is this OS's Store.
func New() Store { return linux{} }

func (linux) DeviceKey(ctx context.Context, name string, _ bool) ([]byte, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, wait)
		defer cancel()
	}
	s, err := retrieve(ctx)
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
func retrieve(ctx context.Context) ([]byte, error) {
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
	code, _, err := portal.Request(ctx, func(_ *dbus.Conn, p dbus.BusObject, token string) (dbus.ObjectPath, error) {
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
	select {
	case b := <-read:
		return b, nil
	case <-ctx.Done():
		return nil, errors.New("the portal never wrote the secret")
	}
}
