package streams

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h265"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts/codecs"
)

// isTS says a piece is MPEG-TS: packets of 188 bytes, each starting 0x47.
func isTS(data []byte) bool {
	return len(data) >= 188 && data[0] == 0x47 && (len(data) < 377 || data[188] == 0x47)
}

// tsClock is the units of an MPEG-TS stamp in a second, and tsWrap where its 33 bits start
// over.
const (
	tsClock = 90000
	tsWrap  = 1 << 33
)

// tsMux repacks a rendition's MPEG-TS pieces into MP4 fragments in the working file: one
// picture track, H.264 or H.265, and one sound track, AAC. Each sample keeps the piece's
// own stamps, the picture's in the TS clock and the sound's in its sampling rate.
type tsMux struct {
	w         *work
	rendition int
	r         Rendition
	seq       uint32
	video     *tsVideo
	audio     *tsAudio
}

type tsVideo struct {
	id    uint32
	t     *track
	hevc  bool
	run   int
	begun bool
	// sets are the parameter sets of each sample entry made so far, and entry the one in
	// use, from 1.
	sets  [][]byte
	entry uint32
	// last is the newest stamp, for the 33 bits' wrap; held the newest sample, kept until
	// the next says how long it lasts, and lasts the length of the one before it.
	last  int64
	turns int64
	held  *mp4.FullSample
	lasts uint32
}

type tsAudio struct {
	id    uint32
	t     *track
	rate  int64
	run   int
	begun bool
	last  int64
	turns int64
	// next is where the next sample is decoded: the end of the one before.
	next uint64
}

// unwrap is a 33-bit stamp on a clock that never starts over within a run.
func unwrap(stamp int64, last, turns *int64, begun bool) int64 {
	v := stamp + *turns
	if begun {
		for v < *last-tsWrap/2 {
			v += tsWrap
			*turns += tsWrap
		}
	}
	return v
}

type tsPicture struct {
	pts, dts int64
	au       [][]byte
}

type tsSound struct {
	pts int64
	aus [][]byte
}

// add repacks one piece of the rendition's run.
func (m *tsMux) add(run int, data []byte) error {
	reader := &mpegts.Reader{R: bytes.NewReader(data)}
	if err := reader.Initialize(); err != nil {
		return ErrFormat
	}
	var pictures []tsPicture
	var sounds []tsSound
	var sound *mpeg4audio.AudioSpecificConfig
	hevc, picture := false, false
	for _, t := range reader.Tracks() {
		switch c := t.Codec.(type) {
		case *codecs.H264:
			if picture || m.r.SoundOnly {
				continue
			}
			picture = true
			reader.OnDataH264(t, func(pts, dts int64, au [][]byte) error {
				pictures = append(pictures, tsPicture{pts, dts, au})
				return nil
			})
		case *codecs.H265:
			if picture || m.r.SoundOnly {
				continue
			}
			picture, hevc = true, true
			reader.OnDataH265(t, func(pts, dts int64, au [][]byte) error {
				pictures = append(pictures, tsPicture{pts, dts, au})
				return nil
			})
		case *codecs.MPEG4Audio:
			if sound != nil || m.r.PictureOnly {
				continue
			}
			sound = &c.Config
			reader.OnDataMPEG4Audio(t, func(pts int64, aus [][]byte) error {
				sounds = append(sounds, tsSound{pts, aus})
				return nil
			})
		}
	}
	if !picture && sound == nil {
		// A piece of a picture or a sound the app doesn't repack.
		return ErrFormat
	}
	for {
		if err := reader.Read(); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			// What was read up to a damaged packet is kept.
			break
		}
	}
	if len(pictures) > 0 {
		if err := m.addPictures(run, hevc, pictures); err != nil {
			return err
		}
	}
	if len(sounds) > 0 {
		if err := m.addSounds(run, *sound, sounds); err != nil {
			return err
		}
	}
	return nil
}

