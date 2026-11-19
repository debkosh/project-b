package mockengine

import (
	"bytes"
	pic "image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"projectb/engine"
)

// SavedImage is one SaveImage asked of the mock: the image's address, where it went and
// its largest side asked for.
type SavedImage struct {
	URL     string `json:"url"`
	Path    string `json:"path"`
	MaxSize int    `json:"maxSize"`
}

// ShareText keeps the text, as a share sheet would get it.
func (e *Engine) ShareText(w engine.WindowID, text string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sharedText = text
}

// SaveImage writes a small PNG at path, as the engine writes the image it fetched, and says
// so; an address that isn't http(s) or data fails, as one the page can't load does.
func (e *Engine) SaveImage(tab engine.TabID, url, path string, maxSize int) {
	e.mu.Lock()
	e.savedImages = append(e.savedImages, SavedImage{URL: url, Path: path, MaxSize: maxSize})
	e.mu.Unlock()
	ok := false
	if strings.HasPrefix(url, "http:") || strings.HasPrefix(url, "https:") || strings.HasPrefix(url, "data:") {
		img := pic.NewRGBA(pic.Rect(0, 0, 4, 3))
		for x := range 4 {
			for y := range 3 {
				img.Set(x, y, color.RGBA{R: uint8(60 * x), G: 120, B: uint8(80 * y), A: 255})
			}
		}
		var b bytes.Buffer
		png.Encode(&b, img)
		ok = os.MkdirAll(filepath.Dir(path), 0o755) == nil && os.WriteFile(path, b.Bytes(), 0o644) == nil
	}
	e.Post(func() { e.listener.ImageSaved(path, ok) })
}

// CopyImage keeps the file, as the clipboard would hold it.
func (e *Engine) CopyImage(path string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.copiedImage = path
}

// SharedText is the text ShareText last offered, or "".
func (e *Engine) SharedText() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sharedText
}

// CopiedImage is the file CopyImage last put on the clipboard, or "".
func (e *Engine) CopiedImage() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.copiedImage
}

// SavedImages are the SaveImage calls so far, in order.
func (e *Engine) SavedImages() []SavedImage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]SavedImage{}, e.savedImages...)
}
