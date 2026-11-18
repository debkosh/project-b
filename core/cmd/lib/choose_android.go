package main

import "projectb/engine"

// ChooseFolder does nothing on a phone, which imports nothing from other apps.
func (e *nativeEngine) ChooseFolder(engine.WindowID) {}
