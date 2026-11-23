# The update's notification through oskit/notify

Status: agreed and built 2026-11-23 (Android's progress moves too: one path everywhere). The
Android Install tap that did nothing (his report, 2026-11-23) was amarit's: its button sent a
broadcast, and Go then opened the package installer from the background, which Android 10 and
later don't allow; oskit's buttons start the app's activity first.

**Now.** Each OS shows the updater's state with amarit's own presenter, from core/cmd/lib: the
Mac `macapp.Present` (the ready notification with Relaunch, and the Dock tile's progress bar),
Android amarit's `UpdateNotification.java` (progress with Cancel, Install when ready, Try again),
Linux `flatpak.Source.Present` (the ready notification with Relaunch, through the portal). Three
notification paths beside oskit/notify, and on the Mac a second notification delegate's worth of
routing (`otherNote`, amarit's category).

**The way (amarit's suggestion).** No new repo, no change to amarit: a presenter is only a
listener on `appupdate.Updater` (`OnChange`, then `Relaunch`, `Cancel`, `Retry`). core/update
gets one presenter for every OS, over oskit/notify, and stops calling amarit's three; amarit
keeps them for apps without oskit. oskit imports nothing of amarit or core.

- **Ready**: a note, id `update`, "Project B 0.3.10 is ready", "Relaunch to update.", with
  Relaunch; a click on the note or the button relaunches. As amarit learned: the note goes when
  the app relaunches from inside, so it doesn't outlive the update; on Linux its button routes
  through the app's desktop file, as the portal does already.
- **Progress, Android only today**: while it downloads, the note shows the progress with Cancel;
  a failure a retry can help shows Try again. oskit/notify gains `Note.Ongoing`, `Note.Progress`
  (0 to 1, or -1 for unknown) and `Note.Quiet` (no sound, a channel of its own), which Android's
  halves draw; the Mac and Linux show only the ready note, as now.
- **The Mac's Dock progress bar** is not a notification: it stays the app's, as the Dock dot is,
  in core/system/platform (`SetDockProgress`), out of oskit.
- **What goes**: `otherNote`'s amarit routing, `platform.ShowUpdate`, the APK's copy of
  `UpdateNotification.java` and its strings, and the three `cfg.Present`s.

**Tests.** Go tests on oskit/notify's fake for each state (progress, ready, failed, relaunch
takes the note away); drive_android's update_on_the_phone and the VM's update check, which
watch the notification now, on the new path.

**For him.** One choice: Android's progress in oskit/notify (one path everywhere, as above), or
Android keeps amarit's progress notification and only the ready note moves (two paths stay on
Android). The first is recommended.
