//go:build !android

package open

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/sudiptadeb/oskit/internal/portal"
	"github.com/sudiptadeb/oskit/internal/portaltest"
)

func TestMain(m *testing.M) { os.Exit(portaltest.Run(m)) }

// openURIPortal is the OpenURI portal, which says what it was handed: the method and the file
// behind the descriptor.
type openURIPortal struct{ got chan string }

func (p *openURIPortal) opened(method string, fd dbus.UnixFD) {
	path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
	if err != nil {
		path = err.Error()
	}
	syscall.Close(int(fd))
	p.got <- method + " " + path
}

func (p *openURIPortal) OpenFile(parent string, fd dbus.UnixFD, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	p.opened("OpenFile", fd)
	return "/org/freedesktop/portal/desktop/request/1_1/t", nil
}

func (p *openURIPortal) OpenDirectory(parent string, fd dbus.UnixFD, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	p.opened("OpenDirectory", fd)
	return "/org/freedesktop/portal/desktop/request/1_1/t", nil
}

func TestOpenAndReveal(t *testing.T) {
	portaltest.Need(t)
	p := &openURIPortal{got: make(chan string, 2)}
	if err := portaltest.Desktop.Export(p, portal.Path, "org.freedesktop.portal.OpenURI"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(file, []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := New()
	for _, c := range []struct {
		method string
		call   func(string)
	}{{"OpenFile", o.OpenFile}, {"OpenDirectory", o.Reveal}} {
		c.call(file)
		select {
		case got := <-p.got:
			if got != c.method+" "+file {
				t.Errorf("the portal got %q, want %q", got, c.method+" "+file)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the portal got nothing", c.method)
		}
	}
}

func TestDefaultApp(t *testing.T) {
	list := `[Added Associations]
x-scheme-handler/https=io.github.sudiptadeb.project-b.desktop;

[Default Applications]
text/html=firefox.desktop
x-scheme-handler/https = io.github.sudiptadeb.project-b.desktop;firefox.desktop;
x-scheme-handler/http=firefox.desktop;
`
	for mime, want := range map[string]string{
		"x-scheme-handler/https": "io.github.sudiptadeb.project-b.desktop",
		"x-scheme-handler/http":  "firefox.desktop",
		"text/html":              "firefox.desktop",
		"x-scheme-handler/ftp":   "",
	} {
		if got := defaultApp(list, mime); got != want {
			t.Errorf("%s: %q, want %q", mime, got, want)
		}
	}
	if got := defaultApp("[Added Associations]\nx-scheme-handler/https=a.desktop;\n", "x-scheme-handler/https"); got != "" {
		t.Errorf("an added association is not the default: %q", got)
	}
}

// The default browser is the Flatpak's desktop file in the settings folder outside it, or the
// one SetDesktopID named outside a Flatpak; asking changes nothing and answers what is so.
func TestIsDefaultBrowser(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOST_XDG_CONFIG_HOME", dir)
	t.Setenv("FLATPAK_ID", "org.example.Browser")
	o := New()
	if o.IsDefaultBrowser() {
		t.Error("with no mimeapps.list, the default browser")
	}
	os.WriteFile(filepath.Join(dir, "mimeapps.list"), []byte("[Default Applications]\nx-scheme-handler/https=org.example.Browser.desktop\n"), 0o644)
	if !o.IsDefaultBrowser() {
		t.Error("not the default browser mimeapps.list names")
	}
	t.Setenv("FLATPAK_ID", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	if o.IsDefaultBrowser() {
		t.Error("outside a Flatpak with no desktop file named, the default browser")
	}
	SetDesktopID("org.example.Browser.desktop")
	defer SetDesktopID("")
	if !o.IsDefaultBrowser() {
		t.Error("outside a Flatpak, not the default browser SetDesktopID names")
	}
	got := make(chan bool, 1)
	o.AskDefaultBrowser(func(is bool) { got <- is })
	if !<-got {
		t.Error("asking answers what is so")
	}
}
