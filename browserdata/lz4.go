package browserdata

import (
	"encoding/binary"
	"errors"
)

var errLZ4 = errors.New("browserdata: a broken mozlz4 file")

// mozlz4 opens Firefox's .mozlz4 files: "mozLz40\0", the size opened as 4 bytes little
// endian, then one LZ4 block.
func mozlz4(b []byte) ([]byte, error) {
	const magic = "mozLz40\x00"
	if len(b) < len(magic)+4 || string(b[:len(magic)]) != magic {
		return nil, errLZ4
	}
	size := binary.LittleEndian.Uint32(b[len(magic):])
	return lz4Block(b[len(magic)+4:], int(size))
}

// lz4Block opens an LZ4 block to size bytes: each sequence is a token, literals, and a match
// of earlier output by its offset back.
func lz4Block(src []byte, size int) ([]byte, error) {
	out := make([]byte, 0, size)
	length := func(n int, i *int) (int, error) {
		if n != 15 {
			return n, nil
		}
		for {
			if *i >= len(src) {
				return 0, errLZ4
			}
			c := src[*i]
			*i++
			n += int(c)
			if c != 255 {
				return n, nil
			}
		}
	}
	for i := 0; i < len(src); {
		token := src[i]
		i++
		lits, err := length(int(token>>4), &i)
		if err != nil || i+lits > len(src) {
			return nil, errLZ4
		}
		out = append(out, src[i:i+lits]...)
		i += lits
		if i == len(src) {
			break
		}
		if i+2 > len(src) {
			return nil, errLZ4
		}
		offset := int(binary.LittleEndian.Uint16(src[i:]))
		i += 2
		n, err := length(int(token&15), &i)
		if err != nil || offset == 0 || offset > len(out) {
			return nil, errLZ4
		}
		n += 4
		// A match may overlap what it writes, so byte by byte.
		from := len(out) - offset
		for j := range n {
			out = append(out, out[from+j])
		}
		if len(out) > size {
			return nil, errLZ4
		}
	}
	if len(out) != size {
		return nil, errLZ4
	}
	return out, nil
}
