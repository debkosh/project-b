package streams

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"projectb/engine"
)

// DASH with a SegmentTemplate of $Number$ and one length for every piece: the picture's
// and the sound's Representations, each in pieces of its own, become one MP4.
func TestDASHByNumberBecomesOneMP4(t *testing.T) {
	r := newRig(t)
	s := r.save("/dash/template/manifest.mpd", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatal(s.err)
	}
	got := same(t, s.out, expected(t, "dash-template"))
	if got.Tracks[0].Kind != "vide" || got.Tracks[1].Kind != "soun" || !near(got.Micros, 3_000_000, 60_000) {
		t.Errorf("tracks %s, %s, %d long", got.Tracks[0].Kind, got.Tracks[1].Kind, got.Micros)
	}
	// As many pieces as the playlist's length holds, each header piece once, each as from
	// the page, from where the playlist's server is.
	asked := strings.Join(r.requests(), " ")
	for _, want := range []string{"init-stream0.m4s", "init-stream1.m4s", "chunk-stream0-00001.m4s", "chunk-stream0-00003.m4s", "chunk-stream1-00003.m4s"} {
		if strings.Count(asked, "/dash/template/"+want) != 1 {
			t.Errorf("%s asked for %d times in %s", want, strings.Count(asked, want), asked)
		}
	}
	if strings.Contains(asked, "00004") || len(r.requests()) != 8 {
		t.Errorf("asked for %s", asked)
	}
	if last := s.progress[len(s.progress)-1]; last.Pieces != 6 || last.Total != 6 {
		t.Errorf("progress ended %+v", last)
	}
	for _, f := range r.wanted {
		if f.Page != "https://video.test/watch?v=1" || f.Workspace != "w1" || f.From != engine.SpaceLoopback {
			t.Errorf("fetched as %+v", f)
		}
	}
}

// DASH with a SegmentTimeline, and the same named by $Time$.
func TestDASHByTimelineBecomesOneMP4(t *testing.T) {
	r := newRig(t)
	s := r.save("/dash/timeline/manifest.mpd", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatal(s.err)
	}
	same(t, s.out, expected(t, "dash-timeline"))

	// The same stream with its pieces named by their start, $Time$, served under those names.
	b, err := os.ReadFile("testdata/dash/timeline/manifest.mpd")
	if err != nil {
		t.Fatal(err)
	}
	r = newRig(t)
	r.extra["/dash/timeline/time.mpd"] = strings.ReplaceAll(string(b), "chunk-stream$RepresentationID$-$Number%05d$.m4s", "at-$RepresentationID$-$Time$.m4s")
	for name, to := range map[string]string{
		"at-0-0": "chunk-stream0-00001", "at-0-12800": "chunk-stream0-00002", "at-0-25600": "chunk-stream0-00003",
		"at-1-0": "chunk-stream1-00001", "at-1-40960": "chunk-stream1-00002", "at-1-84992": "chunk-stream1-00003", "at-1-130048": "chunk-stream1-00004",
	} {
		r.alias["/dash/timeline/"+name+".m4s"] = "/dash/timeline/" + to + ".m4s"
	}
	s = r.save("/dash/timeline/time.mpd", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatalf("by $Time$: %v, asked %v", s.err, r.requests())
	}
	same(t, s.out, expected(t, "dash-timeline"))
}

// DASH with each Representation in one file: a SegmentList of byte ranges, and a
// SegmentBase, whose pieces the file's own index, a sidx box, says.
func TestDASHInOneFileBecomesOneMP4(t *testing.T) {
	r := newRig(t)
	s := r.save("/dash/single/manifest.mpd", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatal(s.err)
	}
	same(t, s.out, expected(t, "dash-single"))
	ranged := 0
	for _, f := range r.wanted {
		if f.Range != "" {
			ranged++
		}
	}
	if ranged != len(r.wanted) || ranged != 2+3+4 {
		t.Errorf("%d of %d fetches asked for a range: %+v", ranged, len(r.wanted), r.wanted)
	}

	r = newRig(t)
	s = r.save("/dash/single/base.mpd", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatal(s.err)
	}
	same(t, s.out, expected(t, "dash-single"))
	// The index of each file, its header piece, then its pieces by the index's ranges.
	var spans []string
	for _, f := range r.wanted {
		if strings.HasSuffix(f.URL, "manifest-stream0.mp4") {
			spans = append(spans, f.Range)
		}
	}
	if want := []string{"804-879", "0-803", "880-3918", "3919-6492", "6493-9288"}; !reflect.DeepEqual(spans, want) {
		t.Errorf("the picture's file was asked for %v, want %v", spans, want)
	}
}

