package app

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"projectb/engine"
	"projectb/settings"
	"projectb/shell"
	"projectb/sync"
	"projectb/timeline"
	"projectb/words"
)

// isAppLink says whether u is one of the app's own links, project-b:.
func isAppLink(u string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(u)), "project-b:")
}

// OpenAppLink takes one of the app's own links, however it came: from another app, as the Mac
// and Android hand it over, or a page's link clicked. project-b://sync?address=<address>, with
// &token=<token> or without, opens window w's Settings at Sync, with the set-up Dialog filled in from it, and sends nothing to
// the server nor keeps anything until the person connects. Any other does nothing.
func (a *App) OpenAppLink(w engine.WindowID, link string) {
	if u, err := url.Parse(strings.TrimSpace(link)); err != nil || !strings.EqualFold(u.Scheme, "project-b") || u.Host != "sync" {
		timeline.Infof("a project-b: link that isn't sync's, ignored")
		return
	}
	a.Settings.OpenSection(w, "sync", map[string]any{"t": "syncLink", "link": link})
}

// syncSettingsPart is Settings' Sync (docs/sync.md): the status, which every shell has too,
// and what the page asks of sync, each answered with a message of its own. A deep link
// reaches the page as syncLink (OpenAppLink), and its address comes back as the page's
// syncPropose, which connects nothing.
func (a *App) syncSettingsPart() settings.Part {
	// The page has the address too, which Reconnect fills in; the shells don't.
	a.Sync.OnStatus(func(s sync.Status) {
		a.Settings.Send(map[string]any{"t": "sync", "sync": s, "address": a.Sync.Address()})
		a.Shell.Send(map[string]any{"t": "sync", "sync": s})
	})
	a.Shell.AddPart(shell.Part{
		Snapshot: func(into map[string]any) { into["sync"] = a.Sync.Status() },
		Handle:   func(engine.WindowID, string, []byte) bool { return false },
	})
	// answer sends t, with err's text when there is one, and its problem's name.
	answer := func(t string, err error, more map[string]any) {
		m := map[string]any{"t": t}
		for k, v := range more {
			m[k] = v
		}
		if err != nil {
			m["error"], m["problem"] = sync.Words(err), sync.Problem(err)
		}
		a.Settings.Send(m)
	}
	return settings.Part{
		Snapshot: func(into map[string]any) {
			into["sync"], into["syncAddress"], into["syncStart"] = a.Sync.Status(), a.Sync.Address(), sync.StartServer
		},
		Handle: func(tab engine.TabID, t string, msg []byte) bool {
			// The backup code's answers go to the page that asked alone: one of them is the code.
			asked := func(t string, err error, more map[string]any) {
				m := map[string]any{"t": t}
				for k, v := range more {
					m[k] = v
				}
				if err != nil {
					m["error"], m["problem"] = sync.Words(err), sync.Problem(err)
				}
				a.Settings.SendTo(tab, m)
			}
			var c struct {
				Address    string `json:"address"`
				Token      string `json:"token"`
				Passphrase string `json:"passphrase"`
				Link       string `json:"link"`
				// Code is a backup code: the one just made, to copy, or one typed, to check.
				Code string `json:"code"`
				// Again says the bucket is the one sync is on with, set up again after it was
				// reset: Go holds its address and token.
				Again bool `json:"again"`
			}
			json.Unmarshal(msg, &c)
			switch t {
			case "syncGetBucket":
				// The page where a person gets a bucket on the server to start with, in a tab
				// beside Settings. Its own "Open in Project B" comes back as a link.
				if t, ok := a.Tabs.Get(tab); ok {
					a.Tabs.OpenFrom(tab, sync.BucketPage, t.Workspace, true)
				}
			case "syncPropose":
				address, token, err := c.Address, "", error(nil)
				if c.Link != "" {
					address, token, err = sync.Link(c.Link)
				}
				var p sync.Proposal
				if err == nil {
					p, err = sync.Propose(address)
				}
				if err != nil {
					// With the address it was given, which a link's set-up shows under its error.
					answer("syncProposal", err, map[string]any{"address": address})
				} else {
					// A link's token fills the set-up's Token field, where the person sees it is there.
					answer("syncProposal", nil, map[string]any{"address": p.Address, "host": p.Host, "token": token})
				}
			case "syncInspect":
				inspected := func(isNew bool, err error) {
					if err != nil {
						answer("syncBucket", err, nil)
					} else {
						answer("syncBucket", nil, map[string]any{"new": isNew})
					}
				}
				if c.Again {
					a.Sync.InspectAgain(inspected)
				} else {
					a.Sync.Inspect(c.Address, c.Token, inspected)
				}
			case "syncConnect":
				connected := func(err error) {
					answer("syncConnected", err, map[string]any{"wrongPassphrase": errors.Is(err, sync.ErrPassphrase)})
				}
				if c.Again {
					a.Sync.ConnectAgain(c.Passphrase, connected)
				} else {
					a.Sync.Connect(c.Address, c.Token, c.Passphrase, connected)
				}
			case "syncChangePassphrase":
				a.Sync.ChangePassphrase(c.Passphrase, func(err error) { answer("syncPassphraseChanged", err, nil) })
			case "syncDisconnect":
				a.Sync.Disconnect(func(err error) { answer("syncDisconnected", err, nil) })
			case "syncNow":
				a.Sync.SyncNow()
			case "syncDetails":
				// For the Sync details page (docs/sync-details.md), which asks again after each run.
				a.Sync.Details(func(d sync.Details) { a.Settings.Send(map[string]any{"t": "syncDetails", "details": d}) })
			case "syncCancel":
				a.Sync.CancelRun()
			case "syncSuggest":
				answer("syncSuggested", nil, map[string]any{"passphrase": words.Passphrase(5)})
			case "syncStartFresh":
				a.Sync.StartFresh(func(err error) { answer("syncStartedFresh", err, nil) })
			case "syncBackupMake":
				// The code, to show once, and its QR, which the page draws only when asked.
				a.Sync.MakeBackupCode(func(code string, err error) {
					more := map[string]any{}
					if err == nil {
						more["code"], more["qr"] = code, qrRows(code)
					}
					asked("syncBackupMade", err, more)
				})
			case "syncBackupKeep":
				a.Sync.KeepBackupCode(func(err error) { asked("syncBackupSettled", err, map[string]any{"kept": true}) })
			case "syncBackupDrop":
				a.Sync.DropBackupCode(func(err error) { asked("syncBackupSettled", err, nil) })
			case "syncBackupCheck":
				a.Sync.CheckBackupCode(c.Code, func(is bool, err error) { asked("syncBackupChecked", err, map[string]any{"is": is}) })
			case "syncBackupCopy":
				// As a saved password is copied: marked as a secret, so what keeps the
				// clipboard's history leaves it out.
				a.System.CopySecret(c.Code)
				asked("syncBackupCopied", nil, nil)
			default:
				return false
			}
			return true
		},
	}
}
