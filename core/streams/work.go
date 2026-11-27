package streams

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/Eyevinn/mp4ff/mp4"
)

// work is a stream being put together: one file of MP4 fragments, each piece added to its
// end as it comes, the tracks they belong to, and at the end the MP4 made of them. A piece
// that is already a fragment of MP4 is added as it is; an MPEG-TS piece is repacked into
// fragments first (ts.go). Pieces of the picture and of separate sound arrive in turn, so
// the two lie side by side in the file, and in the MP4.
//
// Each piece keeps the clock its stream gave it. Only finish sets the times: every run of
// pieces between two discontinuities is moved, all its tracks by the same amount, to start
// where the run before ended, and the first to start at nothing. mp4ff's Defragment then
// writes the regular MP4: the index first, the samples copied as they are.
type work struct {
	dir   string
	frags *os.File
	size  int64
	// tracks are every track the pieces have, in the order they showed up: a track's number
	// in the MP4 is its place here, from 1.
	tracks []*track
	// spans are the pieces added, in the file's order.
	spans []span
}

// track is one track of the stream.
type track struct {
	rendition int
	// source is its number in its rendition's own fragments; one whose fragments are
	// written here (MPEG-TS) has none, and written is its number in those.
	source  uint32
	written uint32
	trak    *mp4.TrakBox
	trex    *mp4.TrexBox
	// keep says the track goes into the MP4: the picture, and the sound that is used.
	keep bool
	// skip is how much earlier than its stamps the track's stream says a sample is shown, in
	// the track's units: its header's edits, the media time showing starts from, which takes
	// off a picture's reordering delay, less any wait before it.
	skip int64
}

// span is one piece's place in the working file, which rendition and run it is of, and
// whether its fragments were written here.
type span struct {
	start, end int64
	rendition  int
	run        int
	written    bool
}

func newWork(dir string) (*work, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "fragments"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	return &work{dir: dir, frags: f}, nil
}

func (w *work) close() {
	if w.frags != nil {
		w.frags.Close()
		w.frags = nil
	}
}

// append adds bytes to the working file as a piece of rendition's run: fragments as the
// stream had them, or written here.
func (w *work) append(rendition, run int, data []byte, written bool) error {
	if _, err := w.frags.WriteAt(data, w.size); err != nil {
		return err
	}
	w.spans = append(w.spans, span{start: w.size, end: w.size + int64(len(data)), rendition: rendition, run: run, written: written})
	w.size += int64(len(data))
	return nil
}

// addInit takes the header piece of a rendition of MP4 fragments: its tracks become the
// stream's, the picture and the sound that r keeps. A later header of the same rendition
// must describe the same tracks. A header that says its tracks are encrypted is
// ErrProtected.
func (w *work) addInit(rendition int, r Rendition, data []byte) error {
	f, err := mp4.DecodeFile(bytes.NewReader(data))
	if err != nil || f.Moov == nil || len(f.Moov.Traks) == 0 {
		return ErrFormat
	}
	if len(f.Moov.Psshs) > 0 {
		return ErrProtected
	}
	for _, trak := range f.Moov.Traks {
		if trak.Tkhd == nil || trak.Mdia == nil || trak.Mdia.Hdlr == nil || trak.Mdia.Mdhd == nil || trak.Mdia.Minf == nil ||
			trak.Mdia.Minf.Stbl == nil || trak.Mdia.Minf.Stbl.Stsd == nil || trak.Mdia.Mdhd.Timescale == 0 {
			return ErrFormat
		}
		id := trak.Tkhd.TrackID
		if f.Moov.IsEncrypted(id) {
			return ErrProtected
		}
		kind := trak.Mdia.Hdlr.HandlerType
		keep := kind == "vide" && !r.SoundOnly || kind == "soun" && !r.PictureOnly
		if have := w.find(rendition, id); have != nil {
			if keep && !sameEntries(have.trak, trak) {
				return ErrFormat
			}
			continue
		}
		t := &track{rendition: rendition, source: id, trak: trak, keep: keep}
		if trak.Edts != nil && len(trak.Edts.Elst) > 0 && f.Moov.Mvhd != nil && f.Moov.Mvhd.Timescale > 0 {
			var wait float64
			for _, e := range trak.Edts.Elst[0].Entries {
				if e.MediaTime < 0 {
					wait += float64(e.SegmentDuration) / float64(f.Moov.Mvhd.Timescale)
					continue
				}
				t.skip = e.MediaTime - int64(math.Round(wait*float64(trak.Mdia.Mdhd.Timescale)))
				break
			}
		}
		if f.Moov.Mvex != nil {
			for _, trex := range f.Moov.Mvex.Trexs {
				if trex.TrackID == id {
					t.trex = trex
				}
			}
		}
		w.tracks = append(w.tracks, t)
	}
	return nil
}

