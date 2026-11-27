package app

import (
	"fmt"
	"testing"

	"projectb/downloads"
)

func TestDownloadNews(t *testing.T) {
	old := downloads.Item{ID: 1, Name: "old.zip", State: "done"}
	n := newDownloadNews([]downloads.Item{old})
	said := func(list []downloads.Item, offer bool) []string {
		var out []string
		for _, m := range n.changed(list, offer) {
			w := wordsOf(m)
			if ask, _ := m["ask"].(string); ask != "" {
				w += " / " + ask
			}
			out = append(out, m["state"].(string)+": "+w)
		}
		return out
	}
	big := downloads.Item{ID: 2, Name: "big.bin", State: "progress"}
	// The first start offers notifications, with Allow only; one already there is not told.
	got := said([]downloads.Item{big, old}, true)
	if len(got) != 1 || got[0] != "progress: Downloading big.bin / Show progress in notifications?" {
		t.Fatalf("a first start said %q", got)
	}
	if got := said([]downloads.Item{big, old}, true); len(got) != 0 {
		t.Fatalf("nothing new said %q", got)
	}
	small := downloads.Item{ID: 3, Name: "small.bin", State: "progress"}
	news := n.changed([]downloads.Item{small, big, old}, false)
	if len(news) != 1 || wordsOf(news[0]) != "Downloading small.bin" {
		t.Fatalf("a start once offered said %v", news)
	}
	// A running one has Pause and Cancel only: Retry is for one that failed.
	if got := fmt.Sprint(news[0]["actions"]); got != "[{pause Pause} {cancel Cancel}]" {
		t.Fatalf("a running download's bar has %s", got)
	}
	paused := downloads.Item{ID: 5, Name: "paused.bin", State: "progress", Paused: true}
	if news := n.changed([]downloads.Item{paused, small, big, old}, false); len(news) != 1 || fmt.Sprint(news[0]["actions"]) != "[{resume Resume} {cancel Cancel}]" {
		t.Fatalf("a paused download's bar has %v", news)
	}
	big.State, small.State, small.Reason = "done", "failed", "server"
	got = said([]downloads.Item{small, big, old}, false)
	if len(got) != 2 || got[0] != "done: big.bin downloaded" || got[1] != "failed: Couldn't download small.bin: the server refused it" {
		t.Fatalf("an end and a failure, oldest first, said %q", got)
	}
	// A stream that can't be saved says why as any failure does, and one that trying again
	// can't help has no Retry.
	film := downloads.Item{ID: 9, Name: "film.mp4", State: "progress", Stream: "hls"}
	said([]downloads.Item{film, small, big, old}, false)
	film.State, film.Reason = "failed", "live"
	if got := said([]downloads.Item{film, small, big, old}, false); len(got) != 1 || got[0] != "failed: Couldn't download film.mp4: live videos can't be saved yet" {
		t.Fatalf("a live stream said %q", got)
	}
	film.State = "progress"
	said([]downloads.Item{film, small, big, old}, false)
	film.State, film.Reason, film.NoRetry = "failed", "protected", true
	news = n.changed([]downloads.Item{film, small, big, old}, false)
	if len(news) != 1 || wordsOf(news[0]) != "Couldn't download film.mp4: this video is protected" || fmt.Sprint(news[0]["actions"]) != "[]" {
		t.Fatalf("a protected stream said %v", news)
	}
	report := downloads.Item{ID: 4, Name: "crash.log", From: "Crash report", State: "done"}
	if got := said([]downloads.Item{report, small, big, old}, false); len(got) != 1 || got[0] != "done: Crash report saved to Downloads" {
		t.Fatalf("a file the app kept said %q", got)
	}
	small.State = "progress"
	said([]downloads.Item{report, small, big, old}, false)
	small.State = "cancelled"
	if got := said([]downloads.Item{report, small, big, old}, false); len(got) != 1 || got[0] != "gone: " {
		t.Fatalf("a cancel said %q", got)
	}
}

func wordsOf(m map[string]any) string {
	w, _ := m["words"].(string)
	return w
}
