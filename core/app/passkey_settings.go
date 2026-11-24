package app

import (
	"context"
	"encoding/json"
	"errors"

	"projectb/engine"
	"projectb/keys"
	"projectb/passkeys"
	"projectb/settings"
	"projectb/timeline"
)

// Settings' Passkeys (docs/passkeys.md, Seeing and deleting them): how many there are, on
// its row under Privacy, from the plain columns; and its list page, a row per passkey with
// its site, its account and when it was made, each with Delete. The accounts are sealed,
// so the list is a person's call, which a locked profile answers by asking to unlock, and
// Delete sits behind the same list.

// passkeyRow is one row of Settings' Passkeys.
type passkeyRow struct {
	ID        string `json:"id"`
	Site      string `json:"site"`
	Title     string `json:"title"`
	Secondary string `json:"secondary,omitempty"`
	Unnamed   bool   `json:"unnamed,omitempty"`
	// Created is when it was made, Unix milliseconds.
	Created int64 `json:"created"`
}

func (a *App) passkeySettingsPart(e engine.Engine) settings.Part {
	// The count is read off the main thread and kept: at start, and after each change here
	// or from sync.
	count := 0
	var count_ func()
	count_ = func() {
		go func() {
			n, err := a.Passkeys.Total()
			if err != nil {
				timeline.Errorf("passkeys: %v", err)
				return
			}
			e.Post(func() {
				if n != count {
					count = n
					a.Settings.Send(map[string]any{"t": "passkeyCount", "passkeyCount": n})
				}
			})
		}()
	}
	a.passkeysChanged = count_
	a.Passkeys.OnChanged(count_)
	count_()
	// list sends the page every passkey, as a person asked.
	list := func() {
		go func() {
			all, err := a.Passkeys.All(context.Background(), true)
			// A profile whose key doesn't open here has passkeys it can't show.
			notHere := errors.Is(err, passkeys.ErrNotHere)
			// A desktop keyring that gave no key: the page says so, and offers to try again.
			keyring := errors.Is(err, keys.ErrKeyring)
			if err != nil && !notHere && !keyring {
				timeline.Errorf("passkeys: %v", err)
			}
			rows := []passkeyRow{}
			for _, p := range all {
				account := accountRow(p.UserName, p.DisplayName)
				rows = append(rows, passkeyRow{ID: b64(p.ID), Site: p.RPID, Title: account.Title, Secondary: account.Secondary, Unnamed: account.Unnamed, Created: p.Created})
			}
			e.Post(func() {
				a.Settings.Send(map[string]any{"t": "passkeys", "passkeys": rows, "notHere": notHere, "keyring": keyring})
			})
		}()
	}
	return settings.Part{
		Snapshot: func(into map[string]any) { into["passkeyCount"] = count },
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			switch t {
			case "passkeysList":
				list()
			case "passkeyDelete":
				var c struct {
					ID string `json:"id"`
				}
				json.Unmarshal(msg, &c)
				go func() {
					if err := a.Passkeys.Delete(context.Background(), true, unb64(c.ID)); err != nil {
						timeline.Errorf("passkeys: %v", err)
					}
					list()
				}()
			default:
				return false
			}
			return true
		},
	}
}
