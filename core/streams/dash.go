package streams

import (
	"bytes"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/Eyevinn/dash-mpd/mpd"
	"github.com/Eyevinn/mp4ff/mp4"
)

// ReadDASH reads a DASH playlist (an MPD) fetched from the address base: its best picture,
// the Representation of the greatest height and then the greatest bandwidth, or with a
// height asked for (0 is none) the one closest to it, and its best sound, each a rendition
// of MP4 fragments. A live one (type="dynamic") is ErrLive, and one
// with any ContentProtection is ErrProtected. Each Period is a run of its own. Pieces are
// named by a SegmentTemplate ($Number$ or $Time$, with a duration or a SegmentTimeline), a
// SegmentList, or a SegmentBase, whose pieces the file's own index gives (Indexed).
func ReadDASH(body []byte, base string, height int) (*Plan, error) {
	from, err := url.Parse(base)
	if err != nil {
		return nil, ErrFormat
	}
	if Sniff(body) != DASH {
		return nil, ErrFormat
	}
	m, err := mpd.MPDFromBytes(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormat, err)
	}
	if len(m.ContentProtection) > 0 {
		return nil, ErrProtected
	}
	for _, p := range m.Periods {
		for _, a := range p.AdaptationSets {
			if len(a.ContentProtections) > 0 {
				return nil, ErrProtected
			}
			for _, r := range a.Representations {
				if len(r.ContentProtections) > 0 {
					return nil, ErrProtected
				}
			}
		}
	}
	if m.GetType() == "dynamic" {
		return nil, ErrLive
	}
	from = within(from, m.BaseURL)
	var picture, sound Rendition
	for run, p := range m.Periods {
		// How long the period is, for a template that counts its pieces by their length.
		length := math.Inf(1)
		if d, err := p.GetDuration(); err == nil && d > 0 {
			length = d.Seconds()
		}
		at := within(from, p.BaseURLs)
		v, a := best(p, "video", height), best(p, "audio", 0)
		for _, c := range []struct {
			into *Rendition
			rep  *mpd.RepresentationType
		}{{&picture, v}, {&sound, a}} {
			if c.rep == nil {
				continue
			}
			if err := c.into.addRepresentation(c.rep, at, p, length, run); err != nil {
				return nil, err
			}
		}
	}
	plan := &Plan{}
	has := func(r Rendition) bool { return len(r.Pieces) > 0 || len(r.Indexed) > 0 }
	switch {
	case has(picture) && has(sound):
		picture.PictureOnly, sound.SoundOnly = true, true
		plan.Renditions = []Rendition{picture, sound}
	case has(picture):
		plan.Renditions = []Rendition{picture}
	case has(sound):
		plan.Renditions = []Rendition{sound}
	default:
		return nil, ErrEmpty
	}
	return plan, nil
}

// within is the address the first of a level's BaseURLs gives, against the level above.
func within(from *url.URL, bases []*mpd.BaseURLType) *url.URL {
	if len(bases) == 0 || bases[0] == nil || strings.TrimSpace(string(bases[0].Value)) == "" {
		return from
	}
	if u, err := url.Parse(resolve(from, string(bases[0].Value))); err == nil {
		return u
	}
	return from
}

// best is a period's best Representation of a kind, "video" or "audio", among those in
// MP4: for a picture the greatest height and then the greatest bandwidth; for sound the
// greatest bandwidth of the first AdaptationSet that has sound, the stream's own first
// choice of language. With a height asked for, the picture is the one whose height is
// closest to it, the smaller of two as close, and the greatest bandwidth of that height;
// where none says its height, the best.
func best(p *mpd.Period, kind string, asked int) *mpd.RepresentationType {
	if asked > 0 {
		for _, a := range p.AdaptationSets {
			for _, r := range a.Representations {
				if height(r) > 0 {
					return closest(p, kind, asked)
				}
			}
		}
	}
	var found *mpd.RepresentationType
	for _, a := range p.AdaptationSets {
		for _, r := range a.Representations {
			if !isKind(a, r, kind) {
				continue
			}
			if found == nil || height(r) > height(found) || height(r) == height(found) && r.Bandwidth > found.Bandwidth {
				found = r
			}
		}
		if found != nil && kind == "audio" {
			break
		}
	}
	return found
}

// closest is a period's Representation of a kind in MP4 whose height is closest to asked.
func closest(p *mpd.Period, kind string, asked int) *mpd.RepresentationType {
	var found *mpd.RepresentationType
	for _, a := range p.AdaptationSets {
		for _, r := range a.Representations {
			if !isKind(a, r, kind) || height(r) == 0 {
				continue
			}
			h := int(height(r))
			if found == nil || closer(h, int(height(found)), asked) || h == int(height(found)) && r.Bandwidth > found.Bandwidth {
				found = r
			}
		}
	}
	return found
}

