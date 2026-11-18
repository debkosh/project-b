// Command stress runs the Go core on perf's rig at several tab counts, with a mixed
// workload of what people and pages do, and reports per-operation latency, throughput,
// heap, GC pauses and CPU share, goroutines and threads.
//
//	go run ./perf/cmd/stress -tabs 1,100,1000,5000 -ops 3000 -cpuprofile cpu.out
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"time"

	"projectb/engine"
	"projectb/perf"
)

var (
	tabsFlag   = flag.String("tabs", "1,100,1000,5000", "tab counts")
	windows    = flag.Int("windows", 4, "windows")
	ops        = flag.Int("ops", 3000, "operations per scale")
	history    = flag.Int("history", 20000, "history entries at start")
	cpuProfile = flag.String("cpuprofile", "", "CPU profile of the workload phase, at the last scale")
	memProfile = flag.String("memprofile", "", "allocation profile, at the last scale")
	flood      = flag.Int("flood", 20000, "background navigation events in the flood phase")
)

type sample struct {
	kind string
	d    time.Duration
}

func main() {
	flag.Parse()
	var scales []int
	for _, s := range strings.Split(*tabsFlag, ",") {
		n, _ := strconv.Atoi(s)
		scales = append(scales, n)
	}
	fmt.Printf("GOMAXPROCS=%d go=%s\n", runtime.GOMAXPROCS(0), runtime.Version())
	for i, n := range scales {
		run(n, i == len(scales)-1)
	}
}

func run(n int, last bool) {
	dir, _ := os.MkdirTemp("", "stress-")
	defer os.RemoveAll(dir)
	runtime.GC()
	before := read()
	t0 := time.Now()
	// The first window's tabs are made before its shell is up, the others' likewise, as a
	// restored session would be; then every shell loads and asks for its snapshot.
	per := max(n / *windows, 1)
	r := perf.New(dir, perf.Options{History: *history, FaviconBytes: 2000, Tabs: per})
	ws := []engine.WindowID{1}
	for w := 1; w < *windows && n > 1; w++ {
		id := r.NewWindowQuiet()
		ws = append(ws, id)
		r.Events(r.A.Tabs.ActiveIn(id), "https://www.first.example.com/")
		for i := 1; i < per; i++ {
			workspace := []string{"", "ws-a", "ws-b"}[i%3]
			r.Events(r.A.Tabs.Add(id, workspace, "", true), fmt.Sprintf("https://www.w%d-site%d.example.com/page", id, i))
		}
	}
	for _, w := range ws[1:] {
		r.WindowUp(w)
	}
	r.Drain()
	setup := time.Since(t0)
	total := 0
	for _, w := range ws {
		total += len(r.A.Tabs.AllIn(w))
	}

	rng := rand.New(rand.NewSource(1))
	if last && *cpuProfile != "" {
		f, _ := os.Create(*cpuProfile)
		pprof.StartCPUProfile(f)
		defer func() { pprof.StopCPUProfile(); f.Close() }()
	}
	mid := read()
	c0, b0, m0 := r.E.Crossings(), r.E.ShellBytes, r.E.ShellMsgs
	var samples []sample
	start := time.Now()
	for i := 0; i < *ops; i++ {
		w := ws[rng.Intn(len(ws))]
		in := r.A.Tabs.AllIn(w)
		p := rng.Intn(100)
		t := time.Now()
		var kind string
		switch {
		case p < 30:
			// A page in any tab of any window loads: its 13 events.
			kind = "bg-navigation"
			tab := in[rng.Intn(len(in))].ID
			r.Events(tab, fmt.Sprintf("https://www.bg%d.example.com/p/%d", rng.Intn(9999), i))
		case p < 45:
			kind = "navigate"
			r.Navigate(w, fmt.Sprintf("www.site%d.example.com/x/%d", rng.Intn(5000), i))
		case p < 60:
			kind = "activate"
			r.Activate(w, in[rng.Intn(len(in))].ID)
		case p < 70:
			kind = "new-tab"
			r.NewTab(w)
		case p < 80:
			kind = "close-tab"
			if len(in) > 1 {
				r.Close(w, in[rng.Intn(len(in))].ID)
			} else {
				r.NewTab(w)
			}
		case p < 90:
			kind = "type-key"
			r.A.ShellMessage(w, []byte(`{"t":"suggest","id":1,"text":"site`+strconv.Itoa(rng.Intn(99))+`"}`))
		case p < 95:
			kind = "window-focus"
			r.A.WindowFocused(w)
		default:
			kind = "snapshot"
			r.A.ShellMessage(w, []byte(`{"t":"ready"}`))
		}
		r.Drain()
		samples = append(samples, sample{kind, time.Since(t)})
	}
	elapsed := time.Since(start)
	after := read()

	// The flood: background loads only, as fast as they come, to find the event rate the
	// main thread can take.
	all := r.A.Tabs.AllIn(ws[0])
	fstart := time.Now()
	nav := *flood / 13
	for i := 0; i < nav; i++ {
		r.Events(all[i%len(all)].ID, "https://www.flood.example.com/"+strconv.Itoa(i))
	}
	fel := time.Since(fstart)

	fmt.Printf("\n== %d tabs requested, %d made, %d windows; setup %v\n", n, total, len(ws), setup.Round(time.Millisecond))
	fmt.Printf("workload: %d ops in %v = %.0f ops/s; %.1f crossings/op, %.0f shell bytes/op, %.1f shell msgs/op\n",
		*ops, elapsed.Round(time.Millisecond), float64(*ops)/elapsed.Seconds(),
		float64(r.E.Crossings()-c0)/float64(*ops), float64(r.E.ShellBytes-b0)/float64(*ops), float64(r.E.ShellMsgs-m0)/float64(*ops))
	kinds := map[string][]time.Duration{}
	for _, s := range samples {
		kinds[s.kind] = append(kinds[s.kind], s.d)
	}
	names := slices.Sorted(func(yield func(string) bool) {
		for k := range kinds {
			if !yield(k) {
				return
			}
		}
	})
	fmt.Printf("%-14s %6s %10s %10s %10s\n", "op", "n", "median", "p99", "max")
	for _, k := range names {
		d := kinds[k]
		slices.Sort(d)
		fmt.Printf("%-14s %6d %10v %10v %10v\n", k, len(d), q(d, 0.5), q(d, 0.99), d[len(d)-1])
	}
	fmt.Printf("flood: %d nav events in %v = %.0f events/s (%.1f µs/event)\n", nav*13, fel.Round(time.Millisecond),
		float64(nav*13)/fel.Seconds(), float64(fel.Microseconds())/float64(nav*13))
	gcCPU := after.gcCPU - mid.gcCPU
	allCPU := after.totalCPU - mid.totalCPU
	fmt.Printf("heap live %.1f MB (was %.1f before setup), heap goal %.1f MB, Go mapped %.1f MB, RSS %.1f MB\n",
		mb(after.live), mb(before.live), mb(after.goal), mb(after.mapped), rssMB())
	fmt.Printf("GC during workload: %d cycles in %v (%.1f/s); alloc %.1f MB (%.0f KB/op); GC CPU %.1f%% of Go CPU\n",
		after.cycles-mid.cycles, elapsed.Round(time.Millisecond), float64(after.cycles-mid.cycles)/elapsed.Seconds(),
		mb(after.alloc-mid.alloc), float64(after.alloc-mid.alloc)/1024/float64(*ops), 100*gcCPU/allCPU)
	fmt.Printf("CPU during workload: user (our goroutines) %.2fs, GC %.2fs (of it mark assist on our goroutines %.2fs), wall %.2fs\n",
		after.userCPU-mid.userCPU, gcCPU, after.assistCPU-mid.assistCPU, elapsed.Seconds())
	fmt.Printf("GC stop-the-world pauses (whole run): p50 %v, p99 %v, max bucket %v, n=%d\n", after.pauseP50, after.pauseP99, after.pauseMax, after.pauseN)
	fmt.Printf("goroutines %d, OS threads made %d, process threads now %d\n", runtime.NumGoroutine(), pprof.Lookup("threadcreate").Count(), threads())
	if last && *memProfile != "" {
		f, _ := os.Create(*memProfile)
		pprof.Lookup("allocs").WriteTo(f, 0)
		f.Close()
	}
}

