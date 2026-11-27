package streams

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"projectb/engine"
)

// Fetcher fetches an address into a file as a page would and says how it ended, once,
// unless the cancel it returns is called first: core/fetch's Manager. Main thread.
type Fetcher interface {
	Fetch(r engine.FetchRequest, done func(engine.Fetched)) (cancel func())
}

// Source is the stream to save: the kind of its playlist (HLS when none is said), the
// playlist's address, what the playlist holds when a download brought it (nil has the job
// fetch it), the page it is fetched as from and the workspace whose storage it is fetched
// in, and where the playlist's server is, which is where its pieces are asked from.
type Source struct {
	Kind      Kind
	URL       string
	Body      []byte
	Page      string
	Workspace string
	Space     engine.AddressSpace
}

// Config is a job's source, where it works and where the MP4 goes, and what it tells.
type Config struct {
	Source
	Fetcher Fetcher
	// Post runs f on the main thread, soon: engine.Post.
	Post func(f func())
	// Dir is a folder of the job's own, made and removed by it; Out is the MP4's path.
	Dir string
	Out string
	// Changed is told each time a piece is in; Done once, with nil when the MP4 is at Out,
	// or why it isn't: ErrLive, ErrProtected, ErrFormat, ErrRefused, ErrEmpty, a *FetchError,
	// or a file's error. Neither is called after Cancel. Main thread.
	Changed func(Progress)
	Done    func(err error)
}

// Progress is how far a job has got: the pieces in, of how many, and their bytes.
type Progress struct {
	Pieces int
	Total  int
	Bytes  int64
}

// together is how many pieces are fetched at a time, ahead how many may wait to be repacked
// before more are fetched, and tries how often one piece is asked for.
const (
	together = 4
	ahead    = 8
	tries    = 3
)

// Job saves one stream. Every method is for the main thread; the repacking runs on a
// goroutine of the job's own, one piece at a time, in the playlist's order.
type Job struct {
	c Config
	// renditions are the playlists of pieces, each asked from the space its playlist's
	// server is in; todo the pieces of each still to fetch, in order, and fetched the files
	// of those that are in and not yet handed to be repacked, which handed counts.
	renditions []Rendition
	from       []engine.AddressSpace
	todo       [][]int
	fetched    []map[int]string
	handed     []int
	attempts   map[string]int
	parts      map[*Part]*part
	// running are the fetches under way, by a number, each with what puts it back in line.
	running map[int]running
	last    int
	// waiting are the keys, header pieces and playlists to ask for again once the job goes on.
	waiting []func()
	paused  bool
	over    bool
	// progress, and how many pieces are repacked and whether the MP4 is being made.
	progress  Progress
	finishing bool
	// The goroutine's own: the working file and each rendition's MPEG-TS repacker.
	w     *work
	muxes map[int]*tsMux
	inits map[int]*Part
	tasks chan func()
	quit  chan struct{}
}

type running struct {
	cancel func()
	// again puts what was being fetched back to be fetched, when a pause stops it.
	again func()
}

// part is a key or a header piece, fetched once: what it holds, or that it is on its way.
type part struct {
	data     []byte
	have     bool
	fetching bool
}

// Start begins saving the stream.
func Start(c Config) *Job {
	j := &Job{c: c, attempts: map[string]int{}, parts: map[*Part]*part{}, running: map[int]running{},
		muxes: map[int]*tsMux{}, inits: map[int]*Part{}, quit: make(chan struct{})}
	if err := os.MkdirAll(c.Dir, 0o700); err != nil {
		c.Post(func() { j.fail(err) })
		return j
	}
	if c.Body != nil {
		// Not from inside Start: whoever started it has the job first.
		c.Post(func() {
			if !j.over {
				j.read(c.Body, c.URL, c.Space)
			}
		})
		return j
	}
	j.small(c.URL, "", c.Space, func(body []byte, got engine.Fetched) { j.read(body, got.URL, got.Space) })
	return j
}

// Progress is how far the job has got.
func (j *Job) Progress() Progress { return j.progress }

// Paused says whether the job is paused.
func (j *Job) Paused() bool { return j.paused }

// Pause stops fetching; what is in stays.
func (j *Job) Pause() {
	if j.over || j.paused {
		return
	}
	j.paused = true
	for id, r := range j.running {
		delete(j.running, id)
		r.cancel()
		r.again()
	}
}

// Resume goes on from where Pause stopped.
func (j *Job) Resume() {
	if j.over || !j.paused {
		return
	}
	j.paused = false
	j.pump()
}

