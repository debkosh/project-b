//go:build js

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall/js"

	"projectb/app"
	"projectb/engine"
	"projectb/links"
	"projectb/mockengine"
)

// demoEngine is the mock engine as the app sees it in the demo: each thing the app asks
// of the engine that a visitor would notice goes to the stage's log as it is asked, with
// what the stand-in did about it. Everything else is the mock's own.
type demoEngine struct {
	*mockengine.Engine
	say func(string)
}

func (d *demoEngine) LoadURL(tab engine.TabID, u string) {
	if _, ours := d.OurPage(u); !ours && u != links.Home && u != "" && u != "about:blank" {
		d.say("load " + u + ": the renderer is a stand-in here, so Project B's own page shows instead")
	}
	d.Engine.LoadURL(tab, u)
}

func (d *demoEngine) GoBack(tab engine.TabID)    { d.say("back"); d.Engine.GoBack(tab) }
func (d *demoEngine) GoForward(tab engine.TabID) { d.say("forward"); d.Engine.GoForward(tab) }
func (d *demoEngine) Reload(tab engine.TabID)    { d.say("reload"); d.Engine.Reload(tab) }

func (d *demoEngine) Zoom(tab engine.TabID, f float64) {
	d.say(fmt.Sprintf("zoom the page to %.0f%%", f*100))
	d.Engine.Zoom(tab, f)
}

var edits = map[engine.EditCommand]string{engine.EditUndo: "undo", engine.EditRedo: "redo", engine.EditCut: "cut", engine.EditCopy: "copy", engine.EditPaste: "paste", engine.EditSelectAll: "select all"}

func (d *demoEngine) Edit(c engine.EditCommand) {
	if n, ok := edits[c]; ok {
		d.say(n + " in the page")
	}
	d.Engine.Edit(c)
}

func (d *demoEngine) WriteClipboard(text string) {
	d.say("copy to the clipboard: " + clip(text))
	d.Engine.WriteClipboard(text)
}

func (d *demoEngine) DownloadURL(tab engine.TabID, u, referrer string, saveAs bool) {
	d.say("download " + u + ": the renderer is a stand-in here, so nothing is fetched")
	d.Engine.DownloadURL(tab, u, referrer, saveAs)
}

func (d *demoEngine) Find(tab engine.TabID, request int, text string, forward, next bool) {
	if !next {
		d.say("find \"" + clip(text) + "\" in the page")
	}
	d.Engine.Find(tab, request, text, forward, next)
}

func (d *demoEngine) Print(tab engine.TabID) { d.say("print: no printer here"); d.Engine.Print(tab) }

func (d *demoEngine) CreateWindow(w, near engine.WindowID, p engine.Page, l engine.WindowLayout) {
	if w > 1 {
		d.say(fmt.Sprintf("open window %d", w))
	}
	d.Engine.CreateWindow(w, near, p, l)
}

var requests = map[engine.WindowRequest]string{engine.WindowClose: "close", engine.WindowMinimise: "minimise", engine.WindowZoom: "zoom", engine.WindowFullScreen: "full screen", engine.WindowFront: "bring to the front"}

func (d *demoEngine) RequestWindow(w engine.WindowID, r engine.WindowRequest) {
	d.say(fmt.Sprintf("%s window %d", requests[r], w))
	d.Engine.RequestWindow(w, r)
}

var appCommands = map[engine.AppCommand]string{
	engine.AppAbout:           "show the About panel: that is the OS's, not here",
	engine.AppHide:            "hide Project B: that is the OS's, not here",
	engine.AppHideOthers:      "hide other apps: that is the OS's, not here",
	engine.AppShowAll:         "show all apps: that is the OS's, not here",
	engine.AppBringAllToFront: "bring every window to the front",
	engine.AppQuit:            "quit Project B",
}

func (d *demoEngine) AppCommand(c engine.AppCommand) {
	d.say(appCommands[c])
	d.Engine.AppCommand(c)
}

func (d *demoEngine) ShareText(w engine.WindowID, text string) {
	d.say("share " + clip(text) + ": no share sheet here")
	d.Engine.ShareText(w, text)
}

// clip is text short enough for one log line.
func clip(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 60 {
		return s[:57] + "…"
	}
	return s
}

// stageCalls are what the stage around the window asks of the core: the app's menu bar,
// the windows and what they report, and the app's own actions, as the OS's menu bar and
// Dock reach them. Each that needs the app's main thread answers through a callback.
func stageCalls(core js.Value, e *mockengine.Engine, a func() *app.App) {
	reply := func(cb js.Value, v any) {
		b, _ := json.Marshal(v)
		cb.Invoke(string(b))
	}
	// menu() is the menu bar the app gave, as JSON.
	core.Set("menu", js.FuncOf(func(js.Value, []js.Value) any {
		b, _ := json.Marshal(e.Menu())
		return string(b)
	}))
	// menuList(name, cb) gives cb the items of a menu list as they are now.
	core.Set("menuList", js.FuncOf(func(_ js.Value, args []js.Value) any {
		name, cb := args[0].String(), args[1]
		e.Post(func() { go reply(cb, a().MenuList(name)) })
		return nil
	}))
	// choose(action) is a menu bar item chosen, as the OS's menu bar tells the app.
	core.Set("choose", js.FuncOf(func(_ js.Value, args []js.Value) any {
		action := args[0].String()
		e.Post(func() { a().MenuChosen(action) })
		return nil
	}))
	// windows() is each open window and what it last reported, and whether the app quit.
	core.Set("windows", js.FuncOf(func(js.Value, []js.Value) any {
		type win struct {
			ID        engine.WindowID `json:"id"`
			Full      bool            `json:"full"`
			Minimised bool            `json:"minimised"`
			Zoomed    bool            `json:"zoomed"`
		}
		out := struct {
			Windows []win `json:"windows"`
			Quit    bool  `json:"quit"`
		}{Windows: []win{}, Quit: e.Quit()}
		for _, w := range e.Windows() {
			s := e.Reported(w)
			out.Windows = append(out.Windows, win{w, s.FullScreen, s.Minimised, s.Zoomed})
		}
		b, _ := json.Marshal(out)
		return string(b)
	}))
	// mode(w, m) is what the OS does to window w from outside the app: its own yellow and
	// green buttons, and a click on the Dock icon that shows a minimised window again,
	// each reported to the app as the real window reports it. m is "normal", "minimised"
	// or "zoomed".
	modes := map[string]engine.WindowMode{"normal": engine.ModeNormal, "minimised": engine.ModeMinimised, "zoomed": engine.ModeZoomed}
	core.Set("mode", js.FuncOf(func(_ js.Value, args []js.Value) any {
		w := engine.WindowID(args[0].Int())
		m, ok := modes[args[1].String()]
		if !ok {
			return nil
		}
		e.Post(func() {
			if e.Open(w) {
				e.SetWindowMode(w, m)
			}
		})
		return nil
	}))
}
