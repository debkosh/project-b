#include "project_b/engine/browser/notifications.h"

#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>

#include "base/memory/weak_ptr.h"
#include "base/strings/sys_string_conversions.h"

// macOS's notification centre, for the notifications a page shows. It needs an app bundle:
// a bare binary, as the tests run, posts nothing.

namespace {

// The service that posted the notifications the centre shows; one profile per process.
project_b::Notifications* g_owner = nullptr;

bool HasBundle() {
  return NSBundle.mainBundle.bundleIdentifier != nil;
}

}  // namespace

// Shows them while the app is in front too, and takes their clicks.
@interface PBNotificationDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation PBNotificationDelegate

- (void)userNotificationCenter:(UNUserNotificationCenter*)center
       willPresentNotification:(UNNotification*)notification
         withCompletionHandler:
             (void (^)(UNNotificationPresentationOptions))completionHandler {
  completionHandler(UNNotificationPresentationOptionBanner |
                    UNNotificationPresentationOptionList |
                    UNNotificationPresentationOptionSound);
}

- (void)userNotificationCenter:(UNUserNotificationCenter*)center
    didReceiveNotificationResponse:(UNNotificationResponse*)response
             withCompletionHandler:(void (^)(void))completionHandler {
  std::string id = base::SysNSStringToUTF8(
      response.notification.request.identifier);
  bool clicked =
      [response.actionIdentifier isEqualToString:UNNotificationDefaultActionIdentifier];
  dispatch_async(dispatch_get_main_queue(), ^{
    if (clicked && g_owner) {
      g_owner->Clicked(id);
    }
  });
  completionHandler();
}

@end

namespace project_b {

namespace {

UNUserNotificationCenter* Center() {
  static PBNotificationDelegate* delegate = nil;
  UNUserNotificationCenter* center =
      UNUserNotificationCenter.currentNotificationCenter;
  if (!delegate) {
    delegate = [[PBNotificationDelegate alloc] init];
    center.delegate = delegate;
  }
  return center;
}

}  // namespace

void PostSystemNotification(Notifications* owner,
                            const std::string& id,
                            const std::string& title,
                            const std::string& subtitle,
                            const std::string& body) {
  if (!HasBundle()) {
    return;
  }
  g_owner = owner;
  UNMutableNotificationContent* content = [[UNMutableNotificationContent alloc] init];
  content.title = base::SysUTF8ToNSString(title);
  content.subtitle = base::SysUTF8ToNSString(subtitle);
  content.body = base::SysUTF8ToNSString(body);
  content.threadIdentifier = base::SysUTF8ToNSString(subtitle);
  UNNotificationRequest* request =
      [UNNotificationRequest requestWithIdentifier:base::SysUTF8ToNSString(id)
                                           content:content
                                           trigger:nil];
  UNUserNotificationCenter* center = Center();
  // macOS asks the person once whether the app may notify; until then, and if they say
  // no, the request does nothing.
  [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert |
                                           UNAuthorizationOptionSound)
                        completionHandler:^(BOOL granted, NSError* error) {
                          if (granted) {
                            [center addNotificationRequest:request
                                     withCompletionHandler:nil];
                          }
                        }];
}

void RemoveSystemNotification(const std::string& id) {
  if (!HasBundle()) {
    return;
  }
  NSArray* ids = @[ base::SysUTF8ToNSString(id) ];
  [Center() removeDeliveredNotificationsWithIdentifiers:ids];
  [Center() removePendingNotificationRequestsWithIdentifiers:ids];
}

}  // namespace project_b
