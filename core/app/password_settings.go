package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"projectb/engine"
	"projectb/passwords"
	"projectb/settings"
	"projectb/system"
	"projectb/timeline"
)

// Settings' Passwords (docs/passwords.md): how many there are, on its row under Privacy, and
// its list page, a row per saved login with its site and username. Neither is sealed, so the
// list asks the person nothing. A password is: Show and Copy each have the device check the
// person first. Show hands the asking page the password; Copy puts it on the clipboard here,
// and the page never has it.

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
	return settings.Part{
		Snapshot: func(into map[string]any) { into["passwordCount"] = count },
		Handle: func(tab engine.TabID, t string, msg []byte) bool {
			var c struct {
				ID   string `json:"id"`
				What string `json:"what"`
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
				list()
			case "passwordShow":
				// The answer goes to the page that asked alone.
				json.Unmarshal(msg, &c)
				go func() {
					answer := map[string]any{"t": "passwordShown", "id": c.ID}
					if password, err := a.Passwords.Password(context.Background(), c.ID); err != nil {
						answer["error"] = why(err)
					} else {
						answer["password"] = password
					}
					e.Post(func() { a.Settings.SendTo(tab, answer) })
				}()
			case "passwordCopy":
				// What is "password", after the device's check, or "username". Go puts it on the
				// clipboard: the password never goes to the page for a copy.
				json.Unmarshal(msg, &c)
				go func() {
					answer := map[string]any{"t": "passwordCopied", "id": c.ID, "what": c.What}
					var text string
					var err error
					if c.What == "username" {
						var l passwords.Login
						l, err = a.Passwords.Get(c.ID)
						text = l.Username
					} else {
						text, err = a.Passwords.Password(context.Background(), c.ID)
					}
					e.Post(func() {
						if err != nil {
							answer["error"] = why(err)
						} else {
							e.WriteClipboard(text)
						}
						a.Settings.SendTo(tab, answer)
					})
				}()
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
