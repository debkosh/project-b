// Package mp4check reads a regular MP4 as a player would lay it out, for core/streams'
// tests: each track's frames in the order they are stored, when each is shown, and a sum of
// what it holds. The tests hold the MP4 the app makes of a stream to what ffmpeg makes of
// the same stream (testdata/expect.json, written by ./expect from testdata/make.sh).
package mp4check

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/Eyevinn/mp4ff/mp4"
)

// File is an MP4 as read.
type File struct {
	// MoovFirst says its index comes before its data, and Fragmented that it is cut into
	// fragments: a regular MP4 has the first and not the second.
	MoovFirst  bool `json:"moovFirst"`
	Fragmented bool `json:"fragmented"`
	// Micros is its length as its header says, in microseconds.
	Micros int64   `json:"micros"`
	Tracks []Track `json:"tracks"`
}

// Track is one track: its kind, "vide" or "soun", its codec as its sample entry names it,
// and its frames in stored order.
type Track struct {
	Kind   string  `json:"kind"`
	Codec  string  `json:"codec"`
	Frames []Frame `json:"frames"`
}

// Frame is one sample: when it is shown and for how long on the movie's own clock, edits
// applied, in microseconds; whether a player may start at it; and a sum of what it holds,
// for a picture only its slices, so that parameter sets and delimiters carried beside them
// don't count.
type Frame struct {
	At   int64  `json:"at"`
	For  int64  `json:"for"`
	Sync bool   `json:"sync,omitempty"`
	Sum  string `json:"sum"`
}

// End is when the track's last frame ends, in microseconds.
func (t Track) End() int64 {
	var end int64
	for _, f := range t.Frames {
		end = max(end, f.At+f.For)
	}
	return end
}

// Read reads the MP4 at path.
func Read(path string) (*File, error) {
	in, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	f, err := mp4.DecodeFile(in, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		return nil, err
	}
	if f.Moov == nil {
		return nil, fmt.Errorf("no moov")
	}
	out := &File{Fragmented: f.IsFragmented()}
	for _, b := range f.Children {
		if b.Type() == "moov" {
			out.MoovFirst = true
		}
		if b.Type() == "moov" || b.Type() == "mdat" {
			break
		}
	}
	movie := int64(f.Moov.Mvhd.Timescale)
	out.Micros = int64(f.Moov.Mvhd.Duration) * 1e6 / movie
	if out.Fragmented {
		return out, nil
	}
	for _, trak := range f.Moov.Traks {
		t := Track{Kind: trak.Mdia.Hdlr.HandlerType}
		stbl := trak.Mdia.Minf.Stbl
		if len(stbl.Stsd.Children) > 0 {
			t.Codec = stbl.Stsd.Children[0].Type()
		}
		n := trak.GetNrSamples()
		if n == 0 {
			out.Tracks = append(out.Tracks, t)
			continue
		}
		samples, err := trak.GetSampleData(1, n)
		if err != nil {
			return nil, err
		}
		scale := int64(trak.Mdia.Mdhd.Timescale)
		// The edits: an empty one delays the track, and the media time of the one after is
		// the media's moment shown first.
		var delay, start int64
		if trak.Edts != nil {
			for _, elst := range trak.Edts.Elst {
				for _, e := range elst.Entries {
					if e.MediaTime < 0 {
						delay += int64(e.SegmentDuration) * 1e6 / movie
						continue
					}
					start = e.MediaTime
					break
				}
			}
		}
		var decode int64
		for i, s := range samples {
			data, err := f.ReadSampleData(trak, uint32(i+1), uint32(i+1), in, nil)
			if err != nil {
				return nil, err
			}
			if t.Kind == "vide" {
				data = slices(data, t.Codec)
			}
			sum := md5.Sum(data)
			t.Frames = append(t.Frames, Frame{
				At:   (decode+int64(s.CompositionTimeOffset)-start)*1e6/scale + delay,
				For:  int64(s.Dur) * 1e6 / scale,
				Sync: s.IsSync(),
				Sum:  hex.EncodeToString(sum[:8]),
			})
			decode += int64(s.Dur)
		}
		out.Tracks = append(out.Tracks, t)
	}
	return out, nil
}

// slices is a picture's coded slices alone, out of a sample of length-prefixed NAL units:
// H.264's types 1 to 5, H.265's below 32.
func slices(sample []byte, codec string) []byte {
	var out []byte
	for len(sample) >= 4 {
		n := int(binary.BigEndian.Uint32(sample))
		sample = sample[4:]
		if n > len(sample) || n == 0 {
			break
		}
		nal := sample[:n]
		sample = sample[n:]
		keep := false
		switch codec {
		case "hvc1", "hev1":
			keep = (nal[0]>>1)&0x3f < 32
		default:
			kind := nal[0] & 0x1f
			keep = kind >= 1 && kind <= 5
		}
		if keep {
			out = append(out, nal...)
		}
	}
	return out
}