// isKind says whether a Representation is of a kind, "video" or "audio", and in MP4.
func isKind(a *mpd.AdaptationSetType, r *mpd.RepresentationType, kind string) bool {
	mime := strings.ToLower(r.GetMimeType())
	is := strings.EqualFold(string(a.ContentType), kind) || strings.HasPrefix(mime, kind+"/")
	return is && (mime == "" || strings.Contains(mime, "mp4"))
}

func height(r *mpd.RepresentationType) uint32 {
	if r.Height != 0 || r.Parent() == nil {
		return r.Height
	}
	return max(r.Parent().Height, r.Parent().MaxHeight)
}

// addRepresentation adds a period's Representation's pieces to the rendition, as its run.
func (r *Rendition) addRepresentation(rep *mpd.RepresentationType, at *url.URL, p *mpd.Period, length float64, run int) error {
	a := rep.Parent()
	at = within(within(at, a.BaseURLs), rep.BaseURLs)
	// Each way of naming pieces is taken from the Representation, else its AdaptationSet,
	// else its Period.
	if t := firstOf(rep.SegmentTemplate, a.SegmentTemplate, p.SegmentTemplate); t != nil {
		// What the nearest leaves out, the levels above give.
		for _, up := range []*mpd.SegmentTemplateType{rep.SegmentTemplate, a.SegmentTemplate, p.SegmentTemplate} {
			if up == nil || up == t {
				continue
			}
			merged := *t
			if merged.Media == "" {
				merged.Media = up.Media
			}
			if merged.Initialization == "" {
				merged.Initialization = up.Initialization
			}
			if merged.Timescale == nil {
				merged.Timescale = up.Timescale
			}
			if merged.Duration == nil {
				merged.Duration = up.Duration
			}
			if merged.StartNumber == nil {
				merged.StartNumber = up.StartNumber
			}
			if merged.EndNumber == nil {
				merged.EndNumber = up.EndNumber
			}
			if merged.SegmentTimeline == nil {
				merged.SegmentTimeline = up.SegmentTimeline
			}
			t = &merged
		}
		return r.addTemplate(t, rep, at, length, run)
	}
	if l := firstOf(rep.SegmentList, a.SegmentList, p.SegmentList); l != nil {
		var init *Part
		if l.Initialization != nil {
			init = &Part{URL: at.String(), Range: l.Initialization.Range}
			if l.Initialization.SourceURL != "" {
				init.URL = resolve(at, string(l.Initialization.SourceURL))
			}
		}
		for _, s := range l.SegmentURL {
			if s == nil {
				continue
			}
			piece := Piece{URL: at.String(), Range: string(s.MediaRange), Init: init, Run: run}
			if s.Media != "" {
				piece.URL = resolve(at, string(s.Media))
			}
			r.Pieces = append(r.Pieces, piece)
		}
		return nil
	}
	b := firstOf(rep.SegmentBase, a.SegmentBase, p.SegmentBase)
	if b == nil || b.IndexRange == "" {
		// One file, whole: its header piece is its own start.
		r.Pieces = append(r.Pieces, Piece{URL: at.String(), Run: run})
		return nil
	}
	var init *Part
	if b.Initialization != nil {
		init = &Part{URL: at.String(), Range: b.Initialization.Range}
		if b.Initialization.SourceURL != "" {
			init.URL = resolve(at, string(b.Initialization.SourceURL))
		}
	} else if first, _, ok := strings.Cut(b.IndexRange, "-"); ok {
		// With none named, the header piece is what comes before the index.
		if n, err := strconv.ParseInt(first, 10, 64); err == nil && n > 0 {
			init = &Part{URL: at.String(), Range: byteRange(0, n)}
		}
	}
	r.Indexed = append(r.Indexed, Indexed{URL: at.String(), Range: b.IndexRange, Init: init, Run: run, At: len(r.Pieces)})
	return nil
}

func firstOf[T any](of ...*T) *T {
	for _, x := range of {
		if x != nil {
			return x
		}
	}
	return nil
}

