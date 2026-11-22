// The Mac's secret copy (clip_darwin.h), through NSPasteboard.

#import <AppKit/AppKit.h>

#include "clip_darwin.h"

void oskit_clip_copy_secret(const char* text) {
  NSString* secret = [NSString stringWithUTF8String:text];
  if (secret == nil) {
    return;
  }
  dispatch_block_t copy = ^{
    NSPasteboard* board = NSPasteboard.generalPasteboard;
    [board clearContents];
    [board setString:secret forType:NSPasteboardTypeString];
    [board setString:@"" forType:@"org.nspasteboard.ConcealedType"];
    [board setString:@"" forType:@"org.nspasteboard.TransientType"];
  };
  if (NSThread.isMainThread) {
    copy();
  } else {
    dispatch_async(dispatch_get_main_queue(), copy);
  }
}
