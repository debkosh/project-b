//go:build !android

package secret

import (
	"bytes"
	"context"
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"os"
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/sudiptadeb/oskit/internal/portal"
	"github.com/sudiptadeb/oskit/internal/portaltest"
)

func TestMain(m *testing.M) { os.Exit(portaltest.Run(m)) }

// secretPortal writes the app's secret to the descriptor, then answers its Request, as the
// portal does; refuse answers 2 and writes nothing.
type secretPortal struct {
	secret []byte
	refuse bool
}

func (p *secretPortal) RetrieveSecret(sender dbus.Sender, fd dbus.UnixFD, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	f := os.NewFile(uintptr(fd), "secret")
	if !p.refuse {
		f.Write(p.secret)
	}
	f.Close()
	token, _ := options["handle_token"].Value().(string)
	code := uint32(0)
	if p.refuse {
		code = 2
	}
	return portaltest.Respond(sender, token, code, map[string]dbus.Variant{}), nil
}

// The key is the portal's secret through HKDF with the name's info, as 0.3.8's was ("project-b
// device key"), so a profile it wrapped opens; a refusal is ErrNoPortal.
func TestDeviceKeyFromThePortal(t *testing.T) {
	portaltest.Need(t)
	p := &secretPortal{secret: []byte("0123456789abcdef0123456789abcdef")}
	if err := portaltest.Desktop.Export(p, portal.Path, "org.freedesktop.portal.Secret"); err != nil {
		t.Fatal(err)
	}
	key, err := New().DeviceKey(context.Background(), "Project B", false)
	want, _ := hkdf.Key(sha256.New, p.secret, nil, "project-b device key", 32)
	if err != nil || !bytes.Equal(key, want) {
		t.Fatalf("key %x, %v; want %x", key, err, want)
	}
	p.refuse = true
	if _, err := New().DeviceKey(context.Background(), "Project B", true); !errors.Is(err, ErrNoPortal) {
		t.Fatalf("a refusal: %v", err)
	}
}