func (w *work) find(rendition int, source uint32) *track {
	for _, t := range w.tracks {
		if t.rendition == rendition && t.source == source {
			return t
		}
	}
	return nil
}

// sameEntries says two tracks describe their samples alike.
func sameEntries(a, b *mp4.TrakBox) bool {
	var x, y bytes.Buffer
	if a.Mdia.Minf.Stbl.Stsd.Encode(&x) != nil || b.Mdia.Minf.Stbl.Stsd.Encode(&y) != nil {
		return false
	}
	return bytes.Equal(x.Bytes(), y.Bytes()) && a.Mdia.Mdhd.Timescale == b.Mdia.Mdhd.Timescale
}

// newTrack adds a track whose fragments this package writes itself, and returns it with its
// number in those fragments.
func (w *work) newTrack(rendition int, timescale uint32, kind string) (*track, uint32) {
	id := uint32(len(w.tracks) + 1)
	t := &track{rendition: rendition, written: id, trak: mp4.CreateEmptyTrak(id, timescale, kind, "und"), keep: true}
	w.tracks = append(w.tracks, t)
	return t, id
}

// stretch is one track's samples in one run: the fragments that hold them, when the first is
// decoded and shown and when the last ends, on the stream's own clock, in the track's units.
type stretch struct {
	trafs       []*mp4.TrafBox
	first, show int64
	end         int64
}

