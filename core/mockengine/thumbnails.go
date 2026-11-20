package mockengine

import (
	"encoding/base64"
	"fmt"
	"html"

	"projectb/engine"
)

// Capture is one picture the app asked for: the tab, and how wide it may be.
type Capture struct {
	Tab      engine.TabID
	MaxWidth int
}

// thumbnail is a tab's latest picture, as the real engine keeps one: its revision and the
// image the dev server serves.
type thumbnail struct {
	revision int64
	svg      []byte
}

// CaptureThumbnail draws a picture of what the tab's mock page shows, its address on a
// colour of its own, and keeps it as the tab's, telling the app a little later, as the
// real engine does once its copy is made. A tab that closed meanwhile gets none.
func (e *Engine) CaptureThumbnail(id engine.TabID, maxWidth int) {
	e.mu.Lock()
	e.captures = append(e.captures, Capture{id, maxWidth})
	e.mu.Unlock()
	t := e.tabs[id]
	if t == nil {
		return
	}
	svg := thumbnailSVG(id, t.current)
	e.Post(func() {
		if e.tabs[id] == nil || e.listener == nil {
			return
		}
		e.mu.Lock()
		if e.thumbs == nil {
			e.thumbs = map[engine.TabID]thumbnail{}
		}
		th := thumbnail{revision: e.thumbs[id].revision + 1, svg: svg}
		e.thumbs[id] = th
		e.mu.Unlock()
		e.listener.Thumbnail(id, th.revision)
	})
}

// ThumbnailURL is where the dev server serves a tab's picture (core/cmd/dev), or with
// InlineThumbnails the picture itself, as a data: URL.
func (e *Engine) ThumbnailURL(tab engine.TabID, revision int64) string {
	if e.InlineThumbnails {
		if svg, ok := e.ThumbnailImage(tab); ok {
			return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(svg)
		}
	}
	return fmt.Sprintf("/mock/thumb/%d?v=%d", tab, revision)
}

// ThumbnailImage is a tab's latest picture, an SVG, or false for none. For the dev server.
func (e *Engine) ThumbnailImage(tab engine.TabID) ([]byte, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	th, ok := e.thumbs[tab]
	return th.svg, ok
}

// Captures are the pictures the app asked for, in order.
func (e *Engine) Captures() []Capture {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]Capture(nil), e.captures...)
}

// dropThumbnail lets a closed tab's picture go, as the real engine's goes with its tab.
func (e *Engine) dropThumbnail(id engine.TabID) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.thumbs, id)
}

// thumbnailSVG is a phone-shaped picture with the page's address at the top.
func thumbnailSVG(id engine.TabID, address string) []byte {
	if address == "" {
		address = "New tab"
	}
	hue := (int(id) * 67) % 360
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" width="360" height="640" viewBox="0 0 360 640">`+
		`<rect width="360" height="640" fill="hsl(%d 60%% 92%%)"/>`+
		`<rect width="360" height="72" fill="hsl(%d 55%% 45%%)"/>`+
		`<text x="16" y="46" font-family="sans-serif" font-size="20" fill="#fff">%s</text>`+
		`<text x="16" y="120" font-family="sans-serif" font-size="16" fill="#333">Mock page, tab %d</text>`+
		`</svg>`, hue, hue, html.EscapeString(address), id)
}