// addTemplate names a Representation's pieces from its SegmentTemplate: by a
// SegmentTimeline, each S a piece's start and length, repeated r times more; or by one
// length for all, as many as fill the period.
func (r *Rendition) addTemplate(t *mpd.SegmentTemplateType, rep *mpd.RepresentationType, at *url.URL, length float64, run int) error {
	if t.Media == "" {
		return ErrFormat
	}
	var init *Part
	if t.Initialization != "" {
		init = &Part{URL: resolve(at, fill(t.Initialization, rep, 0, 0))}
	}
	number := uint64(1)
	if t.StartNumber != nil {
		number = uint64(*t.StartNumber)
	}
	last := uint64(math.MaxUint64)
	if t.EndNumber != nil {
		last = uint64(*t.EndNumber)
	}
	add := func(time uint64) {
		r.Pieces = append(r.Pieces, Piece{URL: resolve(at, fill(t.Media, rep, number, time)), Init: init, Run: run})
		number++
	}
	if t.SegmentTimeline != nil {
		var time uint64
		for i, s := range t.SegmentTimeline.S {
			if s == nil || s.D == 0 {
				return ErrFormat
			}
			if s.T != nil {
				time = *s.T
			}
			repeat := s.R
			if repeat < 0 {
				// To the next S, or the period's end.
				end := math.Inf(1)
				if i+1 < len(t.SegmentTimeline.S) && t.SegmentTimeline.S[i+1] != nil && t.SegmentTimeline.S[i+1].T != nil {
					end = float64(*t.SegmentTimeline.S[i+1].T)
				} else if !math.IsInf(length, 1) {
					end = length * float64(scaleOf(t))
				}
				if math.IsInf(end, 1) {
					return ErrFormat
				}
				repeat = max(int(math.Ceil((end-float64(time))/float64(s.D)))-1, 0)
			}
			for n := 0; n <= repeat && number <= last; n++ {
				if len(r.Pieces) >= pieceLimit {
					return ErrFormat
				}
				add(time)
				time += s.D
			}
		}
		return nil
	}
	if t.Duration == nil || *t.Duration == 0 {
		return ErrFormat
	}
	each := float64(*t.Duration) / float64(scaleOf(t))
	count := uint64(math.MaxUint64)
	if !math.IsInf(length, 1) {
		// A hair short of a whole piece is rounding, not a piece.
		count = uint64(math.Ceil(length/each - 1e-6))
	} else if t.EndNumber == nil {
		return ErrFormat
	}
	for n := uint64(0); n < count && number <= last; n++ {
		if len(r.Pieces) >= pieceLimit {
			return ErrFormat
		}
		add(n * uint64(*t.Duration))
	}
	return nil
}

// pieceLimit is the most pieces one stream may name: a day of one-second pieces.
const pieceLimit = 100000

func scaleOf(t *mpd.SegmentTemplateType) uint32 {
	if t.Timescale != nil && *t.Timescale > 0 {
		return *t.Timescale
	}
	return 1
}

// fill is a template's address with its identifiers filled in: $RepresentationID$,
// $Bandwidth$, $Number$ and $Time$, the last three with a width as printf's %0Nd when
// written $Number%05d$, and $$ for a dollar.
func fill(template string, rep *mpd.RepresentationType, number, time uint64) string {
	var out strings.Builder
	for {
		start := strings.IndexByte(template, '$')
		if start < 0 {
			break
		}
		end := strings.IndexByte(template[start+1:], '$')
		if end < 0 {
			break
		}
		out.WriteString(template[:start])
		name, format, _ := strings.Cut(template[start+1:start+1+end], "%")
		width := 1
		if w, ok := strings.CutSuffix(format, "d"); ok {
			if n, err := strconv.Atoi(strings.TrimPrefix(w, "0")); err == nil && n > 0 && n < 32 {
				width = n
			}
		}
		pad := func(v uint64) string { return fmt.Sprintf("%0*d", width, v) }
		switch name {
		case "":
			out.WriteByte('$')
		case "RepresentationID":
			out.WriteString(rep.Id)
		case "Bandwidth":
			out.WriteString(pad(uint64(rep.Bandwidth)))
		case "Number":
			out.WriteString(pad(number))
		case "Time":
			out.WriteString(pad(time))
		default:
			out.WriteString(template[start : start+2+end])
		}
		template = template[start+2+end:]
	}
	out.WriteString(template)
	return out.String()
}

// indexed is a file's pieces, as the index in its bytes at span says them: a sidx box,
// each of whose references is a piece, one after another from where the index ends.
func indexed(x Indexed, index []byte) ([]Piece, error) {
	_, last, ok := strings.Cut(x.Range, "-")
	end, err := strconv.ParseInt(last, 10, 64)
	if !ok || err != nil {
		return nil, ErrFormat
	}
	f, err := mp4.DecodeFile(bytes.NewReader(index))
	if err != nil || f.Sidx == nil {
		return nil, ErrFormat
	}
	at := end + 1 + int64(f.Sidx.FirstOffset)
	var pieces []Piece
	for _, ref := range f.Sidx.SidxRefs {
		// An index of indexes isn't read.
		if ref.ReferenceType != 0 || ref.ReferencedSize == 0 || len(pieces) >= pieceLimit {
			return nil, ErrFormat
		}
		pieces = append(pieces, Piece{URL: x.URL, Range: byteRange(at, int64(ref.ReferencedSize)), Init: x.Init, Run: x.Run})
		at += int64(ref.ReferencedSize)
	}
	if len(pieces) == 0 {
		return nil, ErrEmpty
	}
	return pieces, nil
}
