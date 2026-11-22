//go:build !android

package platform

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// A desktop of the tests' own: a private session bus for the whole package, as the portal's
// code keeps one connection to it, and on it the desktop, which owns the portal's name and
// serves what each test exports.
var bus string
var desktop *dbus.Conn

func TestMain(m *testing.M) {
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		fmt.Println("no dbus-daemon here: the portal's tests don't run")
		os.Exit(m.Run())
	}
	cmd := exec.Command(daemon, "--session", "--nofork", "--print-address")
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	addr, err := bufio.NewReader(out).ReadString('\n')
	if err == nil {
		bus = strings.TrimSpace(addr)
		os.Setenv("DBUS_SESSION_BUS_ADDRESS", bus)
		desktop, err = dbus.Connect(bus)
	}
	if err == nil {
		_, err = desktop.RequestName(portalName, dbus.NameFlagDoNotQueue)
	}
	code := 1
	if err != nil {
		fmt.Println(err)
	} else {
		code = m.Run()
	}
	cmd.Process.Kill()
	cmd.Wait()
	os.Exit(code)
}

func needBus(t *testing.T) {
	if desktop == nil {
		t.Skip("no session bus of the tests' own")
	}
}

// newPortal is a portal of version 2, with ReadOne.
type newPortal struct{ scheme uint32 }

func (p *newPortal) ReadOne(ns, key string) (dbus.Variant, *dbus.Error) {
	if ns != appearance || key != colorScheme {
		return dbus.Variant{}, dbus.NewError("org.freedesktop.portal.Error.NotFound", nil)
	}
	return dbus.MakeVariant(p.scheme), nil
}

// oldPortal has only Read, whose answer is a variant in a variant.
type oldPortal struct{ scheme uint32 }

func (p *oldPortal) Read(ns, key string) (dbus.Variant, *dbus.Error) {
	return dbus.MakeVariant(dbus.MakeVariant(p.scheme)), nil
}

func TestWatchDark(t *testing.T) {
	needBus(t)
	if err := desktop.Export(&newPortal{scheme: 1}, portalPath, settings); err != nil {
		t.Fatal(err)
	}

	got := make(chan bool, 8)
	New().WatchDark(func(dark bool) { got <- dark })
	expect := func(what string, want bool) {
		t.Helper()
		select {
		case dark := <-got:
			if dark != want {
				t.Fatalf("%s: dark %v, want %v", what, dark, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: nothing came", what)
		}
	}
	nothing := func(what string) {
		t.Helper()
		select {
		case dark := <-got:
			t.Fatalf("%s: dark %v came, want nothing", what, dark)
		case <-time.After(300 * time.Millisecond):
		}
	}
	change := func(ns, key string, scheme uint32) {
		t.Helper()
		if err := desktop.Emit(portalPath, settings+".SettingChanged", ns, key, dbus.MakeVariant(scheme)); err != nil {
			t.Fatal(err)
		}
	}

	expect("a dark desktop at start", true)
	change(appearance, colorScheme, 2)
	expect("turned light", false)
	change(appearance, colorScheme, 0)
	nothing("no preference, still light")
	change("org.gnome.desktop.interface", colorScheme, 1)
	nothing("another namespace's setting")
	change(appearance, "accent-color", 1)
	nothing("another key")
	change(appearance, colorScheme, 1)
	expect("turned dark", true)
}

// An older portal, under another name on the same bus.
func TestReadSchemeOnAnOlderPortal(t *testing.T) {
	needBus(t)
	older, err := dbus.Connect(bus)
	if err != nil {
		t.Fatal(err)
	}
	defer older.Close()
	older.Export(&oldPortal{scheme: 1}, portalPath, settings)
	if _, err := older.RequestName("org.example.OlderPortal", dbus.NameFlagDoNotQueue); err != nil {
		t.Fatal(err)
	}
	scheme, err := readScheme(desktop.Object("org.example.OlderPortal", portalPath))
	if err != nil || scheme != schemeDark {
		t.Fatalf("Read: %d, %v; want dark", scheme, err)
	}
}

func TestNoPortal(t *testing.T) {
	needBus(t)
	if _, err := readScheme(desktop.Object("org.example.NoPortal", portalPath)); err == nil {
		t.Fatal("a bus with no portal answered")
	}
}
