package streams

import (
	"encoding/hex"
	"reflect"
	"testing"
)

// What a media playlist names: each piece's address against the playlist's own, the bytes
// of a file that are the piece (a range with no start goes on from the last), the header
// piece and the key in force at it, the IV the playlist gives or else the piece's sequence
// number, and a run for each discontinuity.
func TestWhatAMediaPlaylistNames(t *testing.T) {
	plan, err := ReadHLS([]byte(`#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:4
#EXT-X-MEDIA-SEQUENCE:40
#EXT-X-MAP:URI="all.mp4",BYTERANGE="700@0"
#EXTINF:4,
#EXT-X-BYTERANGE:100@700
all.mp4
#EXTINF:4,
#EXT-X-BYTERANGE:200
all.mp4
#EXT-X-KEY:METHOD=AES-128,URI="/keys/one"
#EXTINF:4,
#EXT-X-BYTERANGE:50@2000
all.mp4
#EXT-X-DISCONTINUITY
#EXT-X-MAP:URI="https://other.test/next.mp4"
#EXT-X-KEY:METHOD=AES-128,URI="../two.key",IV=0x00112233445566778899AABBCCDDEEFF
#EXTINF:4,
next/1.m4s?sig=1
#EXT-X-KEY:METHOD=NONE
#EXTINF:4,
next/2.m4s
#EXT-X-ENDLIST
`), "https://cdn.test/films/a/index.m3u8?token=9", 0)
	if err != nil || len(plan.Renditions) != 1 || len(plan.Next) != 0 {
		t.Fatalf("%+v, %v", plan, err)
	}
	type piece struct {
		url, span, init, initSpan, key, iv string
		run                                int
	}
	var got []piece
	for _, p := range plan.Renditions[0].Pieces {
		x := piece{url: p.URL, span: p.Range, iv: hex.EncodeToString(p.IV), run: p.Run}
		if p.Init != nil {
			x.init, x.initSpan = p.Init.URL, p.Init.Range
		}
		if p.Key != nil {
			x.key = p.Key.URL
		}
		got = append(got, x)
	}
	const all, first = "https://cdn.test/films/a/all.mp4", "https://cdn.test/films/a/all.mp4"
	want := []piece{
		{url: all, span: "700-799", init: first, initSpan: "0-699"},
		{url: all, span: "800-999", init: first, initSpan: "0-699"},
		// Sequence number 42, as the IV, with a key and none given.
		{url: all, span: "2000-2049", init: first, initSpan: "0-699", key: "https://cdn.test/keys/one", iv: "0000000000000000000000000000002a"},
		{url: "https://cdn.test/films/a/next/1.m4s?sig=1", init: "https://other.test/next.mp4", key: "https://cdn.test/films/two.key", iv: "00112233445566778899aabbccddeeff", run: 1},
		{url: "https://cdn.test/films/a/next/2.m4s", init: "https://other.test/next.mp4", run: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pieces\n%+v\nwant\n%+v", got, want)
	}
	// One header piece and one key each, shared by the pieces that use them.
	p := plan.Renditions[0].Pieces
	if p[0].Init != p[2].Init || p[3].Init == p[0].Init || p[3].Init != p[4].Init {
		t.Fatal("header pieces aren't shared")
	}
}
