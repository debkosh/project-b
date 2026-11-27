package downloads

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"projectb/engine"
	"projectb/streams"
	"projectb/timeline"
)

// A streamed video saved as one MP4 (docs/video-streams.md). A download that is a stream's
// playlist isn't kept: once the engine has it, its row goes on as the stream, a job of
// core/streams that fetches the pieces and repacks them, under the playlist's name with
// .mp4. The row is a download like any other: progress, Pause, Resume, Cancel and Retry,
// a Retry of one whose pieces kept failing going on where it stopped, and on a phone its notification, which keeps the app running with no screen as a running
// download's does, and its copy in the shared Downloads once done.

// playlistLimit is the most a playlist may hold: a day of two-second pieces is a few MB.
const playlistLimit = 16 << 20

// sniff says whether the finished file at path starts as a stream's playlist does.
func sniff(path string) streams.Kind {
	f, err := os.Open(path)
	if err != nil {
		return streams.None
	}
	defer f.Close()
	head := make([]byte, streams.SniffLength)
	n, _ := io.ReadFull(f, head)
	return streams.Sniff(head[:n])
}

// playlist takes the engine's news of a download that is a stream's playlist: its row shows
// the MP4's name from the start, and once the playlist is here the stream's job takes over.
func (m *Manager) playlist(it *Item, d engine.Download, b began) {
	it.Stream, it.Workspace, it.Space = string(b.stream), b.workspace, int(engine.SpaceOf(d.Server))
	it.Name, it.Path, it.NoRetry, it.into = b.name, "", false, b.dir
	it.State, it.Received, it.Total, it.Paused, it.Reason = states[d.State], 0, -1, d.Paused, d.Reason
	if d.State == engine.DownloadProgress {
		m.show(it)
		m.changed()
		return
	}
	// The engine's part is over, however it ended: the playlist's file goes, and the row is
	// the stream's own from here.
	var body []byte
	if d.State == engine.DownloadDone {
		if info, err := os.Stat(d.Path); err == nil && info.Size() <= playlistLimit {
			body, _ = os.ReadFile(d.Path)
		}
	}
	os.Remove(d.Path)
	delete(m.byEngine, d.ID)
	if d.State != engine.DownloadCancelled {
		// Once this news of it is over: the engine is in the middle of telling it, and may
		// not take one of its downloads away meanwhile.
		id := d.ID
		m.e.Post(func() { m.e.DownloadCommand(id, engine.DownloadRemove) })
	}
	it.engine = 0
	if d.State == engine.DownloadDone && body != nil {
		m.startStream(it, body)
		return
	}
	if d.State == engine.DownloadDone {
		// A playlist too long to be one.
		it.State, it.Reason, it.NoRetry = "failed", "format", true
	}
	m.show(it)
	m.save()
	m.changed()
}

// show shows how a stream's row stands where the system shows downloads.
func (m *Manager) show(it *Item) {
	state := engine.DownloadProgress
	for s, name := range states {
		if name == it.State {
			state = s
		}
	}
	m.e.ShowDownload(it.ID, engine.Download{Name: it.Name, Path: it.Path, State: state, Received: it.Received, Total: it.Total, Paused: it.Paused})
}

// startStream starts saving the stream in its row: from the playlist in hand, or with none
// by fetching it again, as Retry does when the last job isn't held.
func (m *Manager) startStream(it *Item, body []byte) {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		timeline.Errorf("downloads: %v", err)
	}
	if it.Path == "" || it.State == "done" {
		dir := m.dir
		if it.into != "" {
			dir = it.into
		}
		it.Path = m.freeIn(dir, cleanName(it.Name))
		it.Name = filepath.Base(it.Path)
	}
	it.State, it.Paused, it.Reason, it.NoRetry, it.Received, it.Total = "progress", false, "", false, 0, -1
	// A folder of its own for each start: an earlier start's goes a moment after its end.
	m.jobs++
	it.job = streams.Start(streams.Config{
		Source:  streams.Source{Kind: streams.Kind(it.Stream), URL: it.URL, Body: body, Page: it.Referrer, Workspace: it.Workspace, Space: engine.AddressSpace(it.Space), Height: LinkedQuality(it.URL)},
		Fetcher: m.fetcher,
		Post:    m.e.Post,
		After:   m.after,
		Dir:     filepath.Join(m.streamsDir(), fmt.Sprintf("job-%d-%d", it.ID, m.jobs)),
		Out:     it.Path,
		Changed: func(p streams.Progress) {
			// Progress counts pieces: the bytes in, of what the whole would be at that rate.
			it.Received = p.Bytes
			if p.Pieces > 0 {
				it.Total = p.Bytes * int64(p.Total) / int64(p.Pieces)
			}
			m.show(it)
			m.changed()
		},
		Done: func(err error) {
			it.job, it.Paused = nil, false
			if err != nil {
				it.State = "failed"
				reason, again := streams.Reason(err)
				it.Reason, it.NoRetry = reason, !again
				timeline.Warnf("downloads: a stream: %v", err)
			} else {
				it.State = "done"
				if info, err := os.Stat(it.Path); err == nil {
					it.Received, it.Total = info.Size(), info.Size()
				}
				it.publishing = publishes
				m.e.PublishFile(it.Path, "video/mp4")
			}
			m.show(it)
			m.save()
			m.changed()
		},
		// Pieces that kept failing at the connection or a busy server: the row fails, and
		// the job holds what it has fetched and repacked for a Retry to go on from.
		Stopped: func(err error) {
			it.State, it.Paused = "failed", false
			it.Reason, _ = streams.Reason(err)
			timeline.Warnf("downloads: a stream stopped: %v", err)
			m.show(it)
			m.save()
			m.changed()
		},
	})
	m.show(it)
	m.changed()
}

// actStream does what a shell asks of a stream's row, and says whether it did: what is left
// is as any download's.
func (m *Manager) actStream(it *Item, action string) bool {
	switch action {
	case "cancel":
		if it.job == nil {
			return false
		}
		it.job.Cancel()
		it.job, it.State, it.Paused = nil, "cancelled", false
		m.show(it)
		m.save()
	case "pause", "resume":
		if it.job == nil || it.State != "progress" {
			return false
		}
		if it.Paused = action == "pause"; it.Paused {
			it.job.Pause()
		} else {
			it.job.Resume()
		}
		m.show(it)
	case "retry":
		// Not a running one, which has Pause and Cancel. One whose job is held, with its
		// folder, goes on where it stopped; any other starts over from its playlist's
		// address, in this row, under the same name.
		if it.URL == "" || m.fetcher == nil || it.NoRetry || it.State == "progress" {
			return true
		}
		if it.job != nil && it.job.Kept() {
			it.State, it.Reason = "progress", ""
			it.job.Resume()
			m.show(it)
			m.changed()
			return true
		}
		if it.job != nil {
			it.job.Cancel()
			it.job = nil
		}
		m.startStream(it, nil)
		return true
	case "remove":
		if it.job != nil {
			it.job.Cancel()
			it.job = nil
		}
		return false
	default:
		return false
	}
	m.changed()
	return true
}
