package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/shell"
	"projectb/timeline"
)

// downloadsShellPart gives every shell the profile's downloads, in its snapshot and on
// each change, and takes what a shell asks of one (docs/daily-driver.md).
func (a *App) downloadsShellPart() shell.Part {
	a.Downloads.OnChanged(func() { a.Shell.Send(map[string]any{"t": "downloads", "downloads": a.Downloads.List()}) })
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
