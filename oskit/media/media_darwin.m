// The Mac's camera and microphone permission (media_darwin.h).

#import <AVFoundation/AVFoundation.h>
#import <AppKit/AppKit.h>

#include "media_darwin.h"

#include "_cgo_export.h"

static BOOL HasBundle(void) {
  return NSBundle.mainBundle.bundleIdentifier != nil;
}

static AVMediaType TypeOf(int kind) {
  return kind == 0 ? AVMediaTypeVideo : AVMediaTypeAudio;
}

// Whether the Mac has a device of type: a Mac Studio or mini may have no camera, and one with
// none has nothing for macOS to let the app use. Asked then, macOS never answers.
static BOOL HasDevice(AVMediaType type) {
  return [AVCaptureDevice defaultDeviceWithMediaType:type] != nil;
}

int oskit_media_allowed(int kind) {
  if (!HasBundle() || !HasDevice(TypeOf(kind))) {
    return 1;
  }
  return [AVCaptureDevice authorizationStatusForMediaType:TypeOf(kind)] ==
         AVAuthorizationStatusAuthorized;
}

static void Answer(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    oskitMediaAnswered(oskit_media_allowed(0), oskit_media_allowed(1));
  });
}

// Asks for each kind in turn: macOS shows one question at a time.
static void AskFor(NSArray<AVMediaType>* types, NSUInteger i) {
  if (i == types.count) {
    Answer();
    return;
  }
  if (!HasDevice(types[i]) ||
      [AVCaptureDevice authorizationStatusForMediaType:types[i]] != AVAuthorizationStatusNotDetermined) {
    AskFor(types, i + 1);
    return;
  }
  [AVCaptureDevice requestAccessForMediaType:types[i]
                           completionHandler:^(BOOL granted) {
                             AskFor(types, i + 1);
                           }];
}

void oskit_media_ask(int camera, int microphone) {
  if (!HasBundle()) {
    Answer();
    return;
  }
  NSMutableArray<AVMediaType>* types = [NSMutableArray array];
  if (camera) {
    [types addObject:AVMediaTypeVideo];
  }
  if (microphone) {
    [types addObject:AVMediaTypeAudio];
  }
  AskFor(types, 0);
}

void oskit_media_open_settings(int kind) {
  NSString* pane = kind == 0 ? @"Privacy_Camera" : @"Privacy_Microphone";
  NSURL* url = [NSURL
      URLWithString:[@"x-apple.systempreferences:com.apple.preference.security?" stringByAppendingString:pane]];
  dispatch_async(dispatch_get_main_queue(), ^{
    [NSWorkspace.sharedWorkspace openURL:url];
  });
}

void oskit_media_watch_return(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    static id observer;
    if (observer) {
      return;
    }
    observer = [NSNotificationCenter.defaultCenter
        addObserverForName:NSApplicationDidBecomeActiveNotification
                    object:nil
                     queue:NSOperationQueue.mainQueue
                usingBlock:^(NSNotification* note) {
                  oskitMediaReturned();
                }];
  });
}
