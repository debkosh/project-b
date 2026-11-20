package app_test

import (
	"testing"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/system"
)

// Settings' Passwords: the snapshot has how many there are; the list has each login's site
// and username and no password, and asks the person nothing; Show has the device check the
// person, and answers the asking page with the password, or with why not; Delete takes one
// away. The names: passwordsList, passwordShow, passwordCopy, passwordDelete.
func TestSettingsPasswords(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	if added, _, _ := r.a.Passwords.Import([]browserdata.Login{
		{Origin: "https://mail.example.org", Username: "asha@example.org", Password: "correct horse"},
		{Origin: "https://shop.example.com:8443", Username: "asha", Password: "tr0ub4dor"},
	}); added != 2 {
		t.Fatalf("brought %d", added)
	}
	got, snapshot := r.openSettings()
	if snapshot["passwordCount"] != float64(2) {
		// The count is read off the main thread: the snapshot may come first.
		r.waitOn(got, func(m map[string]any) bool { return m["t"] == "passwordCount" && m["passwordCount"] == float64(2) })
	}
	r.settings(`{"t":"passwordsList"}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "passwords" })
	rows := m["passwords"].([]any)
	if len(rows) != 2 {
		t.Fatalf("the list: %v", m)
	}
	first, second := rows[0].(map[string]any), rows[1].(map[string]any)
	if first["site"] != "mail.example.org" || first["username"] != "asha@example.org" || first["password"] != nil ||
		second["site"] != "shop.example.com:8443" || second["origin"] != "https://shop.example.com:8443" {
		t.Fatalf("the rows: %v", rows)
	}
	if n := len(r.sys.Checks()); n != 0 {
		t.Fatalf("listing had the device check the person %d times", n)
	}

	id := first["id"].(string)
	r.settings(`{"t":"passwordShow","id":"` + id + `"}`)
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "passwordShown" })
	if m["id"] != id || m["password"] != "correct horse" || m["error"] != nil {
		t.Fatalf("shown: %v", m)
	}
	if checks := r.sys.Checks(); len(checks) != 1 || checks[0].Subtitle != "mail.example.org" {
		t.Fatalf("the device's check: %+v", checks)
	}
	// Copy puts it on the clipboard here, after the check, and the page never has it; a
	// username is copied with no check.
	r.settings(`{"t":"passwordCopy","id":"` + id + `","what":"password"}`)
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "passwordCopied" })
	if m["what"] != "password" || m["password"] != nil || m["error"] != nil || r.e.Clipboard() != "correct horse" || len(r.sys.Checks()) != 2 {
		t.Fatalf("copied: %v, the clipboard %q, %d checks", m, r.e.Clipboard(), len(r.sys.Checks()))
	}
	r.settings(`{"t":"passwordCopy","id":"` + id + `","what":"username"}`)
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "passwordCopied" })
	if m["what"] != "username" || r.e.Clipboard() != "asha@example.org" || len(r.sys.Checks()) != 2 {
		t.Fatalf("the username copied: %v, the clipboard %q, %d checks", m, r.e.Clipboard(), len(r.sys.Checks()))
	}
	for verdict, why := range map[system.Verdict]string{system.Denied: "denied", system.NoScreenLock: "noScreenLock"} {
		r.sys.SetVerdict(verdict)
		r.settings(`{"t":"passwordShow","id":"` + id + `"}`)
		if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "passwordShown" }); m["error"] != why || m["password"] != nil {
			t.Fatalf("shown after %v: %v", verdict, m)
		}
	}
	r.sys.SetVerdict(system.Verified)
	r.settings(`{"t":"passwordShow","id":"none"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "passwordShown" }); m["error"] != "gone" {
		t.Fatalf("a login that isn't there: %v", m)
	}

	// The list and the count each come again, in either order.
	r.settings(`{"t":"passwordDelete","id":"` + id + `"}`)
	listed, counted := false, false
	r.waitOn(got, func(m map[string]any) bool {
		switch m["t"] {
		case "passwords":
			rows := m["passwords"].([]any)
			listed = len(rows) == 1 && rows[0].(map[string]any)["site"] == "shop.example.com:8443"
		case "passwordCount":
			counted = m["passwordCount"] == float64(1)
		}
		return listed && counted
	})
}
