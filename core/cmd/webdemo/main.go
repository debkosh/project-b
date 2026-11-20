//go:build js

// Command webdemo runs the real managers on the mock engine inside a web page, as
// WebAssembly, with no server: what core/cmd/dev serves over HTTP it answers in the page,
// on globalThis.pbcore, and web/shim.js sends each page's requests for those addresses
// there. The OS window page, the mock pages and the webui.js our pages get are dev's own.
//
//	pbcore.post(path, query, body)   a POST to one of dev's addresses; returns nothing
//	pbcore.stream(path, query, fn)   one of dev's event streams: fn gets each message;
//	                                 returns a function that stops it
//	pbcore.thumb(tab)                a tab's picture, SVG text, or "" for none
//
// Paths are dev's without the leading slash, such as "shell/send"; query is the address's
// query string, which names the window or the tab.
package main

import (
	"net/url"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"projectb/app"
	"projectb/commands"
	"projectb/engine"
	"projectb/mockengine"
	"projectb/mocksystem"
	"projectb/profiles"
	"projectb/shells"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/tones"
	"projectb/update"
)

const profile = "/profile/dev"

// home is the page each window opens on: the homepage, which every page the demo loads
// shows, as there is no web engine to load another.
const home = "https://projectb.debkosh.com/"

func main() {
	// The app is the one for the visitor's OS, as the page says before the core starts
	// (pbcoreOS): a phone's on a phone, the Mac's keys on a Mac, Linux's on any other.
	switch js.Global().Get("pbcoreOS").String() {
	case "android":
		commands.OS, shells.OnPhone = "android", true
	case "mac":
		commands.OS = "darwin"
	default:
		commands.OS = "linux"
	}
	e := mockengine.New()
	e.InlineThumbnails = true
	sys := mocksystem.New(e.Post)
	// What the app asks of the engine goes to the stage's log, pbcoreLog, in order and
	// never holding up the main thread.
	said := make(chan string, 256)
	go func() {
		for line := range said {
			if f := js.Global().Get("pbcoreLog"); f.Type() == js.TypeFunction {
				f.Invoke(line)
			}
		}
	}()
	d := &demoEngine{Engine: e, say: func(s string) {
		select {
		case said <- s:
		default:
		}
	}}
	list := profiles.New("/profile")
	var a *app.App
	started := make(chan struct{})
	e.Post(func() {
		a = app.Start(d, app.Config{
			ProfileDir:  profile,
			Profiles:    list,
			SkipWelcome: true,
			URLs:        []string{home},
			System:      sys,
			Update:      update.Config{Project: "project-b", StateDir: profile + "/updates"},
		})
		a.UsePasswordStore(nil)
		// The first workspace takes the logo's teal, as the profile does, so a shell that
		// shows the workspace's colour (Glide's sidebar) stays in the brand's.
		if ws := a.Workspaces.List(); len(ws) > 0 {
			a.Workspaces.SetColor(ws[0].ID, tones.Brand)
		}
		e.SetListener(a)
		close(started)
	})
	go e.Run()
	<-started

	pages := map[string]bool{}
	for _, p := range tabs.Pages {
		pages[p] = true
	}

	post := func(path, query, body string) {
		q, _ := url.ParseQuery(query)
		win := windowOf(q)
		switch path {
		case "mock/window/size":
			f, _ := url.ParseQuery(body)
			w, err1 := strconv.ParseFloat(f.Get("w"), 64)
			h, err2 := strconv.ParseFloat(f.Get("h"), 64)
			if err1 == nil && err2 == nil {
				e.Resize(win, w, h)
			}
		case "mock/origin":
			f, _ := url.ParseQuery(body)
			if ms, err := strconv.ParseFloat(f.Get("t"), 64); err == nil {
				timeline.PageOrigin(f.Get("page"), timeline.FromWall(time.UnixMicro(int64(ms*1000))))
			}
		case "mock/fire":
			f, _ := url.ParseQuery(body)
			tab, _ := strconv.ParseInt(f.Get("tab"), 10, 64)
			e.Fire(win, engine.TabID(tab), f.Get("kind"), f.Get("value"))
		case "shell/send":
			msg := []byte(body)
			e.Post(func() {
				if e.Open(win) && e.PageOf(win).Shell() != "" {
					a.ShellMessage(win, msg)
				}
			})
		default:
			page, ok := strings.CutSuffix(path, "/send")
			if !ok || !pages[page] {
				js.Global().Get("console").Call("warn", "pbcore: nothing answers", path)
				return
			}
			tab := tabOf(q)
			msg := []byte(body)
			e.Post(func() {
				if e.Internal(tab) {
					a.PageMessage(tab, engine.Page(page), msg)
				}
			})
		}
	}

	stream := func(path, query string, fn js.Value) js.Value {
		q, _ := url.ParseQuery(query)
		win := windowOf(q)
		var f *mockengine.Feed
		var now func() []byte
		switch path {
		case "shell/events":
			f = &e.Feeds(win).Shell
		case "mock/events":
			slot, _ := strconv.Atoi(q.Get("slot"))
			f, now = &e.Feeds(win).Page, func() []byte { return e.PageState(win, slot) }
		case "mock/window/events":
			f, now = &e.Feeds(win).Window, func() []byte { return e.WindowState(win) }
		default:
			page, ok := strings.CutSuffix(path, "/events")
			if !ok || !pages[page] {
				js.Global().Get("console").Call("warn", "pbcore: no stream", path)
				return js.Undefined()
			}
			f = e.PageFeed(tabOf(q))
		}
		msgs, stop := f.Subscribe()
		done := make(chan struct{})
		go func() {
			if now != nil {
				if b := now(); b != nil {
					fn.Invoke(string(b))
				}
			}
			for {
				select {
				case <-done:
					return
				case m := <-msgs:
					fn.Invoke(string(m))
				}
			}
		}()
		var cancel js.Func
		cancel = js.FuncOf(func(js.Value, []js.Value) any {
			stop()
			close(done)
			cancel.Release()
			return nil
		})
		return cancel.Value
	}

	core := js.Global().Get("Object").New()
	core.Set("post", js.FuncOf(func(_ js.Value, args []js.Value) any {
		post(args[0].String(), args[1].String(), args[2].String())
		return nil
	}))
	core.Set("stream", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return stream(args[0].String(), args[1].String(), args[2])
	}))
	core.Set("thumb", js.FuncOf(func(_ js.Value, args []js.Value) any {
		b, ok := e.ThumbnailImage(engine.TabID(args[0].Int()))
		if !ok {
			return ""
		}
		return string(b)
	}))
	stageCalls(core, e, func() *app.App { return a })
	js.Global().Set("pbcore", core)
	if ready := js.Global().Get("pbcoreReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}

// windowOf is the window an address is for, 1 when it names none.
func windowOf(q url.Values) engine.WindowID {
	if w, err := strconv.ParseInt(q.Get("window"), 10, 64); err == nil {
		return engine.WindowID(w)
	}
	return 1
}

// tabOf is the tab an address for one of our pages is for.
func tabOf(q url.Values) engine.TabID {
	t, _ := strconv.ParseInt(q.Get("tab"), 10, 64)
	return engine.TabID(t)
}
