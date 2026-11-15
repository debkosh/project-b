package perf

import (
	"encoding/json"
	"fmt"
	"testing"

	"projectb/engine"
	"projectb/policy"
	"projectb/prefs"
)

var sizes = []int{1, 100, 1000, 5000}

// withTabs is a rig whose first window has n tabs, each navigated once.
func withTabs(b *testing.B, n int, o Options) *Rig {
	if o.FaviconBytes == 0 {
		o.FaviconBytes = 2000
	}
	o.Tabs = n
	r := New(b.TempDir(), o)
	r.Drain()
	return r
}

// report adds what one operation crossed and sent.
func report(b *testing.B, r *Rig, c0, bytes0, msgs0 int) {
	n := float64(b.N)
	b.ReportMetric(float64(r.E.Crossings()-c0)/n, "crossings/op")
	b.ReportMetric(float64(r.E.ShellBytes-bytes0)/n, "shellB/op")
	b.ReportMetric(float64(r.E.ShellMsgs-msgs0)/n, "shellMsgs/op")
}

func mark(r *Rig) (int, int, int) { return r.E.Crossings(), r.E.ShellBytes, r.E.ShellMsgs }

// One New Tab click and one close of that tab, in a window of n tabs.
func BenchmarkOpenClose(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			r := withTabs(b, n, Options{})
			c, by, m := mark(r)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				id := r.NewTab(1)
				r.Close(1, id)
			}
			b.StopTimer()
			report(b, r, c, by, m)
		})
	}
}

// A click on another tab, in a window of n tabs.
func BenchmarkActivate(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			r := withTabs(b, max(n, 2), Options{})
			all := r.A.Tabs.AllIn(1)
			c, by, m := mark(r)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Activate(1, all[i%len(all)].ID)
			}
			b.StopTimer()
			report(b, r, c, by, m)
		})
	}
}

// One navigation from the address bar: the command and its 13 engine events.
func BenchmarkNavigate(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			r := withTabs(b, n, Options{})
			c, by, m := mark(r)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Navigate(1, fmt.Sprintf("www.site%d.example.com/page%d", i%3000, i))
			}
			b.StopTimer()
			r.Drain()
			report(b, r, c, by, m)
		})
	}
}

// The snapshot a shell asks for as it loads, in a window of n tabs.
func BenchmarkSnapshot(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			r := withTabs(b, n, Options{})
			c, by, m := mark(r)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.A.ShellMessage(1, []byte(`{"t":"ready"}`))
			}
			b.StopTimer()
			report(b, r, c, by, m)
		})
	}
}

// Typing a 12-letter address, one suggest query per key, against h history entries.
func BenchmarkTypeSuggest(b *testing.B) {
	for _, h := range []int{100, 1000, 10000, 100000} {
		b.Run(fmt.Sprint(h), func(b *testing.B) {
			r := New(b.TempDir(), Options{History: h})
			c, by, m := mark(r)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Type(1, "site42.examp")
			}
			b.StopTimer()
			report(b, r, c, by, m)
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/12/1000, "µs/key")
		})
	}
}

// The policy gate for one URL against r blocked hosts, as every navigation runs it.
func BenchmarkPolicy(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			var hosts []string
			for i := 0; i < n; i++ {
				hosts = append(hosts, fmt.Sprintf("blocked%d.example.org", i))
			}
			p := policy.New(hosts)
			urls := []string{"https://www.news.example.com/world/2026/11/14/story.html?ref=home", "https://mail.blocked7.example.org/inbox", "http://localhost:8080/x"}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p.IsAllowedURL(urls[i%3])
			}
		})
	}
}

// One visit recorded: the map, and the copy handed to the writer, which writes on its own.
func BenchmarkHistoryAdd(b *testing.B) {
	r := New(b.TempDir(), Options{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.A.History.AddVisit(fmt.Sprintf("https://www.site%d.example.com/a", i%10000), "A title")
	}
}

// A pref changed from settings: the file written on the main thread and every listener told.
func BenchmarkPrefsSave(b *testing.B) {
	r := New(b.TempDir(), Options{})
	themes := []string{prefs.ThemeLight, prefs.ThemeDark}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.A.Prefs.SetTheme(themes[i%2])
	}
	// What the main thread pays; the writer finishes on its own, as the app's does on the
	// way out.
	b.StopTimer()
	r.A.Prefs.Close()
}

// What the snapshot and the tabs message cost in JSON alone: n tabs as the shell's tabs
// message carries them.
func BenchmarkTabsJSON(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			r := withTabs(b, n, Options{})
			m := map[string]any{"t": "tabs", "tabs": r.A.Tabs.AllIn(1), "active": r.A.Tabs.ActiveIn(1), "groups": r.A.Tabs.GroupsIn(1)}
			out, _ := json.Marshal(m)
			b.SetBytes(int64(len(out)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				json.Marshal(m)
			}
		})
	}
}

// The menu rebuilt and compared, which every tab change does.
func BenchmarkRefreshMenu(b *testing.B) {
	r := New(b.TempDir(), Options{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.A.Commands.RefreshMenu()
	}
}

// One tab state change that the shell must hear (a load progress step), alone.
func BenchmarkTabChanged(b *testing.B) {
	r := withTabs(b, 1, Options{})
	id := r.A.Tabs.ActiveIn(1)
	c, by, m := mark(r)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.A.LoadProgressChanged(id, float64(i%1000)/1000)
	}
	b.StopTimer()
	report(b, r, c, by, m)
}

var _ = engine.TabID(0)

// Go's own start: app.Start on a fresh profile, its first window's shell up, as PBStart
// and the shell's ready are, with the engine's work taken out.
func BenchmarkStart(b *testing.B) {
	Quiet()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dir := b.TempDir()
		b.StartTimer()
		r := New(dir, Options{})
		b.StopTimer()
		r.A.Stop()
		b.StartTimer()
	}
}

// Activate with pages whose favicon is a 23-byte stand-in: what the 2 KB data URLs in
// every tabs message cost.
func BenchmarkActivateSmallFavicon(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			r := withTabs(b, max(n, 2), Options{FaviconBytes: 23})
			all := r.A.Tabs.AllIn(1)
			c, by, m := mark(r)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Activate(1, all[i%len(all)].ID)
			}
			b.StopTimer()
			report(b, r, c, by, m)
		})
	}
}
