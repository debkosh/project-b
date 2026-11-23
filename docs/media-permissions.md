# A page's camera and microphone, and the OS's own permission

Status: agreed 2026-11-23 ("we should ask them when we need them if possible": the OS is asked
only when a page a site allows first needs the camera or microphone, never at start); built.

**The gap.** A page's camera and microphone (getUserMedia, video calls) can't work on Android:
the manifest declares neither `CAMERA` nor `RECORD_AUDIO`, and nothing asks Android at run
time, so a site Go allows still gets no device. The Mac has the same gap: its Info.plist has no
`NSCameraUsageDescription` or `NSMicrophoneUsageDescription`, without which macOS refuses the
app its camera and microphone.

**How Chrome does it.** On Android, once a site's setting allows the camera, Chrome checks the
app's own runtime permission (`AndroidPermissionRequester`, `PermissionUpdateRequester`); when
missing it asks Android through the tab's window, and when the person refuses, it says so with a
message offering the app's settings. On the Mac, Chrome asks macOS with
`AVCaptureDevice requestAccessForMediaType` (system_media_capture_permissions_mac) and, when
macOS says no, shows that the system blocked it, with a link to System Settings.

**Ours.** Go decides, as every permission (docs/daily-driver.md, Permissions); the OS's question
is an OS capability, so it is oskit's, next to notify's Ask.

1. **oskit/media**: `Allowed(kind) bool` and `Ask(kinds, answered func(map[kind]bool))`, kinds
   `camera` and `microphone`, with `AskWait`.
   - Android: `checkSelfPermission` / `requestPermissions` for `CAMERA` and `RECORD_AUDIO`
     (oskit's Java, its own request code through `OsKit.onRequestPermissionsResult`).
   - Mac: `AVCaptureDevice authorizationStatusForMediaType` / `requestAccessForMediaType`.
   - Linux: allowed (the Flatpak's device access is its manifest's; the Camera portal is
     later, with the Linux session).
   - A fake.
2. **The manifests**: `CAMERA` and `RECORD_AUDIO` in the APK's manifest; the two usage strings
   in the Mac app's Info.plist ("A site you allow can use the camera." / "…the microphone.").
3. **core/sites**: when a request's camera or microphone is granted (kept, or this time), Go
   asks oskit/media before answering the engine: the OS's yes gives the device, its no answers
   denied. The site's own setting stays as the person chose.
4. **On the page** (ux, 2026-11-22): a state of the tab's permission prompt
   (PermissionPrompt.vue, `.ui-permission`), the same floating card in the same place on every
   shell and the phone, not a PageNotice. The kind's glyph (the camera's for both), a title, one
   muted line, and two buttons; no focus taken, Escape dismisses.
   - Title: "Android hasn't let Project B use your camera" / "…your microphone" / "…your camera
     and microphone"; on the Mac, "macOS hasn't let…".
   - Line, Android: "So meet.example can't use it, though you've allowed it. Turn it on in
     Project B's app settings." ("them" for both). Mac: "So meet.example can't use it, though
     you've allowed it. Turn on Project B in System Settings › Privacy & Security › Camera."
     ("Camera and Microphone" for both). The host as the prompt has it (permissionHost).
   - Buttons: "Open settings" (Android's app details; the Mac's Privacy & Security pane at
     Camera, or Microphone, Camera for both) and "Not now".
   - The page's request waits while the card shows. "Not now" or Escape answers it denied.
     Back from the settings (Android's onResume, the Mac app becoming active), Go asks
     oskit/media.Allowed again; a yes answers the waiting request granted and closes the card.
     A tab that navigates away or closes meanwhile answers it denied and drops the card.
   - It shows each time a page asks while the OS says no, one card per tab, never stacked;
     after "Not now", not again on that tab for that site until the page loads again (the page
     still gets its denial).
5. **The site panel**: the camera or microphone row of a site gets a muted second line while
   the OS says no, "Turned off for Project B in Android settings" (or "…in System Settings"),
   with the same Open settings.

**Tests.** oskit/media's fake in core/sites' tests (OS yes, OS no, asked once for both kinds);
drive_android with a getUserMedia page on both emulators' fake cameras: grant, then deny
(pm revoke), the card, Not now, and Open settings then a grant back; shots of the card in both
states to ux; the Mac app suite with the VM's camera where it has one.
**Declaring CAMERA changes a page's file input on older Android** (found by passwords'
scanner run, 2026-11-22): with CAMERA in the manifest and not yet held, Chromium's file chooser
asks for the camera before the picker so it can offer it as a source, as Chrome does; Android 10
showed that question, Android 17's photo picker didn't. passwords carries drive_android's
file_inputs change and puts the product change to him. Once the camera is allowed, the
chooser hands Android capture intents too, so Android 10 shows its own Camera or Files sheet
before the picker at every file input after; this follows from declaring CAMERA, as in Chrome.
**Who lands the CAMERA line**: whichever of this and passwords' QR scanner lands second keeps
the other's line.
