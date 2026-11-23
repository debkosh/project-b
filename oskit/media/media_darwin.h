// The Mac's camera and microphone permission, in media_darwin.m (AVCaptureDevice). Answers reach
// Go as oskitMediaAnswered and a return to the front as oskitMediaReturned, on the main thread.

#ifndef OSKIT_MEDIA_MEDIA_DARWIN_H_
#define OSKIT_MEDIA_MEDIA_DARWIN_H_

// kind: 0 the camera, 1 the microphone. 1 when macOS lets the app use it. A bare binary, as tests
// run, has no Info.plist to say why it asks, and a Mac with no such device has nothing to let
// the app use (macOS never answers an ask for it): both are answered 1 without asking macOS, and
// content answers the page itself, with its fake device or NotFoundError.
int oskit_media_allowed(int kind);
// Asks macOS for the camera when camera, then the microphone when microphone: each asks the
// person once, then answers from Privacy & Security.
void oskit_media_ask(int camera, int microphone);
// Opens System Settings' Privacy & Security at kind.
void oskit_media_open_settings(int kind);
// Tells Go each time the app becomes active, from now on.
void oskit_media_watch_return(void);

#endif  // OSKIT_MEDIA_MEDIA_DARWIN_H_