func q(d []time.Duration, p float64) time.Duration {
	return d[min(len(d)-1, int(float64(len(d))*p))].Round(time.Microsecond)
}

func mb(b uint64) float64 { return float64(b) / (1 << 20) }

type stats struct {
	live, goal, mapped, alloc, cycles   uint64
	gcCPU, totalCPU, userCPU, assistCPU float64
	pauseP50, pauseP99, pauseMax        time.Duration
	pauseN                              uint64
}

func read() stats {
	s := []metrics.Sample{
		{Name: "/gc/heap/live:bytes"}, {Name: "/gc/heap/goal:bytes"}, {Name: "/memory/classes/total:bytes"},
		{Name: "/gc/heap/allocs:bytes"}, {Name: "/gc/cycles/total:gc-cycles"},
		{Name: "/cpu/classes/gc/total:cpu-seconds"}, {Name: "/cpu/classes/total:cpu-seconds"},
		{Name: "/sched/pauses/total/gc:seconds"},
		{Name: "/cpu/classes/user:cpu-seconds"}, {Name: "/cpu/classes/gc/mark/assist:cpu-seconds"},
	}
	metrics.Read(s)
	out := stats{live: s[0].Value.Uint64(), goal: s[1].Value.Uint64(), mapped: s[2].Value.Uint64(), alloc: s[3].Value.Uint64(),
		cycles: s[4].Value.Uint64(), gcCPU: s[5].Value.Float64(), totalCPU: s[6].Value.Float64(), userCPU: s[8].Value.Float64(), assistCPU: s[9].Value.Float64()}
	h := s[7].Value.Float64Histogram()
	var n uint64
	for _, c := range h.Counts {
		n += c
	}
	out.pauseN = n
	var acc uint64
	for i, c := range h.Counts {
		acc += c
		up := time.Duration(h.Buckets[i+1] * 1e9)
		if out.pauseP50 == 0 && acc*2 >= n && n > 0 {
			out.pauseP50 = up
		}
		if out.pauseP99 == 0 && acc*100 >= n*99 && n > 0 {
			out.pauseP99 = up
		}
		if c > 0 {
			out.pauseMax = up
		}
	}
	return out
}

func rssMB() float64 {
	b, _ := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	kb, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return float64(kb) / 1024
}

func threads() int {
	b, _ := exec.Command("ps", "-M", "-p", strconv.Itoa(os.Getpid())).Output()
	return strings.Count(string(b), "\n") - 1
}

var _ = filepath.Join
