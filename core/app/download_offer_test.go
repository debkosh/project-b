package app_test

import "testing"

// The first download's offer of notifications, by the messages a phone's download bar sends
// (core/app/downloads.go): notificationsOffered keeps that it was shown, so it never comes
// again; allowNotifications asks the system, which answers yes here, as a person who allows.
func TestTheDownloadBarsOffer(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var offered, allowed bool
	r.shell(`{"t":"notificationsOffered"}`)
	r.e.Settle()
	r.main(func() { offered = r.a.Downloads.Offered() })
	if !offered {
		t.Fatal("notificationsOffered didn't keep that the offer was shown")
	}
	r.shell(`{"t":"allowNotifications"}`)
	r.e.Settle()
	r.main(func() { allowed = r.sys.NotificationsAllowed() })
	if !allowed {
		t.Fatal("allowNotifications didn't ask the system")
	}
}