// Cancel stops the job for good and removes what it made. Nothing is told after it.
func (j *Job) Cancel() { j.end() }

// end stops everything and has the goroutine remove the job's folder.
func (j *Job) end() {
	if j.over {
		return
	}
	j.over = true
	for id, r := range j.running {
		delete(j.running, id)
		r.cancel()
	}
	if j.tasks == nil {
		os.RemoveAll(j.c.Dir)
		return
	}
	close(j.quit)
}

func (j *Job) fail(err error) {
	if j.over {
		return
	}
	j.end()
	j.c.Done(err)
}

// fetch asks for one address into a file of the job's folder. again is what puts the
// request back in line when a pause stops it.
func (j *Job) fetch(address, span, name string, from engine.AddressSpace, again func(), done func(path string, got engine.Fetched)) {
	path := filepath.Join(j.c.Dir, name)
	j.last++
	id := j.last
	ended := false
	cancel := j.c.Fetcher.Fetch(engine.FetchRequest{Workspace: j.c.Workspace, Page: j.c.Page, URL: address, Path: path, Range: span, From: from},
		func(got engine.Fetched) {
			ended = true
			delete(j.running, id)
			if j.over {
				os.Remove(path)
				return
			}
			switch got.Outcome {
			case engine.FetchOK:
				done(path, got)
			case engine.FetchRefused:
				j.fail(ErrRefused)
			default:
				// A piece that fails is asked for again, a few times.
				j.attempts[address+" "+span]++
				if j.attempts[address+" "+span] < tries {
					again()
					j.pump()
					return
				}
				reason := "connection"
				if got.Outcome == engine.FetchServer {
					reason = "server"
				}
				j.fail(&FetchError{Reason: reason, URL: address})
			}
		})
	// One refused at once has ended before Fetch returned.
	if !ended && !j.over {
		j.running[id] = running{cancel: cancel, again: again}
	}
}

// small fetches a file the job reads whole: a playlist, a key or a header piece.
func (j *Job) small(address, span string, from engine.AddressSpace, then func(body []byte, got engine.Fetched)) {
	j.last++
	name := fmt.Sprintf("part-%d", j.last)
	// Asked again, it waits for the job to go on.
	again := func() { j.waiting = append(j.waiting, func() { j.small(address, span, from, then) }) }
	j.fetch(address, span, name, from, again, func(path string, got engine.Fetched) {
		body, err := os.ReadFile(path)
		os.Remove(path)
		if err != nil {
			j.fail(err)
			return
		}
		then(body, got)
	})
}

// read takes the stream's playlist: a media playlist's pieces, or a master playlist's best
// quality, whose own playlists are fetched next, each from where the master's server is.
func (j *Job) read(body []byte, base string, space engine.AddressSpace) {
	plan, err := Read(j.c.Kind, body, base)
	if err != nil {
		j.fail(err)
		return
	}
	if len(plan.Next) == 0 {
		from := make([]engine.AddressSpace, len(plan.Renditions))
		for i := range from {
			from[i] = space
		}
		j.index(plan.Renditions, from)
		return
	}
	renditions := make([]Rendition, len(plan.Next))
	from := make([]engine.AddressSpace, len(plan.Next))
	left := len(plan.Next)
	for i, next := range plan.Next {
		j.small(next, "", space, func(body []byte, got engine.Fetched) {
			p, err := ReadHLS(body, got.URL)
			if err == nil && len(p.Renditions) != 1 {
				err = ErrFormat
			}
			if err != nil {
				j.fail(err)
				return
			}
			renditions[i], from[i] = p.Renditions[0], got.Space
			if left--; left > 0 {
				return
			}
			// With sound of its own, the picture's playlist gives the picture alone.
			if len(renditions) > 1 {
				renditions[0].PictureOnly = true
				for i := 1; i < len(renditions); i++ {
					renditions[i].SoundOnly = true
				}
			}
			j.begin(renditions, from)
		})
		if j.over {
			return
		}
	}
}

