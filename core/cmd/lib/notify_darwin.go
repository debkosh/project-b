package main

// A Mac's notifications are the app's own to show: there is nothing to ask.
func (e *nativeEngine) AskNotifications() {}

func (e *nativeEngine) NotificationsAllowed() bool { return true }
