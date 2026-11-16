package app

import "projectb/engine"

// Thumbnail is a new picture of a tab's page, which the tab's state points to.
func (a *App) Thumbnail(tab engine.TabID, revision int64) { a.Tabs.HandleThumbnail(tab, revision) }
