package browserdata

import (
	"encoding/binary"
	"errors"
	"math"
	"time"
	"unicode/utf16"
)

// bplist reads a binary property list, as Safari's Bookmarks.plist and macOS's preferences
// are kept, into Go's values: map[string]any, []any, string, int64, float64, bool,
// time.Time, []byte and nil.
func bplist(b []byte) (any, error) {
	if len(b) < 8+32 || string(b[:8]) != "bplist00" {
		return nil, errBplist
	}
	t := b[len(b)-32:]
	p := &plistReader{
		b:          b,
		offsetSize: int(t[6]),
		refSize:    int(t[7]),
		count:      binary.BigEndian.Uint64(t[8:]),
		table:      binary.BigEndian.Uint64(t[24:]),
	}
	if p.offsetSize == 0 || p.refSize == 0 || p.table >= uint64(len(b)) || p.count > uint64(len(b)) {
		return nil, errBplist
	}
	return p.object(binary.BigEndian.Uint64(t[16:]), 0)
}

var errBplist = errors.New("browserdata: not a binary property list")

type plistReader struct {
	b                   []byte
	offsetSize, refSize int
	count, table        uint64
}

// uint reads n bytes at at as a big-endian number.
func (p *plistReader) uint(at uint64, n int) (uint64, error) {
	if n > 8 || at+uint64(n) > uint64(len(p.b)) {
		return 0, errBplist
	}
	var v uint64
	for _, c := range p.b[at : at+uint64(n)] {
		v = v<<8 | uint64(c)
	}
	return v, nil
}

// object reads object i; depth guards a list that holds itself.
func (p *plistReader) object(i uint64, depth int) (any, error) {
	if i >= p.count || depth > 512 {
		return nil, errBplist
	}
	at, err := p.uint(p.table+i*uint64(p.offsetSize), p.offsetSize)
	if err != nil || at >= uint64(len(p.b)) {
		return nil, errBplist
	}
	marker := p.b[at]
	kind, low := marker>>4, int(marker&0xf)
	at++
	// size is the length a data, string, array or dict says, after its marker.
	size := func() (int, error) {
		if low != 0xf {
			return low, nil
		}
		if at >= uint64(len(p.b)) || p.b[at]>>4 != 1 {
			return 0, errBplist
		}
		n := 1 << (p.b[at] & 0xf)
		v, err := p.uint(at+1, n)
		at += 1 + uint64(n)
		if v > uint64(len(p.b)) {
			return 0, errBplist
		}
		return int(v), err
	}
	switch kind {
	case 0x0:
		switch marker {
		case 0x08:
			return false, nil
		case 0x09:
			return true, nil
		}
		return nil, nil
	case 0x1:
		n := 1 << low
		if n == 16 {
			// A 128-bit number, which macOS writes for one past int64: its low half.
			at, n = at+8, 8
		}
		v, err := p.uint(at, n)
		return int64(v), err
	case 0x2, 0x3:
		v, err := p.uint(at, 1<<(low&3))
		f := float64(math.Float32frombits(uint32(v)))
		if low&3 == 3 {
			f = math.Float64frombits(v)
		}
		if kind == 0x3 {
			return appleTime(f), err
		}
		return f, err
	case 0x4, 0x5, 0x6:
		n, err := size()
		if err != nil {
			return nil, err
		}
		width := 1
		if kind == 0x6 {
			width = 2
		}
		end := at + uint64(n*width)
		if end > uint64(len(p.b)) {
			return nil, errBplist
		}
		raw := p.b[at:end]
		switch kind {
		case 0x4:
			return append([]byte(nil), raw...), nil
		case 0x5:
			return string(raw), nil
		}
		u := make([]uint16, n)
		for j := range u {
			u[j] = binary.BigEndian.Uint16(raw[2*j:])
		}
		return string(utf16.Decode(u)), nil
	case 0x8:
		v, err := p.uint(at, low+1)
		return int64(v), err
	case 0xa, 0xd:
		n, err := size()
		if err != nil {
			return nil, err
		}
		refs := n
		if kind == 0xd {
			refs = 2 * n
		}
		ids := make([]uint64, refs)
		for j := range ids {
			if ids[j], err = p.uint(at+uint64(j*p.refSize), p.refSize); err != nil {
				return nil, err
			}
		}
		if kind == 0xa {
			out := make([]any, n)
			for j := range out {
				if out[j], err = p.object(ids[j], depth+1); err != nil {
					return nil, err
				}
			}
			return out, nil
		}
		out := make(map[string]any, n)
		for j := 0; j < n; j++ {
			k, err := p.object(ids[j], depth+1)
			if err != nil {
				return nil, err
			}
			key, ok := k.(string)
			if !ok {
				return nil, errBplist
			}
			if out[key], err = p.object(ids[n+j], depth+1); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	return nil, errBplist
}

// appleTime is a time kept as Apple keeps it: seconds since the start of 2001, in UTC.
func appleTime(s float64) time.Time {
	if s == 0 {
		return time.Time{}
	}
	sec, frac := math.Modf(s)
	return time.Unix(int64(sec)+978307200, int64(frac*1e9)).UTC()
}