// index reads the index of each file that keeps its own (DASH's SegmentBase), whose pieces
// take their place among the rendition's, and then starts on the pieces.
func (j *Job) index(renditions []Rendition, from []engine.AddressSpace) {
	left := 0
	for _, r := range renditions {
		left += len(r.Indexed)
	}
	if left == 0 {
		j.begin(renditions, from)
		return
	}
	found := make([][][]Piece, len(renditions))
	for r := range renditions {
		found[r] = make([][]Piece, len(renditions[r].Indexed))
		for i, x := range renditions[r].Indexed {
			j.small(x.URL, x.Range, from[r], func(body []byte, _ engine.Fetched) {
				pieces, err := indexed(x, body)
				if err != nil {
					j.fail(err)
					return
				}
				found[r][i] = pieces
				if left--; left > 0 {
					return
				}
				for r := range renditions {
					// From the last, so each one's place among the pieces still holds.
					for i := len(found[r]) - 1; i >= 0; i-- {
						at := renditions[r].Indexed[i].At
						renditions[r].Pieces = append(renditions[r].Pieces[:at:at], append(found[r][i], renditions[r].Pieces[at:]...)...)
					}
					renditions[r].Indexed = nil
				}
				j.begin(renditions, from)
			})
			if j.over {
				return
			}
		}
	}
}

// begin starts on the pieces.
func (j *Job) begin(renditions []Rendition, from []engine.AddressSpace) {
	w, err := newWork(j.c.Dir)
	if err != nil {
		j.fail(err)
		return
	}
	j.w, j.renditions, j.from = w, renditions, from
	for _, r := range renditions {
		todo := make([]int, len(r.Pieces))
		for i := range todo {
			todo[i] = i
		}
		j.todo = append(j.todo, todo)
		j.fetched = append(j.fetched, map[int]string{})
		j.handed = append(j.handed, 0)
		j.progress.Total += len(r.Pieces)
	}
	j.tasks = make(chan func(), j.progress.Total+2)
	go func() {
		for {
			select {
			case <-j.quit:
				j.w.close()
				os.RemoveAll(j.c.Dir)
				return
			case task := <-j.tasks:
				task()
			}
		}
	}()
	j.c.Changed(j.progress)
	j.pump()
}

// need says a piece's key and header piece are in, and fetches the ones that aren't.
func (j *Job) need(r int, p *Part) bool {
	if p == nil {
		return true
	}
	if p.Key != nil && !j.need(r, p.Key) {
		return false
	}
	have := j.parts[p]
	if have == nil {
		have = &part{}
		j.parts[p] = have
	}
	if have.have {
		return true
	}
	if !have.fetching {
		have.fetching = true
		j.small(p.URL, p.Range, j.from[r], func(body []byte, _ engine.Fetched) {
			if p.Key != nil {
				var err error
				if body, err = unlock(body, j.parts[p.Key].data, p.IV); err != nil {
					j.fail(err)
					return
				}
			}
			have.data, have.have = body, true
			j.pump()
		})
	}
	return false
}

// pump fetches more pieces while fewer than together are on their way: the next of the
// rendition that is least far on, so picture and sound come in side by side.
func (j *Job) pump() {
	if j.over || j.paused {
		return
	}
	waiting := j.waiting
	j.waiting = nil
	for _, again := range waiting {
		again()
	}
	for !j.over && !j.paused && len(j.running) < together {
		best := -1
		for r := range j.renditions {
			if len(j.todo[r]) == 0 || j.todo[r][0]-j.handed[r] >= ahead {
				continue
			}
			p := &j.renditions[r].Pieces[j.todo[r][0]]
			if !j.need(r, p.Key) || !j.need(r, p.Init) {
				if j.over {
					return
				}
				continue
			}
			if best < 0 || j.todo[r][0] < j.todo[best][0] {
				best = r
			}
		}
		if best < 0 {
			return
		}
		r, i := best, j.todo[best][0]
		j.todo[r] = j.todo[r][1:]
		p := j.renditions[r].Pieces[i]
		again := func() {
			// Back at the front of its rendition's line, in order.
			at := 0
			for at < len(j.todo[r]) && j.todo[r][at] < i {
				at++
			}
			j.todo[r] = append(j.todo[r][:at], append([]int{i}, j.todo[r][at:]...)...)
		}
		j.fetch(p.URL, p.Range, fmt.Sprintf("piece-%d-%d", r, i), j.from[r], again, func(path string, _ engine.Fetched) {
			j.fetched[r][i] = path
			j.hand()
			j.pump()
		})
	}
}

// hand gives the goroutine every piece that is in and next in its rendition's order.
func (j *Job) hand() {
	for r := range j.renditions {
		for {
			i := j.handed[r]
			path, ok := j.fetched[r][i]
			if !ok {
				break
			}
			delete(j.fetched[r], i)
			j.handed[r]++
			p := j.renditions[r].Pieces[i]
			var key, init []byte
			if p.Key != nil {
				key = j.parts[p.Key].data
			}
			if p.Init != nil {
				init = j.parts[p.Init].data
			}
			j.tasks <- func() {
				size, err := j.repack(r, p, path, key, init)
				os.Remove(path)
				j.c.Post(func() { j.repacked(size, err) })
			}
		}
	}
}