// finish makes the MP4 at out from the working file.
func (w *work) finish(out string) error {
	// The header: every track, the picture's rendition before the sound's, numbered by its
	// place.
	tracks := append([]*track{}, w.tracks...)
	sort.SliceStable(tracks, func(i, j int) bool { return tracks[i].rendition < tracks[j].rendition })
	head := mp4.CreateEmptyInit()
	// The movie's own clock, in which a track's start after the first is written: fine
	// enough that sound and picture stay together to a hair.
	head.Moov.Mvhd.Timescale = tsClock
	kept := []uint32{}
	for i, t := range tracks {
		id := uint32(i + 1)
		t.trak.Tkhd.TrackID = id
		// Any edit of the source's goes: the tracks' places are set below.
		if t.trak.Edts != nil {
			children := []mp4.Box{}
			for _, c := range t.trak.Children {
				if c != mp4.Box(t.trak.Edts) {
					children = append(children, c)
				}
			}
			t.trak.Children, t.trak.Edts = children, nil
		}
		head.Moov.AddChild(t.trak)
		trex := mp4.CreateTrex(id)
		if t.trex != nil {
			*trex = *t.trex
			trex.TrackID = id
		}
		t.trex = trex
		head.Moov.Mvex.AddChild(trex)
		if t.keep {
			kept = append(kept, id)
		}
	}
	if len(kept) == 0 {
		return ErrEmpty
	}
	head.Moov.Mvhd.NextTrackID = uint32(len(tracks) + 1)
	var init bytes.Buffer
	if err := head.Encode(&init); err != nil {
		return err
	}
	whole := &joined{head: init.Bytes(), rest: w.frags, size: int64(init.Len()) + w.size}
	f, err := mp4.DecodeFile(whole, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil || f.Moov == nil || len(f.Moov.Traks) != len(tracks) {
		return fmt.Errorf("%w: %v", ErrFormat, err)
	}
	// The tracks as read back are the ones the MP4 is made of.
	for i, t := range tracks {
		t.trak = f.Moov.Traks[i]
	}
	// Each fragment's track, by the piece it lies in, and each track's stretches by run.
	runs := 0
	stretches := map[*track]map[int]*stretch{}
	for _, seg := range f.Segments {
		for _, frag := range seg.Fragments {
			if frag.Moof == nil {
				continue
			}
			at := int64(frag.Moof.StartPos) - int64(init.Len())
			i := sort.Search(len(w.spans), func(i int) bool { return w.spans[i].end > at })
			if i == len(w.spans) || w.spans[i].start > at {
				return ErrFormat
			}
			sp := w.spans[i]
			runs = max(runs, sp.run+1)
			for _, traf := range frag.Moof.Trafs {
				if traf.Tfhd == nil {
					return ErrFormat
				}
				var t *track
				for _, x := range tracks {
					if x.rendition == sp.rendition && (sp.written && x.written == traf.Tfhd.TrackID || !sp.written && x.written == 0 && x.source == traf.Tfhd.TrackID) {
						t = x
					}
				}
				if t == nil {
					return ErrFormat
				}
				traf.Tfhd.TrackID = t.trak.Tkhd.TrackID
				if !t.keep {
					continue
				}
				if traf.Tfdt == nil {
					return ErrFormat
				}
				first := int64(traf.Tfdt.BaseMediaDecodeTime())
				show, end := first, first
				shown := false
				for _, trun := range traf.Truns {
					for _, s := range trun.Samples {
						dur, cto := int64(s.Dur), int64(s.CompositionTimeOffset)
						if !trun.HasSampleDuration() {
							dur = int64(t.trex.DefaultSampleDuration)
							if traf.Tfhd.HasDefaultSampleDuration() {
								dur = int64(traf.Tfhd.DefaultSampleDuration)
							}
						}
						if !trun.HasSampleCompositionTimeOffset() {
							cto = 0
						}
						if !shown || end+cto < show {
							show, shown = end+cto, true
						}
						end += dur
					}
				}
				if !shown {
					continue
				}
				if stretches[t] == nil {
					stretches[t] = map[int]*stretch{}
				}
				st := stretches[t][sp.run]
				if st == nil {
					st = &stretch{first: first, show: show}
					stretches[t][sp.run] = st
				}
				st.trafs = append(st.trafs, traf)
				st.end = max(st.end, end)
			}
		}
	}
	if len(stretches) == 0 {
		return ErrEmpty
	}
	// A track with nothing in it stays out.
	kept = kept[:0]
	for i, t := range tracks {
		if t.keep && stretches[t] != nil {
			kept = append(kept, uint32(i+1))
		}
	}
	// The times. Each run starts, in seconds of the MP4, where the run before ended, and no
	// earlier than any of its tracks can be decoded from. A sample is shown skip earlier than
	// its stamps say. place is how long after a track's first sample is decoded it is shown,
	// from its first run, which its samples keep in every run; done is where each track's
	// samples end so far, in its own units, and from where the run starts, in seconds.
	place := map[*track]int64{}
	done := map[*track]int64{}
	var from float64
	for run := range runs {
		start := math.Inf(1) // the run's first moment shown, on the stream's own clock
		for t, by := range stretches {
			if st := by[run]; st != nil {
				start = min(start, float64(st.show-t.skip)/scale(t))
			}
		}
		if math.IsInf(start, 1) {
			continue
		}
		for t, by := range stretches {
			st := by[run]
			if st == nil {
				continue
			}
			if _, ok := place[t]; !ok {
				place[t] = st.show - t.skip - st.first
			}
			// Decoded no earlier than the track's samples so far end.
			if need := float64(done[t]-place[t]-st.first)/scale(t) + start; need > from {
				from = need
			}
		}
		var ends float64
		for t, by := range stretches {
			st := by[run]
			if st == nil {
				continue
			}
			shift := int64(math.Round((from-start)*scale(t))) + place[t]
			if st.first+shift < done[t] {
				shift = done[t] - st.first
			}
			for _, traf := range st.trafs {
				traf.Tfdt.SetBaseMediaDecodeTime(uint64(int64(traf.Tfdt.BaseMediaDecodeTime()) + shift))
			}
			if _, placed := done[t]; !placed {
				// The track's one edit: shown from its first sample's own moment on. Defragment
				// counts it from the track's first decode time, and delays the track by how
				// much later than the earliest that is.
				elst := &mp4.ElstBox{Entries: []mp4.ElstEntry{{MediaTime: st.show + shift, MediaRateInteger: 1}}}
				if elst.Entries[0].MediaTime > math.MaxInt32 {
					elst.Version = 1
				}
				edts := &mp4.EdtsBox{Elst: []*mp4.ElstBox{elst}, Children: []mp4.Box{elst}}
				t.trak.Edts = edts
				// After the track's header, where players look for it.
				children := append([]mp4.Box{}, t.trak.Children[:1]...)
				t.trak.Children = append(append(children, edts), t.trak.Children[1:]...)
			}
			done[t] = st.end + shift
			ends = max(ends, float64(done[t]-place[t])/scale(t))
		}
		from = ends
	}
	partial := out + ".crdownload"
	file, err := os.OpenFile(partial, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	err = mp4.DefragmentTracks(f, whole, file, kept...)
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(partial, out)
	}
	if err != nil {
		os.Remove(partial)
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return err
		}
		return fmt.Errorf("%w: %v", ErrFormat, err)
	}
	return nil
}

func scale(t *track) float64 { return float64(t.trak.Mdia.Mdhd.Timescale) }

// joined reads the header and then the working file as one file.
type joined struct {
	head []byte
	rest io.ReaderAt
	size int64
	at   int64
}

func (j *joined) Read(p []byte) (int, error) {
	if j.at >= j.size {
		return 0, io.EOF
	}
	if j.at < int64(len(j.head)) {
		n := copy(p, j.head[j.at:])
		j.at += int64(n)
		return n, nil
	}
	if max := j.size - j.at; int64(len(p)) > max {
		p = p[:max]
	}
	n, err := j.rest.ReadAt(p, j.at-int64(len(j.head)))
	j.at += int64(n)
	if err == io.EOF && n > 0 {
		err = nil
	}
	return n, err
}

func (j *joined) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		offset += j.at
	case io.SeekEnd:
		offset += j.size
	}
	if offset < 0 {
		return 0, errors.New("seek before the start")
	}
	j.at = offset
	return offset, nil
}
