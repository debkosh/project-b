//go:build !android

// Package portaltest is a desktop of a test's own: a private session bus, and on it a
// connection that owns the portals' name and serves the fakes a test exports. oskit's Linux
// tests use it from TestMain, since the code under test keeps one connection to the session
// bus for the process.
package portaltest

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/sudiptadeb/oskit/internal/portal"
)

// Desktop owns the portals' name on the tests' bus; Bus is its address. Nil where there is no
// dbus-daemon: Need skips the test then.
var (
	Desktop *dbus.Conn
	Bus     string
)

// Run starts the bus, runs the tests and stops it: os.Exit(portaltest.Run(m)).
func Run(m *testing.M) int {
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		fmt.Println("no dbus-daemon here: the portal's tests don't run")
		return m.Run()
	}
	cmd := exec.Command(daemon, "--session", "--nofork", "--print-address")
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		return 1
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	addr, err := bufio.NewReader(out).ReadString('\n')
	if err == nil {
		Bus = strings.TrimSpace(addr)
		os.Setenv("DBUS_SESSION_BUS_ADDRESS", Bus)
		Desktop, err = dbus.Connect(Bus)
	}
	if err == nil {
		_, err = Desktop.RequestName(portal.Name, dbus.NameFlagDoNotQueue)
	}
	if err != nil {
		fmt.Println(err)
		return 1
	}
	return m.Run()
}

// Need skips a test with no bus of its own.
func Need(t *testing.T) {
	t.Helper()
	if Desktop == nil {
		t.Skip("no session bus of the tests' own")
	}
}

// Respond answers a Request the fake was asked for, as the portal does: Response on the
// request's path, to the caller.
func Respond(sender dbus.Sender, token string, code uint32, results map[string]dbus.Variant) dbus.ObjectPath {
	path := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" +
		strings.ReplaceAll(strings.TrimPrefix(string(sender), ":"), ".", "_") + "/" + token)
	go Desktop.Emit(path, "org.freedesktop.portal.Request.Response", code, results)
	return path
}