const mpdHead = `<?xml version="1.0" encoding="utf-8"?>
<!-- a playlist -->
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" profiles="urn:mpeg:dash:profile:isoff-live:2011" `

func mpdOf(attrs, period string) string {
	return mpdHead + attrs + ">\n<Period id=\"0\" start=\"PT0S\">" + period + "</Period>\n</MPD>\n"
}

const template = `<SegmentTemplate timescale="1000" duration="1000" initialization="init-stream$RepresentationID$.m4s" media="chunk-stream$RepresentationID$-$Number%05d$.m4s" startNumber="1"/>`

// A live DASH stream and a protected one are refused, in the words the row shows, before
// any piece is fetched.
func TestLiveAndProtectedDASHAreRefused(t *testing.T) {
	video := func(inside string) string {
		return `<AdaptationSet contentType="video">` + inside + `<Representation id="0" mimeType="video/mp4" bandwidth="30000" width="160" height="90">` + template + `</Representation></AdaptationSet>`
	}
	widevine := `<ContentProtection schemeIdUri="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed"/>`
	for name, c := range map[string]struct {
		list string
		want error
	}{
		"live":      {mpdOf(`type="dynamic" availabilityStartTime="2026-01-01T00:00:00Z" minimumUpdatePeriod="PT2S"`, video("")), ErrLive},
		"protected": {mpdOf(`type="static" mediaPresentationDuration="PT3S"`, video(`<ContentProtection schemeIdUri="urn:mpeg:dash:mp4protection:2011" value="cenc"/>`+widevine)), ErrProtected},
		"in the representation": {mpdOf(`type="static" mediaPresentationDuration="PT3S"`,
			`<AdaptationSet contentType="video"><Representation id="0" mimeType="video/mp4" bandwidth="30000" height="90">`+widevine+template+`</Representation></AdaptationSet>`), ErrProtected},
		"only the sound": {mpdOf(`type="static" mediaPresentationDuration="PT3S"`,
			video("")+`<AdaptationSet contentType="audio">`+widevine+`<Representation id="1" mimeType="audio/mp4" bandwidth="16000">`+template+`</Representation></AdaptationSet>`), ErrProtected},
		"both": {mpdOf(`type="dynamic"`, video(widevine)), ErrProtected},
	} {
		r := newRig(t)
		r.extra["/dash/template/"+name+".mpd"] = c.list
		s := r.save("/dash/template/"+name+".mpd", engine.SpaceLoopback)
		reason, _ := Reason(s.err)
		if want := map[error]string{ErrLive: "live", ErrProtected: "protected"}[c.want]; !errors.Is(s.err, c.want) || reason != want {
			t.Errorf("%s: %v, said %q", name, s.err, reason)
		}
		if asked := r.requests(); len(asked) != 0 {
			t.Errorf("%s: fetched %v", name, asked)
		}
	}
	// Pieces that are encrypted, whatever the playlist says: the header piece says so.
	r := newRig(t)
	r.extra["/dash/template/plain.mpd"] = mpdOf(`type="static" mediaPresentationDuration="PT3S"`,
		`<AdaptationSet contentType="video"><Representation id="0" mimeType="video/mp4" bandwidth="30000" height="90">`+
			strings.Replace(template, "init-stream$RepresentationID$.m4s", "locked-init.m4s", 1)+`</Representation></AdaptationSet>`)
	init, err := os.ReadFile("testdata/dash/template/init-stream0.m4s")
	if err != nil {
		t.Fatal(err)
	}
	r.extra["/dash/template/locked-init.m4s"] = string(withPssh(init))
	if s := r.save("/dash/template/plain.mpd", engine.SpaceLoopback); !errors.Is(s.err, ErrProtected) {
		t.Errorf("encrypted pieces under a plain playlist: %v", s.err)
	}
}

// withPssh is a header piece with a protection box put into its moov, as an encrypted
// stream's has.
func withPssh(init []byte) []byte {
	pssh := append([]byte{0, 0, 0, 32, 'p', 's', 's', 'h', 0, 0, 0, 0}, make([]byte, 20)...)
	out := append([]byte{}, init...)
	for at := 0; at+8 <= len(out); {
		size := int(out[at])<<24 | int(out[at+1])<<16 | int(out[at+2])<<8 | int(out[at+3])
		if string(out[at+4:at+8]) == "moov" {
			grown := size + len(pssh)
			out[at], out[at+1], out[at+2], out[at+3] = byte(grown>>24), byte(grown>>16), byte(grown>>8), byte(grown)
			return append(out[:at+size:at+size], append(pssh, out[at+size:]...)...)
		}
		at += size
	}
	return out
}

