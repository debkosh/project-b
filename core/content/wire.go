package content

import (
	"projectb/engine"
)

// Wire gives the listener, the picker and then the app, the adapter's events that are
// still decoded here. It subscribes to no hook: what a tab's page does reaches the app as events of the adapter's own, which
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
	h.Events.OnPickerMessage = l.PickerMessage
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
	h.Events.OnPermission = l.PermissionRequested
	h.Events.OnPermissionOf = l.PermissionStatusOf
	h.Events.OnNotification = l.NotificationShown
	h.Events.OnNotifyClicked = l.NotificationClicked
	h.Events.OnConnection = l.ConnectionChanged
	h.Events.OnCertificate = l.CertificateError
}
