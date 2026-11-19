package app

import (
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
	if got := said([]downloads.Item{small, big, old}, false); len(got) != 1 || got[0] != "progress: Downloading small.bin" {
		t.Fatalf("a start once offered said %q", got)
	}
	big.State, small.State, small.Reason = "done", "failed", "server"
	got = said([]downloads.Item{small, big, old}, false)
	if len(got) != 2 || got[0] != "done: big.bin downloaded" || got[1] != "failed: Couldn't download small.bin: the server refused it" {
		t.Fatalf("an end and a failure, oldest first, said %q", got)
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