// write adds one fragment of a track's samples to the working file.
func (m *tsMux) write(run int, id, entry uint32, samples []mp4.FullSample) error {
	if len(samples) == 0 {
		return nil
	}
	m.seq++
	frag, err := mp4.CreateFragment(m.seq, id)
	if err != nil {
		return err
	}
	for _, s := range samples {
		frag.AddFullSample(s)
	}
	if entry > 1 {
		tfhd := frag.Moof.Traf.Tfhd
		tfhd.Flags |= mp4.TfhdSampleDescriptionIndexPresentFlag
		tfhd.SampleDescriptionIndex = entry
	}
	var b bytes.Buffer
	if err := frag.Encode(&b); err != nil {
		return err
	}
	return m.w.append(m.rendition, run, b.Bytes(), true)
}

func (m *tsMux) addPictures(run int, hevc bool, pictures []tsPicture) error {
	v := m.video
	if v == nil {
		v = &tsVideo{hevc: hevc, run: run}
		v.t, v.id = m.w.newTrack(m.rendition, tsClock, "video")
		m.video = v
	}
	if v.hevc != hevc {
		return ErrFormat
	}
	if v.run != run {
		// The run before ends: its last sample lasts as long as the one before it did.
		if err := m.flushVideo(); err != nil {
			return err
		}
		v.run, v.begun, v.turns = run, false, 0
	}
	var out []mp4.FullSample
	for _, p := range pictures {
		sets, sample, sync := avcc(p.au, hevc)
		if len(sample) == 0 {
			continue
		}
		// A piece with other parameter sets than the last gets a sample entry of its own.
		if len(sets) > 0 && sync {
			joined := bytes.Join(sets, []byte{0, 0, 0, 1})
			entry := uint32(0)
			for i, have := range v.sets {
				if bytes.Equal(have, joined) {
					entry = uint32(i + 1)
				}
			}
			if entry == 0 {
				if err := describe(v.t.trak, sets, hevc); err != nil {
					return ErrFormat
				}
				v.sets = append(v.sets, joined)
				entry = uint32(len(v.sets))
			}
			if entry != v.entry {
				if v.held != nil {
					v.held.Dur = v.lasts
					out = append(out, *v.held)
					v.held = nil
				}
				if err := m.write(v.run, v.id, v.entry, out); err != nil {
					return err
				}
				out, v.entry = nil, entry
			}
		}
		if v.entry == 0 {
			// Nothing can be decoded before the first parameter sets.
			continue
		}
		dts := unwrap(p.dts, &v.last, &v.turns, v.begun)
		pts := p.pts + (dts - p.dts)
		for pts < dts-tsWrap/2 {
			pts += tsWrap
		}
		if v.begun && dts <= v.last && v.held != nil {
			// A stamp that doesn't move on: the sample is decoded a tick after the last.
			dts = v.last + 1
		}
		v.last, v.begun = dts, true
		if v.held != nil {
			v.lasts = uint32(uint64(dts) - v.held.DecodeTime)
			v.held.Dur = v.lasts
			out = append(out, *v.held)
		}
		flags := mp4.NonSyncSampleFlags
		if sync {
			flags = mp4.SyncSampleFlags
		}
		v.held = &mp4.FullSample{
			Sample:     mp4.Sample{Flags: flags, Size: uint32(len(sample)), CompositionTimeOffset: int32(pts - dts)},
			DecodeTime: uint64(dts),
			Data:       sample,
		}
	}
	return m.write(v.run, v.id, v.entry, out)
}

// flushVideo writes the picture's held sample, at the end of a run or of the stream.
func (m *tsMux) flushVideo() error {
	v := m.video
	if v == nil || v.held == nil {
		return nil
	}
	if v.lasts == 0 {
		v.lasts = tsClock / 25
	}
	v.held.Dur = v.lasts
	held := *v.held
	v.held = nil
	return m.write(v.run, v.id, v.entry, []mp4.FullSample{held})
}

