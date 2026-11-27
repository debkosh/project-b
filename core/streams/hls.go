package streams

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/Eyevinn/hls-m3u8/m3u8"
)

// Plan is what a playlist asks for: the pieces of each track to fetch, or the playlists to
// read first.
type Plan struct {
	// Next are the playlists still to read, each for a rendition: a master playlist names
	// its best quality's playlist, and its sound's when that is separate. Empty once the
	// renditions are known.
	Next []string
	// Renditions are the tracks' pieces: one, or picture then sound.
	Renditions []Rendition
}

// Rendition is one playlist of pieces: picture and sound together, or one of them.
type Rendition struct {
	// SoundOnly: the rendition is the stream's separate sound, and only its sound is kept;
	// PictureOnly: the stream has separate sound, so this one's own, if any, is left out.
	SoundOnly   bool
	PictureOnly bool
	Pieces      []Piece
	// Indexed are pieces still to be known: files whose own index says where their pieces
	// are (DASH's SegmentBase), each read before the pieces are fetched.
	Indexed []Indexed
}

// Indexed is a file cut into pieces by an index inside it, a sidx box: the bytes of URL
// that hold the index, the file's header piece, and the run its pieces are of. Its pieces
// go into the rendition's at At.
type Indexed struct {
	URL   string
	Range string
	Init  *Part
	Run   int
	At    int
}

// Piece is one piece to fetch, with the header piece it needs first, if any, and its lock.
type Piece struct {
	URL string
	// Range is "first-last", the bytes of URL that are the piece, or "" for all of it.
	Range string
	// Init is the header piece of an MP4 fragment (EXT-X-MAP), nil for MPEG-TS.
	Init *Part
	// Key is the AES-128 key that locks the piece, nil for none, and IV its 16 bytes.
	Key *Part
	IV  []byte
	// Run counts the discontinuities before the piece: its clock starts over at each.
	Run int
}

// Part is a file a piece needs, shared with others: a header piece or a key.
type Part struct {
	URL   string
	Range string
	// Key and IV lock a header piece as they do a piece.
	Key *Part
	IV  []byte
}

// ReadHLS reads an HLS playlist fetched from the address base. A master playlist gives the
// playlists to read next, its chosen quality's first: its best, or with a height asked for
// (0 is none) the one closest to it; a media playlist gives its pieces. A live one is
// ErrLive and a protected one ErrProtected.
func ReadHLS(body []byte, base string, height int) (*Plan, error) {
	from, err := url.Parse(base)
	if err != nil {
		return nil, ErrFormat
	}
	// Whatever its type or its name said, a playlist starts as one.
	if Sniff(bytes.TrimLeft(body, " \t\r\n")) != HLS {
		return nil, ErrFormat
	}
	list, kind, err := m3u8.DecodeFrom(bytes.NewReader(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))), false)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormat, err)
	}
	if kind == m3u8.MASTER {
		return readMaster(list.(*m3u8.MasterPlaylist), from, height)
	}
	r, err := readMedia(list.(*m3u8.MediaPlaylist), from)
	if err != nil {
		return nil, err
	}
	return &Plan{Renditions: []Rendition{*r}}, nil
}

// readMaster chooses a master playlist's quality, and its sound's playlist when the quality
// names a separate one. The best is the highest resolution, then the highest bandwidth.
// With a height asked for, it is the quality whose height is closest to it, the smaller of
// two as close, and the highest bandwidth of that height; a playlist that gives no heights
// is saved at its best.
func readMaster(m *m3u8.MasterPlaylist, from *url.URL, height int) (*Plan, error) {
	for _, k := range m.SessionKeys {
		if k != nil && protected(*k) {
			return nil, ErrProtected
		}
	}
	var variants []*m3u8.Variant
	for _, v := range m.Variants {
		if v != nil && !v.Iframe && v.URI != "" {
			variants = append(variants, v)
		}
	}
	if len(variants) == 0 {
		return nil, ErrEmpty
	}
	sort.SliceStable(variants, func(i, j int) bool {
		a, b := pixels(variants[i].Resolution), pixels(variants[j].Resolution)
		if a != b {
			return a > b
		}
		return variants[i].Bandwidth > variants[j].Bandwidth
	})
	best := variants[0]
	if height > 0 {
		// Best first, so the first of a height is its highest bandwidth.
		var found *m3u8.Variant
		for _, v := range variants {
			if h := heightOf(v.Resolution); h > 0 && (found == nil || closer(h, heightOf(found.Resolution), height)) {
				found = v
			}
		}
		if found != nil {
			best = found
		}
	}
	plan := &Plan{Next: []string{resolve(from, best.URI)}}
	if best.Audio != "" {
		var sound *m3u8.Alternative
		for _, a := range best.Alternatives {
			if a == nil || a.Type != "AUDIO" || a.GroupId != best.Audio || a.URI == "" {
				continue
			}
			// The default one, or else the first.
			if sound == nil || a.Default && !sound.Default {
				sound = a
			}
		}
		if sound != nil {
			plan.Next = append(plan.Next, resolve(from, sound.URI))
		}
	}
	return plan, nil
}

