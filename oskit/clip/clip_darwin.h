// The Mac's secret copy, in clip_darwin.m. Any thread may call it: it hops to the main thread.

#ifndef OSKIT_CLIP_CLIP_DARWIN_H_
#define OSKIT_CLIP_CLIP_DARWIN_H_

// Puts text on the general pasteboard with the concealed and transient types beside it
// (nspasteboard.org), so clipboard managers and history skip it.
void oskit_clip_copy_secret(const char* text);

#endif  // OSKIT_CLIP_CLIP_DARWIN_H_