func (m *tsMux) addSounds(run int, config mpeg4audio.AudioSpecificConfig, sounds []tsSound) error {
	a := m.audio
	if a == nil {
		if config.SampleRate <= 0 {
			return ErrFormat
		}
		asc, err := config.Marshal()
		if err != nil {
			return ErrFormat
		}
		a = &tsAudio{rate: int64(config.SampleRate), run: run}
		a.t, a.id = m.w.newTrack(m.rendition, uint32(config.SampleRate), "audio")
		channels := config.ChannelCount
		if channels == 0 {
			channels = int(config.ChannelConfig)
		}
		a.t.trak.Mdia.Minf.Stbl.Stsd.AddChild(mp4.CreateAudioSampleEntryBox("mp4a", uint16(channels), 16, uint16(config.SampleRate), mp4.CreateEsdsBox(asc)))
		m.audio = a
	}
	if int64(config.SampleRate) != a.rate {
		return ErrFormat
	}
	if a.run != run {
		a.run, a.begun, a.turns = run, false, 0
	}
	const frame = mpeg4audio.SamplesPerAccessUnit
	var out []mp4.FullSample
	for _, s := range sounds {
		pts := unwrap(s.pts, &a.last, &a.turns, a.begun)
		a.last = pts
		// The stamp in the track's own units. Sound runs on from the sample before; a stamp
		// more than a sample later is a gap in the stream, kept.
		at := uint64(pts * a.rate / tsClock)
		if !a.begun || at > a.next+frame {
			if err := m.write(run, a.id, 1, out); err != nil {
				return err
			}
			out = nil
			if !a.begun || at > a.next {
				a.next = at
			}
		}
		a.begun = true
		for _, au := range s.aus {
			out = append(out, mp4.FullSample{
				Sample:     mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: frame, Size: uint32(len(au))},
				DecodeTime: a.next,
				Data:       au,
			})
			a.next += frame
		}
	}
	return m.write(run, a.id, 1, out)
}

// finish writes what the last piece left held.
func (m *tsMux) finish() error { return m.flushVideo() }

// avcc is an access unit as an MP4 sample: each NAL unit after its length in four bytes,
// without its delimiters, its filler and its parameter sets, which are returned beside it,
// and whether a player may start at it.
func avcc(au [][]byte, hevc bool) (sets [][]byte, sample []byte, sync bool) {
	for _, nal := range au {
		if len(nal) == 0 {
			continue
		}
		if hevc {
			switch h265.NALUType((nal[0] >> 1) & 0x3f) {
			case h265.NALUType_VPS_NUT, h265.NALUType_SPS_NUT, h265.NALUType_PPS_NUT:
				sets = append(sets, nal)
				continue
			case h265.NALUType_AUD_NUT, 38: // 38 is filler data
				continue
			}
		} else {
			switch h264.NALUType(nal[0] & 0x1f) {
			case h264.NALUTypeSPS, h264.NALUTypePPS:
				sets = append(sets, nal)
				continue
			case h264.NALUTypeAccessUnitDelimiter, h264.NALUTypeFillerData:
				continue
			}
		}
		sample = binary.BigEndian.AppendUint32(sample, uint32(len(nal)))
		sample = append(sample, nal...)
	}
	if hevc {
		sync = h265.IsRandomAccess(au)
	} else {
		sync = h264.IsRandomAccess(au)
	}
	return sets, sample, sync
}

// describe adds a sample entry for the parameter sets to a picture track.
func describe(trak *mp4.TrakBox, sets [][]byte, hevc bool) error {
	var vps, sps, pps [][]byte
	for _, nal := range sets {
		if hevc {
			switch h265.NALUType((nal[0] >> 1) & 0x3f) {
			case h265.NALUType_VPS_NUT:
				vps = append(vps, nal)
			case h265.NALUType_SPS_NUT:
				sps = append(sps, nal)
			case h265.NALUType_PPS_NUT:
				pps = append(pps, nal)
			}
		} else if h264.NALUType(nal[0]&0x1f) == h264.NALUTypeSPS {
			sps = append(sps, nal)
		} else {
			pps = append(pps, nal)
		}
	}
	if len(sps) == 0 || len(pps) == 0 {
		return ErrFormat
	}
	if hevc {
		return trak.SetHEVCDescriptor("hvc1", vps, sps, pps, nil, true)
	}
	return trak.SetAVCDescriptor("avc1", sps, pps, true)
}
