package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"projectb/engine"
	"projectb/passwords"
	"projectb/settings"
	"projectb/system"
	"projectb/timeline"
)

// Settings' Passwords (docs/passwords.md): how many there are, on its row under Privacy, and
// its list page, a row per saved login with its site and username. Neither is sealed, so the
// list asks the person nothing. A password is: Show and Copy have the device check the
// person first, once for the page: the check holds for passwordCheckHolds while that page
// stays open, and is asked again after it, or once the page was closed or opened anew. Show
// hands the asking page the password; Copy puts it on the clipboard here, and the page never
// has it.

// passwordCheckHolds is how long the device's check holds for the Passwords page that asked
// it. Sudipta, 2026-11-20: "we should ask it once while on the screen and ask again either
// after sometime say 5mins or if that place is closed". Firefox's saved logins hold a check
// five minutes too; Chrome's settings hold it one.
var passwordCheckHolds = 5 * time.Minute

// passwordRow is one row of Settings' Passwords.
type passwordRow struct {
	ID string `json:"id"`
	// Site is the origin's host, with its port when it has one; Origin the whole of it.
	Site     string `json:"site"`
	Origin   string `json:"origin"`
	Username string `json:"username"`
	// Created is when it was saved, Unix milliseconds.
	Created int64 `json:"created"`
}

// checkForPassword is the device's check before a password is shown or copied, as the
// passkeys' is before one is used.
func checkForPassword(sys system.System, post func(func())) passwords.Verify {
	return func(ctx context.Context, origin string) error {
		site := siteOfOrigin(origin)
		got := make(chan system.Verdict, 1)
		var cancel func()
		post(func() {
			cancel = sys.VerifyPerson(system.PersonCheck{Reason: "show your password for " + site, Title: "Show your password", Subtitle: site}, func(v system.Verdict) { got <- v })
		})
		select {
		case v := <-got:
			switch v {
			case system.Verified:
				return nil
			case system.NoScreenLock:
				return passwords.ErrNoScreenLock
			}
			return passwords.ErrDenied
		case <-ctx.Done():
			// Posted after the asking, so cancel is set by now.
			post(func() { cancel() })
			return passwords.ErrCancelled
		}
	}
}

// siteOfOrigin is an origin's host, with its port when it has one.
func siteOfOrigin(origin string) string {
	if u, err := url.Parse(origin); err == nil && u.Host != "" {
		return u.Host
	}
	return origin
}

func (a *App) passwordSettingsPart(e engine.Engine) settings.Part {
	// The count is read off the main thread and kept: at start, and after each change.
	count := 0
	recount := func() {
		go func() {
			n, err := a.Passwords.Total()
			if err != nil {
				timeline.Errorf("passwords: %v", err)
				return
			}
			e.Post(func() {
				if n != count {
					count = n
					a.Settings.Send(map[string]any{"t": "passwordCount", "passwordCount": n})
				}
			})
		}()
	}
	// list sends the page every saved login.
	list := func() {
		go func() {
			all, err := a.Passwords.List()
			if err != nil {
				timeline.Errorf("passwords: %v", err)
			}
			rows := []passwordRow{}
			for _, l := range all {
				rows = append(rows, passwordRow{ID: l.ID, Site: siteOfOrigin(l.Origin), Origin: l.Origin, Username: l.Username, Created: l.Created})
			}
			e.Post(func() { a.Settings.Send(map[string]any{"t": "passwords", "passwords": rows}) })
		}()
	}
	a.Passwords.OnChanged(func() {
		recount()
		list()
	})
	recount()
	// The check that holds: for the page in one tab, until a time. Read and set on the main
	// thread alone. A tab's id is never another tab's, so a closed tab's hold opens nothing.
	var heldTab engine.TabID
	var heldUntil time.Time
	holds := func(tab engine.TabID) bool { return tab == heldTab && time.Now().Before(heldUntil) }
	release := func(tab engine.TabID) {
		if tab == heldTab {
			heldUntil = time.Time{}
		}
	}
	// opened is a password for the page in tab: with no check while one holds for it, and a
	// check that passed holding from then. done runs on the main thread.
	opened := func(tab engine.TabID, id string, done func(password string, err error)) {
		ctx := context.Background()
		held := holds(tab)
		if held {
			ctx = passwords.Checked(ctx)
		}
		go func() {
			password, err := a.Passwords.Password(ctx, id)
			e.Post(func() {
				if err == nil && !held {
					heldTab, heldUntil = tab, time.Now().Add(passwordCheckHolds)
				}
				done(password, err)
			})
		}()
	}
	// until is when the hold for tab ends, Unix milliseconds, for the page to hide what it
	// shows then; 0 with none.
	until := func(tab engine.TabID) int64 {
		if !holds(tab) {
			return 0
		}
		return heldUntil.UnixMilli()
	}
	return settings.Part{
		Snapshot: func(into map[string]any) { into["passwordCount"] = count },
		Handle: func(tab engine.TabID, t string, msg []byte) bool {
			var c struct {
				ID   string `json:"id"`
				What string `json:"what"`
				// Fresh is the page's first asking for the list, as it opens.
				Fresh bool `json:"fresh"`
			}
			// why is an error as the page words it.
			why := func(err error) string {
				switch {
				case errors.Is(err, passwords.ErrDenied), errors.Is(err, passwords.ErrCancelled):
					return "denied"
				case errors.Is(err, passwords.ErrNoScreenLock):
					return "noScreenLock"
				case errors.Is(err, passwords.ErrNotHere):
					return "notHere"
				case errors.Is(err, passwords.ErrNone):
					return "gone"
				}
				timeline.Errorf("passwords: %v", err)
				return "failed"
			}
			switch t {
			case "passwordsList":
				// A page that opens starts with no check held, whatever its tab held before.
				json.Unmarshal(msg, &c)
				if c.Fresh {
					release(tab)
				}
				list()
			case "passwordsClosed":
				// The page was left: its check holds no more.
				release(tab)
			case "passwordShow":
				// The answer goes to the page that asked alone.
				json.Unmarshal(msg, &c)
				opened(tab, c.ID, func(password string, err error) {
					answer := map[string]any{"t": "passwordShown", "id": c.ID, "heldUntil": until(tab)}
					if err != nil {
						answer["error"] = why(err)
					} else {
						answer["password"] = password
					}
					a.Settings.SendTo(tab, answer)
				})
			case "passwordCopy":
				// What is "password", after the device's check, or "username". Go puts it on the
				// clipboard, a password as a secret (system.CopySecret): it never goes to the
				// page for a copy.
				json.Unmarshal(msg, &c)
				answer := map[string]any{"t": "passwordCopied", "id": c.ID, "what": c.What}
				if c.What == "username" {
					go func() {
						l, err := a.Passwords.Get(c.ID)
						e.Post(func() {
							if err != nil {
								answer["error"] = why(err)
							} else {
								e.WriteClipboard(l.Username)
							}
							a.Settings.SendTo(tab, answer)
						})
					}()
					break
				}
				opened(tab, c.ID, func(password string, err error) {
					answer["heldUntil"] = until(tab)
					if err != nil {
						answer["error"] = why(err)
					} else {
						// A password goes on the clipboard marked as a secret, so what keeps or
						// shows the clipboard's history leaves it out.
						a.System.CopySecret(password)
					}
					a.Settings.SendTo(tab, answer)
				})
			case "passwordDelete":
				json.Unmarshal(msg, &c)
				go func() {
					if err := a.Passwords.Delete(c.ID); err != nil {
						timeline.Errorf("passwords: %v", err)
					}
				}()
			default:
				return false
			}
			return true
		},
	}
}
