// The Mac's opening of files and links (open_darwin.h), through NSWorkspace and NSApp.

#import <AppKit/AppKit.h>

#include "open_darwin.h"

#include "_cgo_export.h"

static void OnMain(dispatch_block_t block) {
  if (NSThread.isMainThread) {
    block();
  } else {
    dispatch_async(dispatch_get_main_queue(), block);
  }
}

static NSURL* FileURL(const char* path) {
  return [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
}

void oskit_open_file(const char* path) {
  NSURL* url = FileURL(path);
  OnMain(^{
    [NSWorkspace.sharedWorkspace openURL:url];
  });
}

void oskit_open_reveal(const char* path) {
  NSURL* url = FileURL(path);
  OnMain(^{
    [NSWorkspace.sharedWorkspace activateFileViewerSelectingURLs:@[ url ]];
  });
}

int oskit_open_is_default_browser(void) {
  NSURL* app = [NSWorkspace.sharedWorkspace
      URLForApplicationToOpenURL:[NSURL URLWithString:@"https://example.com/"]];
  return app && [[NSBundle bundleWithURL:app].bundleIdentifier
                    isEqualToString:NSBundle.mainBundle.bundleIdentifier];
}

void oskit_open_ask_default_browser(void) {
  // macOS asks the person once, for http, and changes https with it; https is set too in case
  // it didn't. Either completion may come on any queue.
  NSURL* app = NSBundle.mainBundle.bundleURL;
  void (^report)(void) = ^{
    dispatch_async(dispatch_get_main_queue(), ^{
      oskitOpenDefaultBrowser(oskit_open_is_default_browser());
    });
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

// The links and files macOS opens with the app, as its default browser or from a file opened
// with it (Info.plist's URL and document types).
@interface OskitLinkReceiver : NSObject <NSApplicationDelegate>
@end

@implementation OskitLinkReceiver
- (void)application:(NSApplication*)application openURLs:(NSArray<NSURL*>*)urls {
  NSMutableArray<NSString*>* lines = [NSMutableArray array];
  for (NSURL* url in urls) {
    [lines addObject:url.absoluteString];
  }
  oskitOpenURLs((char*)[lines componentsJoinedByString:@"\n"].UTF8String);
}
@end

void oskit_open_receive_links(void) {
  OnMain(^{
    // NSApp's delegate is weak, so the receiver lives as long as the app.
    static OskitLinkReceiver* receiver;
    if (!receiver) {
      receiver = [[OskitLinkReceiver alloc] init];
    }
    NSApp.delegate = receiver;
  });
}
