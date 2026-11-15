package content

import (
	"projectb/engine"
)

// Picker is the engine's listener in a copy that hasn't chosen its profile: the picker's
// window, its page and the menu. Its keys reach it through core/abi.
type Picker interface {
	Message(w engine.WindowID, msg []byte)
	WindowChanged(w engine.WindowID, s engine.WindowState)
	WindowClosed(w engine.WindowID)
	WindowFocused(w engine.WindowID)
	ShellCrashed(w engine.WindowID)
	MenuChosen(action string)
	MenuList(name string) engine.Menu
	OSBindings(byTitle map[string]engine.Key)
	ModsChanged(m engine.Mods)
}

// WirePicker gives the picker what it hears, until the choice: Wire then gives each of
// these to the app.
func WirePicker(h *Hooks, p Picker) {
	h.Events.OnPickerMessage = p.Message
	h.Events.OnWindowChanged = p.WindowChanged
	h.Events.OnWindowClosed = p.WindowClosed
	h.Events.OnWindowFocused = p.WindowFocused
	h.Events.OnShellCrashed = p.ShellCrashed
	h.Events.OnMenuChosen = p.MenuChosen
	h.Events.OnMenuList = p.MenuList
	h.Events.OnOSBindings = p.OSBindings
	h.Events.OnModsChanged = p.ModsChanged
}

// Wire gives the app the adapter's events that are still decoded here. It subscribes to
// no hook: what a tab's page does reaches the app as events of the adapter's own, which
// core/abi decodes, so the app never depends on what content calls a thing.
func Wire(h *Hooks, l engine.Listener) {
	h.Events.OnShellMessage = l.ShellMessage
	h.Events.OnTabOpened = l.TabOpened
	h.Events.OnFaviconChanged = l.FaviconChanged
	h.Events.OnWindowChanged = l.WindowChanged
	h.Events.OnShellCrashed = l.ShellCrashed
	h.Events.OnWindowClosed = l.WindowClosed
	h.Events.OnWindowFocused = l.WindowFocused
	h.Events.OnSettingsMessage = l.SettingsMessage
	h.Events.OnMenuChosen = l.MenuChosen
	h.Events.OnMenuList = l.MenuList
	h.Events.OnOSBindings = l.OSBindings
	h.Events.OnModsChanged = l.ModsChanged
	h.Events.OnProcessCrashed = l.ProcessCrashed
	h.Events.OnDevTools = l.DevToolsCommand
	h.Events.OnDownloadTarget = l.DownloadTarget
	h.Events.OnDownloadUpdated = l.DownloadUpdated
	h.Events.OnFileChooser = l.FileChooser
	h.Events.OnFilesChosen = l.FilesChosen
	h.Events.OnDialogOpened = l.DialogOpened
	h.Events.OnDialogClosed = l.DialogClosed
}