// closer says whether a height is closer to the one asked for than another is: by less
// distance, or by being the smaller at the same distance.
func closer(h, than, asked int) bool {
	a, b := h-asked, than-asked
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	return a < b || a == b && h < than
}

// heightOf is the height a RESOLUTION of "WxH" gives, 0 for none.
func heightOf(resolution string) int {
	_, h, ok := strings.Cut(strings.ToLower(resolution), "x")
	if !ok {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(h))
	return max(n, 0)
}

// pixels is how many pixels a RESOLUTION of "WxH" has, 0 for none.
func pixels(resolution string) int {
	w, h, ok := strings.Cut(strings.ToLower(resolution), "x")
	if !ok {
		return 0
	}
	a, _ := strconv.Atoi(w)
	b, _ := strconv.Atoi(h)
	return a * b
}

// protected says a key is one the app never opens: any method but NONE and AES-128, or a
// key format other than the plain one, identity.
func protected(k m3u8.Key) bool {
	method := strings.ToUpper(k.Method)
	if method != "" && method != "NONE" && method != "AES-128" {
		return true
	}
	return k.Keyformat != "" && !strings.EqualFold(k.Keyformat, "identity")
}

func readMedia(m *m3u8.MediaPlaylist, from *url.URL) (*Rendition, error) {
	segments := m.GetAllSegments()
	for _, k := range m.Keys {
		if protected(k) {
			return nil, ErrProtected
		}
	}
	for _, s := range segments {
		for _, k := range s.Keys {
			if protected(k) {
				return nil, ErrProtected
			}
		}
	}
	if !m.Closed {
		return nil, ErrLive
	}
	if len(segments) == 0 || m.Iframe {
		return nil, ErrEmpty
	}
	r := &Rendition{}
	var key *Part
	var iv []byte
	var init *Part
	parts := map[string]*Part{}
	shared := func(p Part) *Part {
		id := p.URL + "\x00" + p.Range
		if p.Key != nil {
			id += "\x00" + p.Key.URL + "\x00" + hex.EncodeToString(p.IV)
		}
		if have := parts[id]; have != nil {
			return have
		}
		parts[id] = &p
		return &p
	}
	lock := func(keys []m3u8.Key) error {
		for _, k := range keys {
			if strings.ToUpper(k.Method) != "AES-128" {
				key, iv = nil, nil
				continue
			}
			if k.URI == "" {
				return ErrFormat
			}
			key = shared(Part{URL: resolve(from, k.URI)})
			iv = nil
			if k.IV != "" {
				b, err := hex.DecodeString(strings.TrimPrefix(strings.TrimPrefix(k.IV, "0x"), "0X"))
				if err != nil || len(b) != 16 {
					return ErrFormat
				}
				iv = b
			}
		}
		return nil
	}
	header := func(h *m3u8.Map, seq uint64) {
		if h == nil || h.URI == "" {
			return
		}
		p := Part{URL: resolve(from, h.URI), Key: key, IV: ivFor(iv, seq)}
		if h.Limit > 0 {
			p.Range = byteRange(h.Offset, h.Limit)
		}
		if key == nil {
			p.IV = nil
		}
		init = shared(p)
	}
	run := 0
	var last string
	var lastEnd int64
	for i, s := range segments {
		seq := m.SeqNo + uint64(i)
		// A key holds from the piece it is written before: the playlist's own list of keys
		// has its first wherever that comes, so each piece's is read from the piece. A header
		// piece written before the first piece is the playlist's; a later one its piece's.
		if err := lock(s.Keys); err != nil {
			return nil, err
		}
		if i == 0 {
			header(m.Map, seq)
		}
		if s.Map != nil {
			header(s.Map, seq)
		}
		if s.Discontinuity && i > 0 {
			run++
		}
		if s.Gap || s.URI == "" {
			continue
		}
		p := Piece{URL: resolve(from, s.URI), Init: init, Key: key, Run: run}
		if key != nil {
			p.IV = ivFor(iv, seq)
		}
		if s.Limit > 0 {
			// A range with no start goes on from where the last one of the same file ended.
			offset := s.Offset
			if offset == 0 && p.URL == last {
				offset = lastEnd
			}
			p.Range = byteRange(offset, s.Limit)
			lastEnd = offset + s.Limit
		} else {
			lastEnd = 0
		}
		last = p.URL
		r.Pieces = append(r.Pieces, p)
	}
	if len(r.Pieces) == 0 {
		return nil, ErrEmpty
	}
	return r, nil
}

// ivFor is a piece's IV: the playlist's, or with none its sequence number, big-endian in 16
// bytes, as the HLS spec has it.
func ivFor(iv []byte, seq uint64) []byte {
	if iv != nil {
		return iv
	}
	out := make([]byte, 16)
	for i := range 8 {
		out[15-i] = byte(seq >> (8 * i))
	}
	return out
}

func byteRange(offset, length int64) string {
	return strconv.FormatInt(offset, 10) + "-" + strconv.FormatInt(offset+length-1, 10)
}

// resolve is an address a playlist gives, against the playlist's own.
func resolve(from *url.URL, ref string) string {
	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ref
	}
	return from.ResolveReference(u).String()
}
