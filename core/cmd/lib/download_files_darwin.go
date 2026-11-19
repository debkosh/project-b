package main

// A Mac's downloads are the user's own files in their own folder: the app doesn't delete
// them.
func (e *nativeEngine) DeleteDownload(string) bool { return false }
