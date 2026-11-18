package app

import (
	"encoding/json"
	"errors"

	"projectb/engine"
	"projectb/settings"
	"projectb/shell"
	"projectb/sync"
	"projectb/words"
)

// syncSettingsPart is Settings' Sync (docs/sync.md): the status, which every shell has too,
// and what the page asks of sync, each answered with a message of its own. A deep link's
// address comes as the page's syncPropose, and connects nothing.
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
			m["error"], m["problem"] = err.Error(), sync.Problem(err)
		}
		a.Settings.Send(m)
	}
	return settings.Part{
		Snapshot: func(into map[string]any) { into["sync"], into["syncAddress"] = a.Sync.Status(), a.Sync.Address() },
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			var c struct {
				Address    string `json:"address"`
				Token      string `json:"token"`
				Passphrase string `json:"passphrase"`
				Link       string `json:"link"`
			}
			json.Unmarshal(msg, &c)
			switch t {
			case "syncPropose":
				address, err := c.Address, error(nil)
				if c.Link != "" {
					address, err = sync.Link(c.Link)
				}
				var p sync.Proposal
				if err == nil {
					p, err = sync.Propose(address)
				}
				if err != nil {
					answer("syncProposal", err, nil)
				} else {
					answer("syncProposal", nil, map[string]any{"address": p.Address, "host": p.Host, "needsToken": p.NeedsToken})
				}
			case "syncInspect":
				a.Sync.Inspect(c.Address, c.Token, func(isNew bool, err error) {
					if err != nil {
						answer("syncBucket", err, nil)
					} else {
						answer("syncBucket", nil, map[string]any{"new": isNew})
					}
				})
			case "syncConnect":
				a.Sync.Connect(c.Address, c.Token, c.Passphrase, func(err error) {
					answer("syncConnected", err, map[string]any{"wrongPassphrase": errors.Is(err, sync.ErrPassphrase)})
				})
			case "syncChangePassphrase":
				a.Sync.ChangePassphrase(c.Passphrase, func(err error) { answer("syncPassphraseChanged", err, nil) })
			case "syncDisconnect":
				a.Sync.Disconnect(func(err error) { answer("syncDisconnected", err, nil) })
			case "syncNow":
				a.Sync.SyncNow()
			case "syncCancel":
				a.Sync.CancelRun()
			case "syncSuggest":
				answer("syncSuggested", nil, map[string]any{"passphrase": words.Passphrase(5)})
			case "syncStartFresh":
				a.Sync.StartFresh(func(err error) { answer("syncStartedFresh", err, nil) })
			default:
				return false
			}
			return true
		},
	}
}
