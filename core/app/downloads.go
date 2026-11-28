package app

import (
	"encoding/json"

	"projectb/downloads"
	"projectb/engine"
	"projectb/shell"
	"projectb/timeline"
)

// downloadsShellPart gives every shell the profile's downloads, in its snapshot and on
// each change, and takes what a shell asks of one (docs/daily-driver.md).
func (a *App) downloadsShellPart(e engine.Engine) shell.Part {
	a.Downloads.OnChanged(func() { a.Shell.Send(map[string]any{"t": "downloads", "downloads": a.Downloads.List()}) })
	news := newDownloadNews(a.Downloads.List())
	a.Downloads.OnChanged(func() {
		for _, n := range news.changed(a.Downloads.List(), !a.System.NotificationsAllowed() && !a.Downloads.Offered()) {
			a.Shell.Send(n)
		}
	})
	return shell.Part{
		Snapshot: func(into map[string]any) { into["downloads"] = a.Downloads.List() },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			switch t {
			case "downloadAction":
				var r struct {
					ID     int    `json:"id"`
					Action string `json:"action"`
				}
				json.Unmarshal(msg, &r)
				a.Downloads.Act(r.ID, r.Action)
			case "notificationsOffered":
				// The bar showed its offer: it never comes again, taken up or not.
				a.Downloads.SetOffered()
			case "allowNotifications":
				a.Downloads.SetOffered()
				a.System.AskNotifications(a.notificationsAnswered)
			case "clearDownloads":
				a.Downloads.Clear(0)
			case "saveReport":
				// A crash report, written now and kept as a finished download, as a phone's
				// share sheet may have nowhere to save it: in the list, with the system's
				// notice, and told to the window as a download that began there is.
				path, err := a.Crash.WriteReport()
				if err == nil {
					var id int
					if id, err = a.Downloads.Keep(path, "Crash report"); err == nil {
						a.Shell.SendTo(w, map[string]any{"t": "downloadStarted", "id": id})
					}
				}
				if err != nil {
					timeline.Errorf("crash report: %v", err)
				}
			default:
				return false
			}
			return true
		},
	}
}

// downloadNews is what a phone's download bar says (docs/shells.md, "A download you can
// see"), worked out here so that every phone shell says the same and only draws it: each
// download's start, end or failure, once, as it happens. A shell shows a start only while
// the app is showing, an end while it shows or the bar does, and nothing while a sheet or
// the Downloads page is open.
type downloadNews struct {
	// seen is each download's state as last told; the ones there at start are not told.
	seen map[int]string
}

func newDownloadNews(list []downloads.Item) *downloadNews {
	n := &downloadNews{seen: map[int]string{}}
	for _, d := range list {
		n.seen[d.ID] = d.State
	}
	return n
}

// whyFailed is why one failed, in the words the desktop's lists use (docs/daily-driver.md).
var whyFailed = map[string]string{
	"connection": "the connection stopped",
	"server":     "the server refused it",
	"disk":       "not enough disk space",
	// A streamed video that can't be saved (docs/video-streams.md).
	"live":      "live videos can't be saved yet",
	"protected": "this video is protected",
	"format":    "this video's format can't be saved yet",
	"refused":   "part of this video is at an address that can't be fetched",
	"empty":     "this video has nothing in it to save",
}

// changed is the news in a new list, oldest first, so the newest is said last. offer is
// whether a start offers to show downloads in the system's notifications, as the first
// download does while they aren't allowed: its bar has only Allow, and stays longer. News is
// short, 2 seconds, a question 8 (design-system.md, "On a phone").
func (n *downloadNews) changed(list []downloads.Item, offer bool) []map[string]any {
	var out []map[string]any
	for i := len(list) - 1; i >= 0; i-- {
		d := list[i]
		was, known := n.seen[d.ID]
		n.seen[d.ID] = d.State
		if known && was == d.State {
			continue
		}
		m := map[string]any{"t": "downloadNews", "id": d.ID, "state": d.State, "mark": "download"}
		type act struct {
			Action string `json:"action"`
			Label  string `json:"label"`
		}
		switch d.State {
		case "progress":
			if offer {
				// The question on a line of its own, so a long name never hides it.
				m["words"] = "Downloading " + d.Name
				m["ask"] = "Show progress in notifications?"
				m["actions"] = []act{{"allowNotifications", "Allow"}}
				m["offer"] = true
				m["lasts"] = 8000
				offer = false
			} else {
				// The name alone, whole: on a phone the buttons are icons, Pause and Cancel,
				// which say it is running, so "Downloading" only cut a long name short. The
				// offer above keeps it, having no Pause or Cancel to say so.
				m["words"] = d.Name
				// A running one has Pause (or Resume) and Cancel only; Retry is for one that
				// failed or was cancelled, and the engine's silence watch fails a stuck one.
				pause := act{"pause", "Pause"}
				if d.Paused {
					pause = act{"resume", "Resume"}
				}
				m["actions"] = []act{pause, {"cancel", "Cancel"}}
				m["lasts"] = 2000
			}
		case "done":
			// A file the app kept itself says what it is: "Crash report saved to Downloads".
			if d.From != "" {
				m["words"] = d.From + " saved to Downloads"
			} else {
				m["words"] = d.Name + " downloaded"
			}
			m["actions"] = []act{{"open", "Open"}}
			m["lasts"] = 2000
		case "failed":
			m["words"] = "Couldn't download " + d.Name
			if why := whyFailed[d.Reason]; why != "" {
				m["words"] = "Couldn't download " + d.Name + ": " + why
			}
			m["mark"] = "warn"
			m["actions"] = []act{{"retry", "Retry"}}
			if d.NoRetry {
				// Trying again can't help: a protected video, or a format the app can't repack.
				m["actions"] = []act{}
			}
			m["lasts"] = 2000
		default:
			// Cancelled: a bar saying it puts itself away.
			m["state"] = "gone"
		}
		out = append(out, m)
	}
	return out
}

// DownloadPublished takes a phone's copy of a finished download in its shared Downloads.
func (a *App) DownloadPublished(path, address string) { a.Downloads.Published(path, address) }

// DownloadNotificationCommand does what a download's notification on a phone asks, as the
// bar's and the Downloads page's buttons do.
func (a *App) DownloadNotificationCommand(id int, c engine.DownloadCommand) {
	switch c {
	case engine.DownloadCancel:
		a.Downloads.Act(id, "cancel")
	case engine.DownloadPause:
		a.Downloads.Act(id, "pause")
	case engine.DownloadResume:
		a.Downloads.Act(id, "resume")
	case engine.DownloadOpen:
		a.Downloads.Act(id, "open")
	}
}