// The fetch's rules hold for DASH as for HLS.
func TestDASHFromTheInternetMayNotReachThisComputer(t *testing.T) {
	for _, path := range []string{"/dash/template/manifest.mpd", "/dash/single/manifest.mpd", "/dash/single/base.mpd"} {
		r := newRig(t)
		s := r.save(path, engine.SpacePublic)
		if !errors.Is(s.err, ErrRefused) {
			t.Errorf("%s: %v", path, s.err)
		}
		if asked := r.requests(); len(asked) != 0 {
			t.Errorf("%s: this computer was asked for %v", path, asked)
		}
	}
	r := newRig(t)
	r.extra["/dash/template/file.mpd"] = mpdOf(`type="static" mediaPresentationDuration="PT3S"`,
		`<BaseURL>file:///etc/</BaseURL><AdaptationSet contentType="video"><Representation id="0" mimeType="video/mp4" bandwidth="30000" height="90">`+template+`</Representation></AdaptationSet>`)
	s := r.save("/dash/template/file.mpd", engine.SpaceLoopback)
	if !errors.Is(s.err, ErrRefused) || len(r.wanted) != 0 {
		t.Errorf("pieces in a file: %v, asked %+v", s.err, r.wanted)
	}
}

// What a DASH playlist names: the best picture, the greatest height and then the greatest
// bandwidth, among those in MP4; the first sound's best; and its pieces by each way of
// naming them, with what the levels above the Representation give.
func TestWhatADASHPlaylistNames(t *testing.T) {
	urls := func(r Rendition) []string {
		var out []string
		for _, p := range r.Pieces {
			u := strings.TrimPrefix(p.URL, "https://cdn.test/")
			if p.Range != "" {
				u += "@" + p.Range
			}
			out = append(out, u)
		}
		return out
	}
	plan, err := ReadDASH([]byte(mpdOf(`type="static" mediaPresentationDuration="PT5.5S"`, `
		<BaseURL>media/</BaseURL>
		<AdaptationSet contentType="video">
			<SegmentTemplate timescale="1000" initialization="$RepresentationID$/init.mp4" media="$RepresentationID$/$Bandwidth$-$Number$.m4s" startNumber="7"/>
			<Representation id="low" mimeType="video/mp4" bandwidth="900000" width="640" height="360"><SegmentTemplate duration="2000"/></Representation>
			<Representation id="webm" mimeType="video/webm" bandwidth="9000000" width="3840" height="2160"><SegmentTemplate duration="2000"/></Representation>
			<Representation id="hd" mimeType="video/mp4" bandwidth="500000" width="1920" height="1080"><SegmentTemplate duration="2000"/></Representation>
			<Representation id="hd2" mimeType="video/mp4" bandwidth="400000" width="1920" height="1080"><SegmentTemplate duration="2000"/></Representation>
		</AdaptationSet>
		<AdaptationSet contentType="audio" lang="en">
			<Representation id="a1" mimeType="audio/mp4" bandwidth="64000"><BaseURL>en/</BaseURL><SegmentList><Initialization sourceURL="head.mp4"/><SegmentURL media="1.m4s"/><SegmentURL media="2.m4s" mediaRange="10-19"/></SegmentList></Representation>
			<Representation id="a2" mimeType="audio/mp4" bandwidth="128000"><BaseURL>en-best/</BaseURL><SegmentList><Initialization sourceURL="head.mp4"/><SegmentURL media="1.m4s"/><SegmentURL media="2.m4s" mediaRange="10-19"/></SegmentList></Representation>
		</AdaptationSet>
		<AdaptationSet contentType="audio" lang="fr">
			<Representation id="fr" mimeType="audio/mp4" bandwidth="256000"><BaseURL>fr/</BaseURL><SegmentList><SegmentURL media="1.m4s"/></SegmentList></Representation>
		</AdaptationSet>
		<AdaptationSet contentType="text"><Representation id="subs" mimeType="application/mp4" bandwidth="1000"><SegmentTemplate duration="2000"/></Representation></AdaptationSet>`)),
		"https://cdn.test/v/manifest.mpd?token=1")
	if err != nil || len(plan.Renditions) != 2 || !plan.Renditions[0].PictureOnly || !plan.Renditions[1].SoundOnly {
		t.Fatalf("%+v, %v", plan, err)
	}
	// 5.5 seconds in 2 second pieces is 3 of them, numbered from 7.
	if got, want := urls(plan.Renditions[0]), []string{"v/media/hd/500000-7.m4s", "v/media/hd/500000-8.m4s", "v/media/hd/500000-9.m4s"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the picture's pieces %v, want %v", got, want)
	}
	if init := plan.Renditions[0].Pieces[0].Init; init == nil || init.URL != "https://cdn.test/v/media/hd/init.mp4" || plan.Renditions[0].Pieces[2].Init != init {
		t.Errorf("the picture's header piece %+v", init)
	}
	if got, want := urls(plan.Renditions[1]), []string{"v/media/en-best/1.m4s", "v/media/en-best/2.m4s@10-19"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the sound's pieces %v, want %v", got, want)
	}
	if init := plan.Renditions[1].Pieces[0].Init; init == nil || init.URL != "https://cdn.test/v/media/en-best/head.mp4" {
		t.Errorf("the sound's header piece %+v", init)
	}

	// A timeline: each S a start and a length, r more of them, a gap where t says so, with
	// $Time$ and a padded $Number$; and two periods, each a run of its own.
	plan, err = ReadDASH([]byte(mpdHead+`type="static">
		<Period id="a" duration="PT4S"><AdaptationSet mimeType="video/mp4"><Representation id="v" bandwidth="1" height="90">
			<SegmentTemplate timescale="10" media="a-$Time$-$Number%03d$.m4s" initialization="a.mp4"><SegmentTimeline><S t="100" d="20" r="1"/><S d="15"/><S t="500" d="20"/></SegmentTimeline></SegmentTemplate>
		</Representation></AdaptationSet></Period>
		<Period id="b" duration="PT2S"><AdaptationSet mimeType="video/mp4"><Representation id="v" bandwidth="1" height="90">
			<SegmentTemplate timescale="10" duration="10" media="b-$Number$$$.m4s" initialization="b.mp4"/>
		</Representation></AdaptationSet></Period>
	</MPD>`), "https://cdn.test/manifest.mpd")
	if err != nil || len(plan.Renditions) != 1 || plan.Renditions[0].PictureOnly {
		t.Fatalf("%+v, %v", plan, err)
	}
	if got, want := urls(plan.Renditions[0]), []string{"a-100-001.m4s", "a-120-002.m4s", "a-140-003.m4s", "a-500-004.m4s", "b-1$.m4s", "b-2$.m4s"}; !reflect.DeepEqual(got, want) {
		t.Errorf("pieces %v, want %v", got, want)
	}
	if p := plan.Renditions[0].Pieces; p[3].Run != 0 || p[4].Run != 1 || p[4].Init.URL != "https://cdn.test/b.mp4" {
		t.Errorf("runs %d, %d, header %+v", p[3].Run, p[4].Run, p[4].Init)
	}
	// Nothing in MP4 to save, and what isn't a playlist.
	if _, err := ReadDASH([]byte(mpdOf(`type="static" mediaPresentationDuration="PT2S"`, `<AdaptationSet contentType="video"><Representation id="w" mimeType="video/webm" bandwidth="1" height="90"><SegmentTemplate duration="1" media="$Number$.webm"/></Representation></AdaptationSet>`)), "https://cdn.test/m.mpd"); !errors.Is(err, ErrEmpty) {
		t.Errorf("only WebM: %v", err)
	}
	if _, err := ReadDASH([]byte("<html><body>MPD</body></html>"), "https://cdn.test/m.mpd"); !errors.Is(err, ErrFormat) {
		t.Errorf("a page: %v", err)
	}
}

