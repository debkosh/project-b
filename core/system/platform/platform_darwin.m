// The Mac's system calls (platform_darwin.h), which were the adapter's until the OS's calls moved
// into the Go core (docs/layers.md).

#import <AppKit/AppKit.h>
#import <UserNotifications/UserNotifications.h>

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

void pb_system_answer_notifications(void) {
  Answer(2, 1);
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

// macOS's notification centre needs an app bundle: a bare binary posts nothing.
static BOOL HasBundle(void) {
  return NSBundle.mainBundle.bundleIdentifier != nil;
}

// Shows a page's notifications while the app is in front too, and takes their clicks.
@interface PBNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation PBNotificationDelegate

- (void)userNotificationCenter:(UNUserNotificationCenter*)center
       willPresentNotification:(UNNotification*)notification
         withCompletionHandler:
             (void (^)(UNNotificationPresentationOptions))completionHandler {
  completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionList |
                    UNNotificationPresentationOptionSound);
}

- (void)userNotificationCenter:(UNUserNotificationCenter*)center
    didReceiveNotificationResponse:(UNNotificationResponse*)response
             withCompletionHandler:(void (^)(void))completionHandler {
  NSString* identifier = response.notification.request.identifier;
  BOOL clicked = [response.actionIdentifier isEqualToString:UNNotificationDefaultActionIdentifier];
  if (clicked) {
    dispatch_async(dispatch_get_main_queue(), ^{
      PBSystemNotificationClicked((char*)identifier.UTF8String);
    });
  }
  completionHandler();
}

@end

static UNUserNotificationCenter* Center(void) {
  static PBNotificationDelegate* delegate;
  UNUserNotificationCenter* center = UNUserNotificationCenter.currentNotificationCenter;
  if (!delegate) {
    delegate = [[PBNotificationDelegate alloc] init];
    center.delegate = delegate;
  }
  return center;
}

static NSString* String(const char* s) {
  return [NSString stringWithUTF8String:s];
}

void pb_system_show_notification(const char* id,
                                 const char* title,
                                 const char* subtitle,
                                 const char* body) {
  if (!HasBundle()) {
    return;
  }
  UNMutableNotificationContent* content = [[UNMutableNotificationContent alloc] init];
  content.title = String(title);
  content.subtitle = String(subtitle);
  content.body = String(body);
  content.threadIdentifier = String(subtitle);
  UNNotificationRequest* request = [UNNotificationRequest requestWithIdentifier:String(id)
                                                                        content:content
                                                                        trigger:nil];
  OnMain(^{
    UNUserNotificationCenter* center = Center();
    // macOS asks the person once whether the app may notify; until then, and if they say
    // no, the request does nothing.
    [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound)
                          completionHandler:^(BOOL granted, NSError* error) {
                            if (granted) {
                              [center addNotificationRequest:request withCompletionHandler:nil];
                            }
                          }];
  });
}

void pb_system_remove_notification(const char* id) {
  if (!HasBundle()) {
    return;
  }
  NSArray* ids = @[ String(id) ];
  OnMain(^{
    [Center() removeDeliveredNotificationsWithIdentifiers:ids];
    [Center() removePendingNotificationRequestsWithIdentifiers:ids];
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
