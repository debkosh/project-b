// Package streams saves a streamed video as one MP4 (docs/video-streams.md). A streaming
// site sends no file: it cuts the video into pieces of a few seconds and lists them in a
// playlist. A download that turns out to be a playlist is read here instead of kept: the
// best quality is chosen, its pieces are fetched as the page would fetch them, unlocked
// when they are locked with AES-128, and repacked, with no re-encoding, into a regular MP4
// with its index at the front.
//
// The parts: a playlist is read into a Plan, the pieces to fetch for each track (hls.go,
// and dash.go for DASH's);
// a Job fetches them, a few at a time, through the engine's own network (job.go, core/fetch);
// each piece, an MPEG-TS file or a fragment of MP4, is added to one working file of MP4
// fragments (ts.go, work.go); and at the end that file is made into the MP4 (work.go).
// Playlists are read with Eyevinn's hls-m3u8 and dash-mpd, MPEG-TS with bluenviron's
// mediacommon, and MP4 is read and written with Eyevinn's mp4ff.
package streams

import (
	"bytes"
	"errors"
	"net/url"
	"path"
	"strings"
)

// Why a stream can't be saved. Its download's row says each in the shells' words, by its
// reason (Reason).
var (
	// ErrLive: the playlist has no end yet.
	ErrLive = errors.New("live videos can't be saved yet")
	// ErrProtected: its pieces are locked by anything other than plain AES-128 with a key the
	// playlist names: FairPlay, Widevine, PlayReady. Never saved.
	ErrProtected = errors.New("this video is protected")
	// ErrFormat: it is a stream, but of pieces the app can't repack.
	ErrFormat = errors.New("this video's format can't be saved yet")
	// ErrRefused: a piece, a key or a playlist is at an address that may not be fetched: not
	// http or https, or on this computer or its network for a playlist from the internet.
	ErrRefused = errors.New("part of this video is at an address that can't be fetched")
	// ErrEmpty: the playlist lists nothing to save.
	ErrEmpty = errors.New("this video has nothing in it to save")
)

// FetchError is a piece, a key or a playlist that couldn't be fetched: Reason is why, as a
// download's is, "connection" or "server".
type FetchError struct {
	Reason string
	URL    string
}

func (e *FetchError) Error() string { return "couldn't fetch " + e.URL + ": " + e.Reason }

// hlsTypes are the types a server gives an HLS playlist.
var hlsTypes = map[string]bool{
	"application/vnd.apple.mpegurl": true,
	"application/x-mpegurl":         true,
	"audio/mpegurl":                 true,
	"audio/x-mpegurl":               true,
}

// Kind is which kind of playlist a stream has.
type Kind string

const (
	// None is no stream at all.
	None Kind = ""
	// HLS is a playlist of HTTP Live Streaming, a text file starting #EXTM3U.
	HLS Kind = "hls"
	// DASH is a playlist of MPEG-DASH, an XML file whose root is MPD.
	DASH Kind = "dash"
)

// KindOf says whether a download is a stream's playlist, from its type and its address
// alone, as known before any of it has arrived: an HLS playlist's type or an address ending
// in .m3u8, or DASH's type, application/dash+xml, or an address ending in .mpd.
func KindOf(mime, address string) Kind {
	mime = strings.ToLower(strings.TrimSpace(strings.SplitN(mime, ";", 2)[0]))
	if hlsTypes[mime] {
		return HLS
	}
	if mime == "application/dash+xml" {
		return DASH
	}
	if u, err := url.Parse(address); err == nil {
		switch strings.ToLower(path.Ext(u.Path)) {
		case ".m3u8":
			return HLS
		case ".mpd":
			return DASH
		}
	}
	return None
}

// Sniff says whether a file's first bytes are a playlist's, for one whose type and address
// didn't say: #EXTM3U, after any byte order mark, or XML whose root element is MPD, after
// its declaration and any comments.
func Sniff(head []byte) Kind {
	head = bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))
	if bytes.HasPrefix(head, []byte("#EXTM3U")) {
		return HLS
	}
	for {
		head = bytes.TrimLeft(head, " \t\r\n")
		var end string
		switch {
		case bytes.HasPrefix(head, []byte("<?")):
			end = "?>"
		case bytes.HasPrefix(head, []byte("<!--")):
			end = "-->"
		default:
			if rest, ok := bytes.CutPrefix(head, []byte("<MPD")); ok && len(rest) > 0 && bytes.IndexByte([]byte(" \t\r\n>"), rest[0]) >= 0 {
				return DASH
			}
			return None
		}
		at := bytes.Index(head, []byte(end))
		if at < 0 {
			return None
		}
		head = head[at+len(end):]
	}
}

// SniffLength is how many of a file's first bytes Sniff needs.
const SniffLength = 1024

// Read reads a playlist of kind fetched from the address base, for the quality closest to
// height, or with 0 the best.
func Read(kind Kind, body []byte, base string, height int) (*Plan, error) {
	if kind == DASH {
		return ReadDASH(body, base, height)
	}
	return ReadHLS(body, base, height)
}

// NameOf is the name a stream's MP4 goes by: the playlist's own, as its server or its
// address gave it, with .mp4 in place of its ending.
func NameOf(playlist string) string {
	name := strings.TrimSpace(playlist)
	if ext := path.Ext(name); ext != "" && len(ext) <= 6 {
		name = strings.TrimSuffix(name, ext)
	}
	if name == "" {
		name = "video"
	}
	return name + ".mp4"
}
