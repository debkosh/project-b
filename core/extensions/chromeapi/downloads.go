//go:build extensions

package chromeapi

import (
	"errors"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
)

// chrome.downloads (docs/extensions.md): download, answered from Go's own downloads, so an
// extension's download is a row in Downloads like any other, and a playlist's address gets
// the stream's MP4 (docs/video-streams.md). The rest of the API isn't here yet.

// Downloader starts a download as the app does.
type Downloader interface {
	// Download starts a download of address as the browser's own, to name under the
	// Downloads folder ("" for the server's own name), through the Save panel when saveAs;
	// conflict is Chrome's FilenameConflictAction. It returns the download's id in Go's
	// list, or why the address may not be downloaded.
	Download(address, name string, saveAs bool, conflict string) (int, error)
}

// SetDownloader is how the API starts downloads. Set as the app starts.
func (a *API) SetDownloader(d Downloader) { a.downloads = d }

// Chrome's own words for a call it refuses.
var (
	errInvalidURL      = errors.New("Invalid URL")
	errInvalidFilename = errors.New("Invalid filename")
)

func init() {
	functions["downloads.download"] = func(a *API, c call) (any, error) {
		var o struct {
			URL            string  `json:"url"`
			Filename       *string `json:"filename"`
			SaveAs         bool    `json:"saveAs"`
			ConflictAction string  `json:"conflictAction"`
			Method         string  `json:"method"`
			Headers        []any   `json:"headers"`
			Body           *string `json:"body"`
		}
		if !c.arg(0, &o) {
			return nil, errInvalidURL
		}
		if a.downloads == nil || !slices.Contains(a.caller(c).Permissions, "downloads") {
			return nil, errors.New("downloads.download needs the \"downloads\" permission")
		}
		u, err := url.Parse(o.URL)
		if err != nil || u.Host == "" || u.Scheme != "http" && u.Scheme != "https" {
			return nil, errInvalidURL
		}
		// As a plain request for the address: no other method, headers or body yet.
		if o.Method != "" && o.Method != "GET" || len(o.Headers) > 0 || o.Body != nil {
			return nil, errors.New("downloads.download with a method, headers or a body is not available in Project B")
		}
		name := ""
		if o.Filename != nil {
			// A path under the Downloads folder, as Chrome takes it: not empty, not absolute,
			// with no way back out.
			name = filepath.FromSlash(*o.Filename)
			if name == "" || filepath.IsAbs(name) || strings.HasPrefix(*o.Filename, "/") || strings.HasSuffix(*o.Filename, "/") || !filepath.IsLocal(name) ||
				slices.Contains(strings.Split(*o.Filename, "/"), "..") {
				return nil, errInvalidFilename
			}
		}
		switch o.ConflictAction {
		case "":
			o.ConflictAction = "uniquify"
		case "uniquify", "overwrite", "prompt":
		default:
			return nil, errors.New("Invalid conflictAction")
		}
		return a.downloads.Download(o.URL, name, o.SaveAs, o.ConflictAction)
	}
}
