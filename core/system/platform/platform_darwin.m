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

// The app's icon, with its dot, and a progress bar along its foot.
@interface PBDockProgressView : NSView
@property double fraction;
@end

@implementation PBDockProgressView
- (void)drawRect:(NSRect)dirty {
  NSRect b = self.bounds;
  [NSApp.applicationIconImage drawInRect:b];
  NSRect bar = NSMakeRect(NSWidth(b) * 0.12, NSHeight(b) * 0.1, NSWidth(b) * 0.76, NSHeight(b) * 0.09);
  CGFloat r = NSHeight(bar) / 2;
  [[NSColor colorWithWhite:0 alpha:0.55] setFill];
  [[NSBezierPath bezierPathWithRoundedRect:bar xRadius:r yRadius:r] fill];
  NSRect fill = NSInsetRect(bar, 1.5, 1.5);
  fill.size.width = MAX(fill.size.height, fill.size.width * MIN(MAX(self.fraction, 0), 1));
  CGFloat fr = NSHeight(fill) / 2;
  [NSColor.controlAccentColor setFill];
  [[NSBezierPath bezierPathWithRoundedRect:fill xRadius:fr yRadius:fr] fill];
}
@end

void pb_system_dock_progress(double fraction) {
  OnMain(^{
    NSDockTile* tile = NSApp.dockTile;
    PBDockProgressView* view =
        [tile.contentView isKindOfClass:PBDockProgressView.class] ? (PBDockProgressView*)tile.contentView : nil;
    if (fraction < 0) {
      if (view != nil) {
        tile.contentView = nil;
        [tile display];
      }
      return;
    }
    if (view == nil) {
      view = [[PBDockProgressView alloc] initWithFrame:NSMakeRect(0, 0, tile.size.width, tile.size.height)];
      tile.contentView = view;
    }
    view.fraction = fraction;
    [tile display];
  });
}
