package main

import (
	"embed"
	"encoding/json"
	"net/http"

	"projectb/app"
	"projectb/engine"
	"projectb/mockengine"
)

// The mock OS page, /mock/os: what the OS draws around the app, with the window page
// (window.html) in a frame for each window. It starts with the menu bar the app sets; the
// OS's other parts (notifications, the person check, the default-browser question, the
// dialogs) are each one more part there (os.js).
//
//go:embed os.html os.js icon.svg icon-small.svg
var osPages embed.FS

// osRoutes are the OS page and what it reads and does: the app's menu and the items of
// a menu list, a menu item chosen, the windows as each last reported, and the OS putting
// a window in a mode, as its own buttons and the Dock do. app is the app, nil in a picker
// copy.
func osRoutes(mux *http.ServeMux, e *mockengine.Engine, app func() *app.App) {
	file := func(name, contentType string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, _ := osPages.ReadFile(name)
			w.Header().Set("Content-Type", contentType)
			w.Write(b)
		}
	}
	answer := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
	onMain := func(fn func()) {
		done := make(chan struct{})
		e.Post(func() { fn(); close(done) })
		<-done
	}
	mux.HandleFunc("GET /mock/os", file("os.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /mock/os.js", file("os.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /mock/icon.svg", file("icon.svg", "image/svg+xml"))
	mux.HandleFunc("GET /mock/icon-small.svg", file("icon-small.svg", "image/svg+xml"))
	mux.HandleFunc("GET /mock/menu", func(w http.ResponseWriter, r *http.Request) { answer(w, e.Menu()) })
	mux.HandleFunc("GET /mock/menulist", func(w http.ResponseWriter, r *http.Request) {
		var items engine.Menu
		onMain(func() {
			if a := app(); a != nil {
				items = a.MenuList(r.URL.Query().Get("name"))
			}
		})
		answer(w, items)
	})
	mux.HandleFunc("POST /mock/choose", func(w http.ResponseWriter, r *http.Request) {
		action := r.FormValue("action")
		e.Post(func() {
			if a := app(); a != nil {
				a.MenuChosen(action)
			}
		})
	})
	mux.HandleFunc("GET /mock/windows", func(w http.ResponseWriter, r *http.Request) { answer(w, OSWindows(e)) })
	mux.HandleFunc("POST /mock/mode", func(w http.ResponseWriter, r *http.Request) {
		m, ok := osModes[r.FormValue("mode")]
		if !ok {
			http.Error(w, "mode is normal, minimised or zoomed", http.StatusBadRequest)
			return
		}
		win := windowOf(r)
		e.Post(func() {
			if e.Open(win) {
				e.SetWindowMode(win, m)
			}
		})
	})
}

var osModes = map[string]engine.WindowMode{"normal": engine.ModeNormal, "minimised": engine.ModeMinimised, "zoomed": engine.ModeZoomed}

// OSWindow is one open window as /mock/windows has it.
type OSWindow struct {
	ID        engine.WindowID `json:"id"`
	Full      bool            `json:"full"`
	Minimised bool            `json:"minimised"`
	Zoomed    bool            `json:"zoomed"`
}

// OSWindows is every open window, as each last reported, and whether the app quit.
func OSWindows(e *mockengine.Engine) any {
	out := struct {
		Windows []OSWindow `json:"windows"`
		Quit    bool       `json:"quit"`
	}{Windows: []OSWindow{}, Quit: e.Quit()}
	for _, w := range e.Windows() {
		s := e.Reported(w)
		out.Windows = append(out.Windows, OSWindow{w, s.FullScreen, s.Minimised, s.Zoomed})
	}
	return out
}
