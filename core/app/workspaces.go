package app

import (
	"encoding/json"
	"maps"
	"slices"

	"projectb/commands"
	"projectb/engine"
	"projectb/settings"
	"projectb/shell"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/tones"
	"projectb/workspaces"
)

// workspaceList is the list in the Tab menu's New Tab in Workspace: every workspace, by
// name. Choosing one opens a new tab in it, in the window in front.
func (a *App) workspaceList() commands.List {
	return commands.List{
		ID: "workspaces",
		Items: func() []commands.ListItem {
			var items []commands.ListItem
			for _, w := range a.Workspaces.List() {
				items = append(items, commands.ListItem{Key: w.ID, Title: w.Name})
			}
			return items
		},
		Choose: func(id string) {
			a.Shell.Interaction(a.Tabs.Front(), "tab.new", func() { a.Tabs.CreateIn(a.Tabs.Front(), id) })
		},
	}
}

// workspaceShellPart gives a window's shell the workspaces in its snapshot, for the dot on
// each tab, and takes the same changes to them the settings page does: workspaces are
// Go's, for every shell to use. A shell names the workspace by "workspace", as its newTab
// does, since "id" is a number in a shell's messages.
func (a *App) workspaceShellPart(e engine.Engine) shell.Part {
	return shell.Part{
		Snapshot: func(into map[string]any) { into["workspaces"] = a.workspacesShown() },
		Handle: func(_ engine.WindowID, t string, msg []byte) bool {
			var c struct {
				Workspace string `json:"workspace"`
				Text      string `json:"text"`
				Color     string `json:"color"`
				By        int    `json:"by"`
			}
			json.Unmarshal(msg, &c)
			return a.changeWorkspaces(e, t, workspaceChange{ID: c.Workspace, Text: c.Text, Color: c.Color, By: c.By})
		},
	}
}

// workspaceChange is a change to the list, as the settings page sends it: the workspace by
// its id, a name, a colour, and the places to move it by.
type workspaceChange struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Color string `json:"color"`
	By    int    `json:"by"`
}

// changeWorkspaces makes a change to the list, by the message's t, whoever asks: the
// settings page or a shell, under the same rules. It says whether t was one of its.
func (a *App) changeWorkspaces(e engine.Engine, t string, c workspaceChange) bool {
	var err error
	switch t {
	case "createWorkspace":
		var w workspaces.Workspace
		if w, err = a.Workspaces.Create(c.Text, c.Color); err == nil {
			timeline.Infof("workspace %s: made, its sites in %s", w.ID, workspaceStorage(a.dir, w.ID))
		}
	case "renameWorkspace":
		err = a.Workspaces.SetName(c.ID, c.Text)
	// The name and colour come together. The name goes first, so a name refused changes
	// nothing.
	case "editWorkspace":
		if err = a.Workspaces.SetName(c.ID, c.Text); err == nil {
			err = a.Workspaces.SetColor(c.ID, c.Color)
		}
	case "moveWorkspace":
		err = a.Workspaces.Move(c.ID, c.By)
	case "deleteWorkspace":
		err = a.deleteWorkspace(e, c.ID)
	default:
		return false
	}
	if err != nil {
		timeline.Warnf("workspaces: %v", err)
	}
	return true
}

// workspaceSettingsPart is the settings page's Workspaces section: the list, each with the
// number of web tabs open in it, New workspace with a name and a colour, editing one's name
// and colour, moving one up or down the list, deleting one, and a new tab in one. Every copy
// has it, a copy on a folder named outright too, since the list is the profile folder's.
// The pages hear the list again through its OnChanged, as the shells do, and the counts
// again when a tab opens or closes and they differ.
func (a *App) workspaceSettingsPart(e engine.Engine) settings.Part {
	counts := a.Tabs.CountByWorkspace()
	a.Tabs.OnListChanged(func() {
		if now := a.Tabs.CountByWorkspace(); !maps.Equal(now, counts) {
			counts = now
			a.Settings.Send(map[string]any{"t": "workspaceTabs", "workspaceTabs": counts})
		}
	})
	return settings.Part{
		Snapshot: func(into map[string]any) {
			into["workspaces"] = a.workspacesShown()
			into["workspaceTabs"] = counts
		},
		Handle: func(tab engine.TabID, t string, msg []byte) bool {
			var c workspaceChange
			json.Unmarshal(msg, &c)
			if t != "openInWorkspace" {
				return a.changeWorkspaces(e, t, c)
			}
			// New tab in workspace: a tab in it at the end of the page's own window, made
			// active, as choosing it in the Tab menu does.
			if !slices.ContainsFunc(a.Workspaces.List(), func(x workspaces.Workspace) bool { return x.ID == c.ID }) {
				timeline.Warnf("workspaces: no workspace %s", c.ID)
				return true
			}
			w := a.Tabs.WindowOf(tab)
			a.Shell.Interaction(w, "tab.new", func() { a.Tabs.CreateIn(w, c.ID) })
			return true
		},
	}
}

// deleteWorkspace deletes a workspace. It leaves the list first, which refuses the last one
// and tells everyone the list as it is now; then its tabs close in every window, and its
// pins go; then the engine erases its storage, once those tabs are gone. A window always
// has a tab, and the person closed none, so one that held only its tabs gets a new tab, in
// the first workspace, before they close. Our own tabs are in no workspace: they stay, and
// their new tabs go in the first.
func (a *App) deleteWorkspace(e engine.Engine, id string) error {
	if err := a.Workspaces.Delete(id); err != nil {
		return err
	}
	for _, w := range a.Tabs.Windows() {
		if !slices.ContainsFunc(a.Tabs.AllIn(w), func(t tabs.Tab) bool { return t.Page != "" || t.Workspace != id }) {
			a.Tabs.CreateIn(w, a.Workspaces.First())
		}
	}
	for _, t := range a.Tabs.All() {
		if t.Page == "" && t.Workspace == id {
			a.Tabs.Close(t.ID)
		}
	}
	a.Pins.RemoveWorkspace(id)
	e.EraseWorkspace(id, engine.Erasure{Kinds: engine.EraseAll})
	delete(a.siteData, id)
	timeline.Infof("workspace %s: deleted, and its sites erased from %s", workspaceName(id), workspaceStorage(a.dir, id))
	return nil
}

// shownWorkspace is a workspace as the shells and settings hear of it: with the tones of
// its colour, light and dark, for a shell that paints with a workspace's colour, as Glide
// paints a Space; a hue is filled as a frame first (tones.Fill). The tones are made as it
// is sent, never saved.
type shownWorkspace struct {
	workspaces.Workspace
	Tones *tones.Both `json:"tones"`
}

func (a *App) workspacesShown() []shownWorkspace {
	var out []shownWorkspace
	for _, w := range a.Workspaces.List() {
		out = append(out, shownWorkspace{w, tones.Of(tones.Fill(w.Color))})
	}
	return out
}
