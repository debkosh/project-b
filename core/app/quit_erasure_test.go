package app_test

import (
	"fmt"
	"testing"

	"projectb/engine"
	"projectb/workspaces"
)

// Clear cookies when the app quits: off, a quit erases nothing; on, the engine hears it
// erases every workspace's cookies and site storage but the sites kept, and hears again
// as a site is kept or let go and as a workspace is made.
func TestQuitErasesCookiesButTheSitesKept(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	told := func() (string, engine.Erasure) {
		var ids []string
		var e engine.Erasure
		r.main(func() { ids, e = r.e.QuitErasure() })
		return fmt.Sprint(ids), e
	}
	if ids, e := told(); ids != "[]" || e.Kinds != 0 {
		t.Fatalf("off, a quit erases %v in %s", e, ids)
	}
	r.openSettings()
	r.settings(`{"t":"setPref","name":"clearCookiesOnQuit","value":"true"}`)
	r.settings(`{"t":"keepCookies","site":"mail.test","keep":true}`)
	r.settings(`{"t":"keepCookies","site":"bank.test","keep":true}`)
	var work workspaces.Workspace
	r.main(func() { work, _ = r.a.Workspaces.Create("Work", "#5b7fd6") })
	ids, e := told()
	if ids != fmt.Sprint([]string{"", work.ID}) || e.Kinds != engine.EraseSiteData || fmt.Sprint(e.Keep) != "[bank.test mail.test]" || len(e.Sites) != 0 {
		t.Fatalf("on, a quit erases %+v in %s", e, ids)
	}
	r.settings(`{"t":"keepCookies","site":"mail.test","keep":false}`)
	r.settings(`{"t":"setPref","name":"askWhereToSave","value":"true"}`)
	if _, e := told(); fmt.Sprint(e.Keep) != "[bank.test]" {
		t.Fatalf("a site let go is still kept: %+v", e)
	}
	r.settings(`{"t":"setPref","name":"clearCookiesOnQuit","value":"false"}`)
	r.settings(`{"t":"setPref","name":"askWhereToSave","value":"false"}`)
	if _, e := told(); e.Kinds != 0 {
		t.Fatalf("off again, a quit erases %+v", e)
	}
}
