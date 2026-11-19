package mockengine

import (
	"net/url"
	"strings"
	"sync"

	"projectb/engine"
)

// A page's permission requests, as the mock page's buttons make them, and the app's
// answers, for tests to read.

// PermissionAnswer is the app's answer to one request: the kinds asked and each status.
type PermissionAnswer struct {
	ID       engine.PermissionID       `json:"id"`
	Kinds    []string                  `json:"kinds"`
	Statuses []engine.PermissionStatus `json:"statuses"`
}

type permissionLog struct {
	mu sync.Mutex
	// clicked is the notifications whose click the app fired in their page, by id.
	clicked []string
	last    engine.PermissionID
	kinds   map[engine.PermissionID][]string
	answers []PermissionAnswer
}

// requestPermission is a page in tab asking for kinds, from a press when gesture. Main
// thread.
func (e *Engine) requestPermission(tab engine.TabID, kinds string, gesture bool) {
	t := e.tabs[tab]
	if t == nil {
		return
	}
	origin := ""
	if u, err := url.Parse(t.current); err == nil && u.Host != "" {
		origin = u.Scheme + "://" + u.Host
	}
	e.perms.mu.Lock()
	if e.perms.kinds == nil {
		e.perms.kinds = map[engine.PermissionID][]string{}
	}
	e.perms.last++
	id := e.perms.last
	e.perms.kinds[id] = strings.Split(kinds, ",")
	e.perms.mu.Unlock()
	e.listener.PermissionRequested(engine.PermissionRequest{ID: id, Tab: tab, Origin: origin, Kinds: strings.Split(kinds, ","), Gesture: gesture})
}

func (e *Engine) AnswerPermission(id engine.PermissionID, statuses []engine.PermissionStatus) {
	e.perms.mu.Lock()
	defer e.perms.mu.Unlock()
	e.perms.answers = append(e.perms.answers, PermissionAnswer{ID: id, Kinds: e.perms.kinds[id], Statuses: statuses})
	delete(e.perms.kinds, id)
}

// PermissionAnswers is every answer the app gave, in order. Any goroutine.
func (e *Engine) PermissionAnswers() []PermissionAnswer {
	e.perms.mu.Lock()
	defer e.perms.mu.Unlock()
	return append([]PermissionAnswer{}, e.perms.answers...)
}

// showNotification is the page in tab showing a notification, id; the app says whether it
// may, and shows it on the system itself. Main thread.
func (e *Engine) showNotification(tab engine.TabID, id string) {
	t := e.tabs[tab]
	if t == nil {
		return
	}
	origin := ""
	if u, err := url.Parse(t.current); err == nil && u.Host != "" {
		origin = u.Scheme + "://" + u.Host
	}
	e.listener.NotificationShown(engine.Notification{ID: id, Origin: origin, Page: t.current, Title: id})
}

// ClickNotification records the page's click event, fired as the app asked.
func (e *Engine) ClickNotification(id string) {
	e.perms.mu.Lock()
	defer e.perms.mu.Unlock()
	e.perms.clicked = append(e.perms.clicked, id)
}

// NotificationsClicked is every notification whose click the app fired, in order. Any
// goroutine.
func (e *Engine) NotificationsClicked() []string {
	e.perms.mu.Lock()
	defer e.perms.mu.Unlock()
	return append([]string{}, e.perms.clicked...)
}
