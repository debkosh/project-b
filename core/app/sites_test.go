package app_test

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"projectb/engine"
	"projectb/mockengine"
)

// prompts waits for the shell to hear n open prompts, and returns them.
func (r *rig) prompts(n int) []any {
	r.t.Helper()
	m := r.waitFor(func(m map[string]any) bool {
		p, ok := m["permissionPrompts"].([]any)
		return m["t"] == "permissionPrompts" && ok && len(p) == n
	})
	return m["permissionPrompts"].([]any)
}

// answers waits for the engine to have n answers, and returns them.
func (r *rig) answers(n int) []mockengine.PermissionAnswer {
	r.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if a := r.e.PermissionAnswers(); len(a) >= n {
			return a
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.t.Fatalf("the engine has %d answers, want %d", len(r.e.PermissionAnswers()), n)
	return nil
}

func TestAPromptReachesTheShellAndItsAnswerTheEngine(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://meet.test/room")
	r.e.Fire(1, 1, "permission", "camera,microphone")
	p := r.prompts(1)[0].(map[string]any)
	if p["origin"] != "https://meet.test" || !reflect.DeepEqual(p["kinds"], []any{"camera", "microphone"}) || p["once"] != true {
		t.Fatalf("prompt %v", p)
	}
	r.shell(`{"t":"permissionAnswer","id":` + jsonNumber(p["id"]) + `,"choice":"allow"}`)
	r.prompts(0)
	if a := r.answers(1)[0]; !reflect.DeepEqual(a.Statuses, []engine.PermissionStatus{engine.PermissionGranted, engine.PermissionGranted}) {
		t.Fatalf("answer %+v", a)
	}
	// Decided, the next request is answered with no prompt.
	r.e.Fire(1, 1, "permission", "camera")
	if a := r.answers(2)[1]; !reflect.DeepEqual(a.Statuses, []engine.PermissionStatus{engine.PermissionGranted}) {
		t.Fatalf("answer %+v", a)
	}
	open := make(chan int)
	r.e.Post(func() { open <- len(r.a.Sites.Prompts()) })
	if n := <-open; n != 0 {
		t.Fatalf("%d prompts left open", n)
	}
}

func TestAPromptGoesWhenItsTabNavigates(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://meet.test/room")
	r.e.Fire(1, 1, "permission", "notifications")
	r.prompts(1)
	r.visit("https://elsewhere.test/")
	if a := r.answers(1)[0]; !reflect.DeepEqual(a.Statuses, []engine.PermissionStatus{engine.PermissionAsk}) {
		t.Fatalf("answer %+v", a)
	}
	open := make(chan int)
	r.e.Post(func() { open <- len(r.a.Sites.Prompts()) })
	if n := <-open; n != 0 {
		t.Fatalf("%d prompts left open", n)
	}
}

func jsonNumber(v any) string { return strconv.FormatFloat(v.(float64), 'f', -1, 64) }

func TestANotificationShowsOnceItsSiteMayAndItsClickBringsItsTabForward(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://chat.test/room")
	r.e.Fire(1, 1, "notification", "before")
	r.e.Fire(1, 1, "permission", "notifications")
	p := r.prompts(1)[0].(map[string]any)
	r.shell(`{"t":"permissionAnswer","id":` + jsonNumber(p["id"]) + `,"choice":"allow"}`)
	r.answers(1)
	r.e.Fire(1, 1, "notification", "after")
	r.e.Settle()
	if got := r.sys.Notifications(); !reflect.DeepEqual(got, []string{"after"}) {
		t.Fatalf("shown %v, want only the one after Allow", got)
	}
	// Another tab is active when the person clicks it: the page gets its click, and its tab
	// comes forward.
	r.shell(`{"t":"newTab"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] != float64(1) })
	r.sys.Click("after")
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == float64(1) })
	if got := r.e.NotificationsClicked(); !reflect.DeepEqual(got, []string{"after"}) {
		t.Fatalf("the page heard clicks on %v", got)
	}
	// A second click on it, from the system, is no one's: the first took it.
	r.sys.Click("after")
	r.e.Settle()
	if got := r.e.NotificationsClicked(); len(got) != 1 {
		t.Fatalf("the page heard clicks on %v", got)
	}
}

// A notification its page closes leaves the system, and a click that comes late does nothing.
func TestANotificationThePageClosesLeavesTheSystem(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://chat.test/room")
	r.e.Fire(1, 1, "permission", "notifications")
	p := r.prompts(1)[0].(map[string]any)
	r.shell(`{"t":"permissionAnswer","id":` + jsonNumber(p["id"]) + `,"choice":"allow"}`)
	r.answers(1)
	r.e.Fire(1, 1, "notification", "n1")
	r.e.Fire(1, 1, "notificationClose", "n1")
	r.e.Settle()
	if got := r.sys.Removed(); !reflect.DeepEqual(got, []string{"n1"}) {
		t.Fatalf("removed %v", got)
	}
	r.sys.Click("n1")
	r.e.Settle()
	if got := r.e.NotificationsClicked(); len(got) != 0 {
		t.Fatalf("the page heard clicks on %v", got)
	}
}