// repack adds one piece to the working file. On the job's goroutine.
func (j *Job) repack(r int, p Piece, path string, key, init []byte) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	size := int64(len(data))
	if key != nil {
		if data, err = unlock(data, key, p.IV); err != nil {
			return size, err
		}
	}
	if isTS(data) {
		m := j.muxes[r]
		if m == nil {
			m = &tsMux{w: j.w, rendition: r, r: j.renditions[r]}
			j.muxes[r] = m
		}
		return size, m.add(p.Run, data)
	}
	if !isFragment(data) {
		return size, ErrFormat
	}
	if init == nil {
		// A whole file of fragments: its header piece is its own start, up to the first
		// fragment.
		at := firstFragment(data)
		if at == 0 {
			return size, ErrFormat
		}
		if err := j.w.addInit(r, j.renditions[r], data[:at]); err != nil {
			return size, err
		}
		return size, j.w.append(r, p.Run, data[at:], false)
	}
	if j.inits[r] != p.Init {
		if err := j.w.addInit(r, j.renditions[r], init); err != nil {
			return size, err
		}
		j.inits[r] = p.Init
	}
	return size, j.w.append(r, p.Run, data, false)
}

// isFragment says a piece is MP4 boxes that hold a fragment: a moof somewhere at its top.
func isFragment(data []byte) bool {
	_, ok := fragmentAt(data)
	return ok
}

// firstFragment is where a file's first fragment starts, after its header: 0 when it has
// no header before it.
func firstFragment(data []byte) int {
	at, _ := fragmentAt(data)
	return at
}

// fragmentAt walks a piece's boxes to its first moof, with any styp, sidx, emsg or prft
// just before it.
func fragmentAt(data []byte) (int, bool) {
	at, start := 0, 0
	for len(data)-at >= 8 {
		size := int(uint32(data[at])<<24 | uint32(data[at+1])<<16 | uint32(data[at+2])<<8 | uint32(data[at+3]))
		kind := string(data[at+4 : at+8])
		if kind == "moof" {
			return start, true
		}
		if kind == "mdat" || size < 8 || size > len(data)-at {
			return 0, false
		}
		at += size
		// What belongs to the fragment that follows stays with it.
		if kind != "styp" && kind != "sidx" && kind != "emsg" && kind != "prft" {
			start = at
		}
	}
	return 0, false
}

// repacked is the goroutine's news of one piece.
func (j *Job) repacked(size int64, err error) {
	if j.over {
		return
	}
	if err != nil {
		j.fail(err)
		return
	}
	j.progress.Pieces++
	j.progress.Bytes += size
	j.c.Changed(j.progress)
	if j.progress.Pieces < j.progress.Total {
		j.pump()
		return
	}
	// Every piece is in: the MP4 is made.
	j.finishing = true
	j.tasks <- func() {
		var err error
		for r := range j.renditions {
			if m := j.muxes[r]; m != nil && err == nil {
				err = m.finish()
			}
		}
		if err == nil {
			err = j.w.finish(j.c.Out)
		}
		j.c.Post(func() {
			if j.over {
				if err == nil {
					os.Remove(j.c.Out)
				}
				return
			}
			j.end()
			j.c.Done(err)
		})
	}
}

// Reason is why a job failed, as a download's reason says it (downloads.Item.Reason): for a
// stream that can't be saved, "live", "protected", "format", "refused" (a part at an address
// that may not be fetched) or "empty"; for a fetch that failed, a download's own,
// "connection" or "server"; for a file that couldn't be written, "disk"; else "". again
// says trying again could go another way: not for a protected stream or one of a format the
// app can't repack.
func Reason(err error) (reason string, again bool) {
	var fetch *FetchError
	switch {
	case errors.As(err, &fetch):
		return fetch.Reason, true
	case errors.Is(err, ErrLive):
		return "live", true
	case errors.Is(err, ErrProtected):
		return "protected", false
	case errors.Is(err, ErrFormat):
		return "format", false
	case errors.Is(err, ErrRefused):
		return "refused", true
	case errors.Is(err, ErrEmpty):
		return "empty", true
	}
	var path *os.PathError
	if errors.As(err, &path) {
		return "disk", true
	}
	return "", true
}
