// The Mac's system calls (platform_darwin.h), which were the adapter's until the OS's calls moved
// into the Go core (docs/layers.md).

#import <AppKit/AppKit.h>
#import <LocalAuthentication/LocalAuthentication.h>

#include "platform_darwin.h"

#include "_cgo_export.h"

// Runs block on the main thread: now when the caller is on it, else soon.
static void OnMain(dispatch_block_t block) {
  if (NSThread.isMainThread) {
    block();
  } else {
    dispatch_async(dispatch_get_main_queue(), block);
  }
}

// Tells Go a question's answer on the main thread, after whatever asked has returned.
static void Answer(int32_t question, int yes) {
  dispatch_async(dispatch_get_main_queue(), ^{
    PBSystemAnswered(question, yes);
  });
}

static NSURL* FileURL(const char* path) {
  return [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
}

void pb_system_open_file(const char* path) {
  NSURL* url = FileURL(path);
  OnMain(^{
    [NSWorkspace.sharedWorkspace openURL:url];
  });
}

void pb_system_copy_secret(const char* text) {
  NSString* secret = [NSString stringWithUTF8String:text];
  if (secret == nil) {
    return;
  }
  OnMain(^{
    NSPasteboard* board = NSPasteboard.generalPasteboard;
    [board clearContents];
    [board setString:secret forType:NSPasteboardTypeString];
    [board setString:@"" forType:@"org.nspasteboard.ConcealedType"];
    [board setString:@"" forType:@"org.nspasteboard.TransientType"];
  });
}

void pb_system_reveal_file(const char* path) {
  NSURL* url = FileURL(path);
  OnMain(^{
    [NSWorkspace.sharedWorkspace activateFileViewerSelectingURLs:@[ url ]];
  });
}

int pb_system_is_default_browser(void) {
  NSURL* app = [NSWorkspace.sharedWorkspace
      URLForApplicationToOpenURL:[NSURL URLWithString:@"https://example.com/"]];
  return app && [[NSBundle bundleWithURL:app].bundleIdentifier
                    isEqualToString:NSBundle.mainBundle.bundleIdentifier];
}

void pb_system_ask_default_browser(void) {
  // macOS asks the person once, for http, and changes https with it; https is set too in case
  // it didn't. Either completion may come on any queue.
  NSURL* app = NSBundle.mainBundle.bundleURL;
  void (^report)(void) = ^{
    Answer(1, pb_system_is_default_browser());
  };
  OnMain(^{
    [NSWorkspace.sharedWorkspace
        setDefaultApplicationAtURL:app
              toOpenURLsWithScheme:@"http"
                 completionHandler:^(NSError* error) {
                   if (error) {
                     NSLog(@"default browser: %@", error.localizedDescription);
                     report();
                     return;
                   }
                   [NSWorkspace.sharedWorkspace setDefaultApplicationAtURL:app
                                                      toOpenURLsWithScheme:@"https"
                                                         completionHandler:^(NSError* e) {
                                                           report();
                                                         }];
                 }];
  });
}

// The checks of the person macOS has not answered, by Go's number. Main thread only.
static NSMutableDictionary<NSNumber*, LAContext*>* PersonChecks(void) {
  static NSMutableDictionary<NSNumber*, LAContext*>* checks;
  if (!checks) {
    checks = [NSMutableDictionary dictionary];
  }
  return checks;
}

void pb_system_verify_person(int32_t id, const char* reason) {
  NSString* why = [NSString stringWithUTF8String:reason];
  OnMain(^{
    LAContext* context = [[LAContext alloc] init];
    NSError* error = nil;
    if (![context canEvaluatePolicy:LAPolicyDeviceOwnerAuthentication error:&error]) {
      // A Mac with no password has nothing to check with.
      int verdict = error.code == LAErrorPasscodeNotSet ? 2 : 1;
      NSLog(@"verify person: %@", error.localizedDescription);
      dispatch_async(dispatch_get_main_queue(), ^{
        PBSystemPersonVerified(id, verdict);
      });
      return;
    }
    PersonChecks()[@(id)] = context;
    // The reply comes on a queue of LocalAuthentication's own.
    [context evaluatePolicy:LAPolicyDeviceOwnerAuthentication
            localizedReason:why
                      reply:^(BOOL success, NSError* e) {
                        dispatch_async(dispatch_get_main_queue(), ^{
                          [PersonChecks() removeObjectForKey:@(id)];
                          PBSystemPersonVerified(id, success ? 0 : 1);
                        });
                      }];
  });
}

void pb_system_cancel_person(int32_t id) {
  OnMain(^{
    // The sheet goes, and its reply, a failure, finds Go no longer waiting.
    [PersonChecks()[@(id)] invalidate];
  });
}

void pb_system_set_dock_color(int32_t rgb) {
  OnMain(^{
    // The bundle's own icon, as it was before the first dot.
    static NSImage* plain;
    if (!plain) {
      plain = [NSApp.applicationIconImage copy];
    }
    // No colour picked: no dot.
    if (rgb < 0) {
      NSApp.applicationIconImage = plain;
      return;
    }
    NSColor* color = [NSColor colorWithSRGBRed:((rgb >> 16) & 0xff) / 255.0
                                         green:((rgb >> 8) & 0xff) / 255.0
                                          blue:(rgb & 0xff) / 255.0
                                         alpha:1];
    NSImage* icon = plain;
    NSApp.applicationIconImage = [NSImage
         imageWithSize:icon.size
               flipped:NO
        drawingHandler:^BOOL(NSRect rect) {
          [icon drawInRect:rect];
          // A dot a quarter of the icon across, inside the artwork's corner, ringed in white
          // so it shows on any colour.
          const CGFloat d = rect.size.width * 0.25;
          const CGFloat inset = rect.size.width * 0.1;
          NSRect dot = NSMakeRect(NSMaxX(rect) - inset - d, NSMinY(rect) + inset, d, d);
          [NSColor.whiteColor setFill];
          [[NSBezierPath bezierPathWithOvalInRect:dot] fill];
          [color setFill];
          [[NSBezierPath bezierPathWithOvalInRect:NSInsetRect(dot, d * 0.1, d * 0.1)] fill];
          return YES;
        }];
  });
}

// The links and files macOS opens with the app, as its default browser or from a file opened
// with it (Info.plist's URL and document types).
@interface PBLinkReceiver : NSObject <NSApplicationDelegate>
@end

@implementation PBLinkReceiver
- (void)application:(NSApplication*)application openURLs:(NSArray<NSURL*>*)urls {
  NSMutableArray<NSString*>* lines = [NSMutableArray array];
  for (NSURL* url in urls) {
    [lines addObject:url.absoluteString];
  }
  PBSystemOpenURLs((char*)[lines componentsJoinedByString:@"\n"].UTF8String);
}
@end

void pb_system_receive_links(void) {
  OnMain(^{
    // NSApp's delegate is weak, so the receiver lives as long as the app.
    static PBLinkReceiver* receiver;
    if (!receiver) {
      receiver = [[PBLinkReceiver alloc] init];
    }
    NSApp.delegate = receiver;
  });
}
