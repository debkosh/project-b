package app

import (
	"fmt"
	"testing"
	"time"

	"projectb/passkeys"
)

// A sign-in's card keeps one account's passkeys together, the account with the newest
// passkey first and each account's own the newest first. Only the rows of an account with
// several say when they were made, as Settings' Passkeys says it, and the time is there
// only when two of them were made on one day.
func TestOwnRowsOrderAndWhenMade(t *testing.T) {
	now := time.Date(2026, 11, 23, 21, 0, 0, 0, time.Local)
	at := func(year int, month time.Month, day, hour, min int) int64 {
		return time.Date(year, month, day, hour, min, 0, 0, time.Local).UnixMilli()
	}
	key := func(id, handle, name, display string, created int64) passkeys.Passkey {
		return passkeys.Passkey{ID: []byte(id), RPID: "example.com", UserHandle: []byte(handle), UserName: name, DisplayName: display, Created: created}
	}
	// As the store lists them: the newest first.
	listed := []passkeys.Passkey{
		key("a4", "asha", "asha@example.com", "Asha Rao", at(2026, 11, 23, 20, 15)),
		key("w1", "work", "work@example.com", "Work Rao", at(2026, 11, 23, 20, 14)),
		key("a3", "asha", "asha@example.com", "Asha Rao", at(2026, 11, 23, 9, 5)),
		key("n2", "none", "", "", at(2026, 11, 12, 8, 0)),
		key("a2", "asha", "asha@example.com", "Asha Rao", at(2026, 11, 3, 18, 30)),
		key("n1", "none", "", "", at(2026, 10, 2, 8, 0)),
		key("a1", "asha", "asha@example.com", "Asha Rao", at(2025, 1, 2, 7, 45)),
	}
	rows := ownOrder(listed)
	ids := ""
	for _, r := range rows {
		ids += string(r.ID) + " "
	}
	if ids != "a4 a3 a2 a1 w1 n2 n1 " {
		t.Fatalf("the order is %s", ids)
	}
	want := []passkeyAccount{
		{Title: "Asha Rao", Secondary: "asha@example.com · Made today, 20:15"},
		{Title: "Asha Rao", Secondary: "asha@example.com · Made today, 09:05"},
		{Title: "Asha Rao", Secondary: "asha@example.com · Made 3 Nov"},
		{Title: "Asha Rao", Secondary: "asha@example.com · Made 2 Jan 2025"},
		{Title: "Work Rao", Secondary: "work@example.com"},
		{Title: "No name", Secondary: "Made 12 Nov", Unnamed: true},
		{Title: "No name", Secondary: "Made 2 Oct", Unnamed: true},
	}
	if got := ownRows(rows, now); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("the rows read\n%+v, want\n%+v", got, want)
	}
}