func TestWhatIsADASHPlaylist(t *testing.T) {
	for _, c := range []struct {
		mime, address string
		want          Kind
	}{
		{"application/dash+xml", "https://a.test/v", DASH},
		{"Application/DASH+XML; charset=utf-8", "https://a.test/v", DASH},
		{"text/xml", "https://a.test/v/manifest.mpd?token=1#pb-download", DASH},
		{"application/octet-stream", "https://a.test/v/MANIFEST.MPD", DASH},
		{"text/xml", "https://a.test/feed.xml", None},
		{"application/vnd.apple.mpegurl", "https://a.test/v.mpd", HLS},
	} {
		if got := KindOf(c.mime, c.address); got != c.want {
			t.Errorf("%q %q: %q", c.mime, c.address, got)
		}
	}
	for body, want := range map[string]Kind{
		mpdHead + `type="static">`:                       DASH,
		"\xef\xbb\xbf<?xml version=\"1.0\"?>\n<MPD>":     DASH,
		"<MPD xmlns=\"urn:mpeg:dash:schema:mpd:2011\">":  DASH,
		"<?xml version=\"1.0\"?><rss><title>MPD</title>": None,
		"<MPDX>": None,
		"<?xml version=\"1.0\"?><!-- <MPD> --><svg></svg>": None,
		"<?xml version=\"1.0\"":                            None,
	} {
		if got := Sniff([]byte(body)); got != want {
			t.Errorf("%q: %q", body, got)
		}
	}
	if NameOf("manifest.mpd") != "manifest.mp4" {
		t.Error(NameOf("manifest.mpd"))
	}
}
