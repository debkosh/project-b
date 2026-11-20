# Action coverage

Every action a person can take, where the app offers it, and the tests that do it. Written by
`tools/action_coverage.py` from the code; `tools/test.sh mock` fails while an action has no
test and no reason in `tools/action-coverage-exempt.json` (roles/tester.md).

**214 actions and messages, 206 with a test, 8 exempt with a reason, 0 with none; 28 of Go's actions done by no test of the real app.**

## Go's actions: the menu bar, keys and the shells' own menus

| Action | Offered in | Real app | Tests |
|---|---|---|---|
| `address.focus` Open Location… | menu bar, keys (chrome, firefox, glide, project-b), glide, page:welcome | yes | `drive_first_app.py` |
| `app.about` About Project B | menu bar, keys (chrome, firefox, glide, project-b) | **no** | `menu_test.go`, `shellmenu_test.go` |
| `app.checkUpdate` Check for Updates… | menu bar, keys (chrome, firefox, glide, project-b) | **no** | **TODO: the real app's Check for Updates against a local manifest, in drive_first_app** |
| `app.hide` Hide Project B | menu bar, keys (chrome, firefox, glide, project-b) | **no** | `commands_test.go` |
| `app.hideOthers` Hide Others | menu bar, keys (chrome, firefox, glide, project-b) | **no** | exempt: macOS's own: AppKit hides the other apps |
| `app.quit` Quit Project B | menu bar | **no** | `menu_test.go`, `commands_test.go`, `shellmenu_test.go`, `settings.cjs` and 1 more |
| `app.settings` Settings… | menu bar, fox, glide, minimal, mobile, standard, terminal, mobile-native | yes | `app_test.go`, `menu_test.go`, `settings_prepare_test.go`, `welcome_test.go` and 14 more |
| `app.showAll` Show All | menu bar, keys (chrome, firefox, glide, project-b) | **no** | exempt: macOS's own: AppKit shows the hidden apps |
| `bookmarks.show` Bookmark Manager | menu bar, keys (chrome, firefox, glide, project-b), mobile, standard, page:welcome | yes | `pages_test.go`, `pages.cjs`, `mobile.cjs`, `drive_first_app.py` |
| `bookmarks.toggle` Bookmark This Page… | menu bar, keys (chrome, firefox, glide, project-b) | **no** | `pages_test.go`, `pages.cjs` |
| `dev.viewSource` View Source | menu bar, keys (chrome, firefox, glide, project-b), page:welcome | yes | `app_test.go`, `drive_first_app.py` |
| `downloads.show` Downloads | menu bar, keys (chrome, firefox, glide, project-b), mobile, page:welcome, mobile-native | yes | `download_folder_settings_test.go`, `menu_test.go`, `pages_test.go`, `welcome_test.go` and 8 more |
| `edit.copy` Copy | menu bar, keys (chrome, firefox, glide, project-b), minimal, ui | yes | `contextmenu_test.go`, `menu_test.go`, `shellmenu_test.go`, `standard.cjs` and 1 more |
| `edit.cut` Cut | menu bar, keys (chrome, firefox, glide, project-b) | **no** | `contextmenu_test.go`, `standard.cjs` |
| `edit.delete` Delete | menu bar, keys (chrome, firefox, glide, project-b) | yes | `download-details.cjs`, `pages.cjs`, `passkey-settings.cjs`, `settings.cjs` and 3 more |
| `edit.paste` Paste | menu bar, keys (chrome, firefox, glide, project-b) | **no** | `contextmenu_test.go`, `menu_test.go`, `standard.cjs` |
| `edit.pasteAndMatchStyle` Paste and Match Style | menu bar, keys (chrome, firefox, glide, project-b) | **no** | exempt: macOS's own: AppKit's editing command, sent to the focused field |
| `edit.redo` Redo | menu bar, keys (chrome, firefox, glide, project-b), glide, minimal, ui | yes | `app_test.go`, `contextmenu_test.go`, `drive_first_app.py` |
| `edit.selectAll` Select All | menu bar, keys (chrome, firefox, glide, project-b) | **no** | `contextmenu_test.go` |
| `edit.undo` Undo | menu bar, keys (chrome, firefox, glide, project-b), glide, minimal, ui | **no** | `contextmenu_test.go` |
| `find.next` Find Next | menu bar, keys (chrome, firefox, glide, project-b), glide, minimal, ui | yes | `find_test.go`, `shellmenu_test.go`, `drive_first_app.py` |
| `find.open` Find… | menu bar, keys (chrome, firefox, glide, project-b), mobile, mobile-native | yes | `find_test.go`, `shellmenu_test.go`, `shell-contract.cjs`, `standard.cjs` and 1 more |
| `find.previous` Find Previous | menu bar, keys (chrome, firefox, glide, project-b), glide, minimal, ui | yes | `drive_first_app.py` |
| `focus.nextPane` Focus Next Pane | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `focus.previousPane` Focus Previous Pane | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `glide.commandBar` Open Command Bar | menu bar, keys (glide), glide | yes | `app_test.go`, `settings.cjs`, `glide.cjs`, `drive_first_app.py` |
| `glide.copyAddress` Copy Address | menu bar, keys (chrome, firefox, glide, project-b), glide | yes | `glide.cjs`, `check_clipboard.py`, `drive_first_app.py` |
| `glide.favorite` Add to Favorites | menu bar, glide | yes | `glide.cjs`, `drive_first_app.py` |
| `glide.newSpace` New Space… | menu bar, glide | yes | `drive_first_app.py` |
| `glide.sidebar` Show or Hide Sidebar | menu bar, keys (chrome, firefox, glide, project-b), glide | **no** | `commands_test.go`, `glide.cjs` |
| `glide.space1` Space 1 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `app_test.go`, `glide.cjs`, `drive_first_app.py` |
| `glide.space2` Space 2 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `glide.space3` Space 3 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `glide.space4` Space 4 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `glide.space5` Space 5 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `glide.space6` Space 6 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `glide.space7` Space 7 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `glide.space8` Space 8 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `glide.space9` Space 9 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `glide.spaceNext` Next Space | menu bar, keys (glide), glide | **no** | `app_test.go`, `glide.cjs` |
| `glide.spacePrevious` Previous Space | menu bar, keys (glide), glide | **no** | `glide.cjs` |
| `glide.splitAdd` Add to Split View | menu bar, keys (chrome, firefox, glide, project-b), glide | **no** | `settings.cjs`, `glide.cjs` |
| `glide.splitRemove` Separate Split View | menu bar, keys (chrome, firefox, glide, project-b), glide | **no** | `glide.cjs` |
| `history.show` Show All History | menu bar, keys (chrome, firefox, glide, project-b), mobile, page:welcome | yes | `pages_test.go`, `pages.cjs`, `drive_first_app.py` |
| `nav.back` Back | menu bar, keys (chrome, firefox, glide, project-b), fox, glide, minimal, ui | yes | `download-details.cjs`, `first-app.cjs`, `imports.cjs`, `shell-contract.cjs` and 6 more |
| `nav.forward` Forward | menu bar, keys (chrome, firefox, glide, project-b), fox, glide, minimal, ui | yes | `first-app.cjs`, `shell-contract.cjs`, `mobile.cjs`, `standard.cjs` and 2 more |
| `nav.reload` Reload This Page | menu bar, keys (chrome, firefox, glide, project-b), fox, glide, minimal, ui | yes | `menu_test.go`, `commands_test.go`, `settings.cjs`, `mobile.cjs` and 1 more |
| `nav.reloadHard` Force Reload This Page | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `nav.stop` Stop | menu bar, keys (chrome, firefox, glide, project-b), fox, glide, minimal, mobile, mobile-native | yes | `app_test.go`, `drive_first_app.py` |
| `page.print` Print… | menu bar, keys (chrome, firefox, glide, project-b), mobile | **no** | `contextmenu_test.go`, `print_test.go`, `shellmenu_test.go`, `mobile.cjs` |
| `profile.picker` Profiles… | menu bar | yes | `drive_first_app.py`, `check.py` |
| `tab.close` Close Tab | menu bar, keys (chrome, firefox, glide, project-b), glide, standard | yes | `commands_test.go`, `shellmenu_test.go`, `settings.cjs`, `shell-contract.cjs` and 3 more |
| `tab.duplicate` Duplicate Tab | menu bar, keys (chrome, firefox, glide, project-b), standard | yes | `app_test.go`, `settings_messages_test.go`, `suggest_test.go`, `standard-groups.cjs` and 1 more |
| `tab.moveToNewWindow` Move Tab to New Window | menu bar, keys (chrome, firefox, glide, project-b), standard | yes | `app_test.go`, `drive_first_app.py` |
| `tab.new` New Tab | menu bar, keys (chrome, firefox, glide, project-b), fox, glide, minimal, standard, terminal, page:welcome | yes | `app_test.go`, `menu_test.go`, `commands_test.go`, `shellmenu_test.go` and 15 more |
| `tab.next` Select Next Tab | menu bar, keys (chrome, firefox, glide, project-b), page:welcome | yes | `menu_test.go`, `drive_first_app.py` |
| `tab.pin` Pin Tab | menu bar, keys (chrome, firefox, glide, project-b), glide, standard | yes | `pins_test.go`, `glide.cjs`, `standard-pins.cjs`, `drive_first_app.py` |
| `tab.previous` Select Previous Tab | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `tab.recentNext` Select Recent Tab | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `tab.recentPrevious` Select Recent Tab Backward | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `tab.reopenClosed` Reopen Closed Tab | menu bar, keys (chrome, firefox, glide, project-b), page:welcome | yes | `session_test.go`, `drive_first_app.py` |
| `tab.select1` Tab 1 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `commands_test.go`, `shell-contract.cjs`, `fox.cjs`, `minimal.cjs` and 2 more |
| `tab.select2` Tab 2 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `mobile.cjs`, `drive_first_app.py` |
| `tab.select3` Tab 3 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `tab.select4` Tab 4 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `tab.select5` Tab 5 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `tab.select6` Tab 6 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `tab.select7` Tab 7 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `tab.select8` Tab 8 | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `tab.selectLast` Last Tab | menu bar, keys (chrome, firefox, glide, project-b) | yes | `minimal.cjs`, `drive_first_app.py` |
| `terminal.focusDown` Focus pane below | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `terminal.cjs` |
| `terminal.focusLeft` Focus pane left | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `terminal.cjs` |
| `terminal.focusRight` Focus pane right | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `terminal.cjs` |
| `terminal.focusUp` Focus pane above | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `terminal.cjs` |
| `terminal.resizeDown` Move divider down | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `terminal.cjs` |
| `terminal.resizeLeft` Move divider left | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `terminal.cjs` |
| `terminal.resizeRight` Move divider right | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `terminal.cjs` |
| `terminal.resizeUp` Move divider up | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `terminal.cjs` |
| `terminal.splitDown` Split down | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `app_test.go`, `terminal.cjs` |
| `terminal.splitRight` Split right | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `app_test.go`, `settings.cjs`, `terminal.cjs` |
| `terminal.zoom` Zoom pane | menu bar, keys (chrome, firefox, glide, project-b), terminal | **no** | `terminal.cjs` |
| `view.fullScreen` Full Screen | menu bar, keys (chrome, firefox, glide, project-b), fox | yes | `app_test.go`, `commands_test.go`, `fox.cjs`, `mobile.cjs` and 1 more |
| `view.tabsSide` Tabs on Side | menu bar, keys (chrome, firefox, glide, project-b) | **no** | `shellmenu_test.go`, `minimal.cjs`, `standard-bookmarks.cjs`, `standard-groups.cjs` and 2 more |
| `view.tabsTop` Tabs on Top | menu bar, keys (chrome, firefox, glide, project-b) | yes | `suggest_test.go`, `minimal.cjs`, `standard-bookmarks.cjs`, `standard-drag.cjs` and 4 more |
| `view.zoomIn` Zoom In | menu bar, keys (chrome, firefox, glide, project-b), fox | yes | `fox.cjs`, `drive_first_app.py` |
| `view.zoomOut` Zoom Out | menu bar, keys (chrome, firefox, glide, project-b), fox | yes | `fox.cjs`, `drive_first_app.py` |
| `view.zoomReset` Actual Size | menu bar, keys (chrome, firefox, glide, project-b), fox | yes | `fox.cjs`, `drive_first_app.py` |
| `window.bringAllToFront` Bring All to Front | menu bar, keys (chrome, firefox, glide, project-b) | **no** | exempt: macOS's own: AppKit orders the app's windows |
| `window.close` Close Window | menu bar, keys (chrome, firefox, glide, project-b) | yes | `drive_first_app.py` |
| `window.minimize` Minimize | menu bar, keys (chrome, firefox, glide, project-b) | **no** | `menu_test.go` |
| `window.new` New Window | menu bar, keys (chrome, firefox, glide, project-b), fox | yes | `app_test.go`, `shellmenu_test.go`, `fox.cjs`, `drive_first_app.py` |
| `window.zoom` Zoom | menu bar, keys (chrome, firefox, glide, project-b) | **no** | `terminal.cjs` |

## What the shells send Go when a person does something

| Action | Offered in | Real app | Tests |
|---|---|---|---|
| `activateTab` | fox, glide, minimal, mobile, mobile-native, standard, terminal | yes | `thumbs_test.go`, `settings.cjs`, `shell-contract.cjs`, `mobile.cjs` and 2 more |
| `addKey` | page:settings | **no** | `app_test.go`, `settings_messages_test.go` |
| `addToGroup` | glide, mobile, mobile-native, standard | yes | `drive_first_app.py` |
| `allowNotifications` | mobile, mobile-native | **no** | `download_offer_test.go` |
| `answerDialog` | mobile-native | **no** | `dialogs_test.go`, `passkeys_own_test.go`, `passkeys_test.go` |
| `answerPasskey` | mobile-native | **no** | `passkeys_test.go`, `passkey-settings.cjs` |
| `askNotifications` | page:welcome | yes | `drive_android.py` |
| `back` | glide, mobile-native | **no** | `fullscreen_test.go` |
| `backCloses` | mobile, mobile-native | **no** | exempt: not a person's action: the Mobile shell tells the engine whether the back key closes something of its own |
| `bookmark` | mobile-native | **no** | `pages_test.go`, `mobile.cjs`, `standard.cjs` |
| `cancelImport` | ui | **no** | `imports_extensions_test.go` |
| `cancelUpdate` | fox, page:settings | **no** | `settings_messages_test.go` |
| `captureTab` | mobile | **no** | `thumbs_test.go` |
| `checkUpdate` | page:settings | **no** | `settings_messages_test.go` |
| `chooseDownloadFolder` | page:settings | **no** | `download_folder_settings_test.go` |
| `chooseFolder` | ui | **no** | `imports_test.go` |
| `clear` | page:history | **no** | `pages_test.go` |
| `clearAllSiteData` | page:settings | **no** | `site_data_settings_test.go` |
| `clearData` | page:settings | **no** | `clear_data_settings_test.go` |
| `clearDataCounts` | page:settings | **no** | `clear_data_settings_test.go` |
| `clearDownloads` | page:downloads | **no** | `downloads_test.go`, `pages_test.go` |
| `clearSiteData` | page:settings | **no** | `site_data_settings_test.go` |
| `close` | page:downloads | **no** | `pages_test.go` |
| `closeGroup` | glide, mobile, mobile-native | **no** | `app_test.go` |
| `closeTab` | fox, glide, minimal, mobile, mobile-native, standard, terminal | **no** | `app_test.go`, `fullscreen_test.go`, `pins_test.go`, `session_test.go` and 6 more |
| `contextMenuChoose` | mobile-native | **no** | `contextmenu_test.go` |
| `contextMenuClose` | mobile-native | **no** | `contextmenu_test.go` |
| `copyText` | glide | **no** | `glide.cjs` |
| `create` | page:picker | **no** | `picker_test.go` |
| `createGroup` | glide, standard | yes | `shell-contract.cjs`, `standard-drag.cjs`, `terminal.cjs`, `drive_first_app.py` |
| `createWorkspace` | glide, page:settings | yes | `app_test.go`, `lifecycle_test.go`, `pins_test.go`, `drive_first_app.py` |
| `delete` | page:history | **no** | `pages_test.go` |
| `deleteDownload` | page:downloads | **no** | `pages_test.go` |
| `deleteWorkspace` | page:settings | **no** | `app_test.go`, `pins_test.go`, `standard-workspaces.cjs` |
| `dismissCrash` | fox, glide, minimal, mobile, mobile-native, standard | **no** | `crash-report.cjs` |
| `dismissMissing` | page:settings | **no** | `settings_messages_test.go` |
| `done` | page:welcome | **no** | `welcome_test.go` |
| `downloadAction` | mobile-native, page:downloads | **no** | `downloads_test.go`, `pages_test.go` |
| `editBookmark` | page:bookmarks | **no** | `pages_test.go` |
| `editGroup` | glide, mobile, mobile-native, standard | **no** | `app_test.go`, `terminal.cjs` |
| `editWorkspace` | glide, page:settings | **no** | `app_test.go` |
| `findBrowsers` | page:settings, page:welcome, ui | **no** | `imports_extensions_test.go`, `imports_test.go` |
| `findClose` | mobile-native | **no** | `find_test.go` |
| `findNext` | mobile-native | **no** | `find_test.go` |
| `findText` | mobile-native | **no** | `find_test.go` |
| `focusShell` | fox, glide, minimal, mobile, mobile-native, standard, terminal, ui | **no** | exempt: not a person's action: a shell asks for the keyboard back when its bar or panel opens |
| `focusTab` | mobile-native | **no** | `app_test.go`, `first-app.cjs`, `shell-contract.cjs`, `fox.cjs` and 2 more |
| `forgetSite` | mobile | **no** | `suggest_test.go` |
| `forgetSiteSetting` | page:settings | **no** | `settings_messages_test.go` |
| `forward` | glide, mobile-native | yes | `drive_first_app.py` |
| `import` | ui | **no** | `imports_extensions_test.go`, `imports_test.go` |
| `importFile` | ui | **no** | `imports_test.go` |
| `keepCookies` | page:settings | **no** | `quit_erasure_test.go` |
| `layout` | mobile-native, page:picker | **no** | `app_test.go`, `first-app.cjs` |
| `makeDefault` | page:settings, page:welcome | yes | `default_browser_settings_test.go`, `drive_android.py` |
| `menuOpened` | mobile, mobile-native | **no** | `settings_prepare_test.go` |
| `more` | page:history | **no** | exempt: not a person's action: the History page asks for its next rows as it scrolls |
| `moveBookmark` | page:bookmarks | **no** | `pages_test.go` |
| `moveTab` | standard, terminal | **no** | `app_test.go` |
| `moveWorkspace` | page:settings | **no** | `app_test.go` |
| `navigate` | fox, glide, minimal, mobile, mobile-native, standard, terminal | yes | `app_test.go`, `contextmenu_test.go`, `dialogs_test.go`, `files_test.go` and 13 more |
| `newFolder` | page:bookmarks | **no** | `pages_test.go`, `mobile.cjs` |
| `newTab` | fox, glide, minimal, mobile, mobile-native, standard, terminal | yes | `app_test.go`, `fullscreen_test.go`, `pins_test.go`, `session_test.go` and 17 more |
| `notificationsOffered` | mobile, mobile-native | **no** | `download_offer_test.go` |
| `open` | page:bookmarks, page:history, page:picker | **no** | `events_test.go`, `pages_test.go`, `picker_test.go` |
| `openAccess` | ui | **no** | `imports_test.go` |
| `openInWorkspace` | page:settings | **no** | `app_test.go` |
| `passkeyDelete` | page:settings | **no** | `passkey-settings.cjs` |
| `passkeysList` | page:settings | **no** | `passkey-settings.cjs` |
| `ready` | mobile-native, page:bookmarks, page:downloads, page:history, page:picker, page:settings, page:welcome | **no** | `events_test.go`, `app_test.go`, `default_browser_settings_test.go`, `extensions_test.go` and 8 more |
| `recording` | page:settings | **no** | `app_test.go` |
| `relaunch` | fox, glide, minimal, mobile, mobile-native, page:settings, standard, terminal | **no** | `app_test.go` |
| `reload` | glide, mobile-native | yes | `drive_first_app.py` |
| `removeBookmark` | mobile-native, page:bookmarks | **no** | `pages_test.go` |
| `removeFromGroup` | glide, mobile, mobile-native, standard | yes | `drive_first_app.py` |
| `removeKey` | page:settings | **no** | `settings_messages_test.go` |
| `resetAction` | page:settings | **no** | `settings_messages_test.go` |
| `resetProfile` | page:settings | **no** | `profile_test.go` |
| `retryUpdate` | fox, page:settings | **no** | `settings_messages_test.go` |
| `run` | fox, glide, minimal, mobile, mobile-native, standard, terminal | yes | `app_test.go`, `settings_prepare_test.go`, `check_clipboard.py`, `drive_first_app.py` |
| `saveBookmark` | mobile-native | **no** | `pages_test.go`, `mobile.cjs` |
| `saveReport` | mobile, standard | **no** | `downloads_test.go` |
| `search` | page:history | **no** | `pages_test.go` |
| `setAddressBarPosition` | page:settings | **no** | `settings_messages_test.go` |
| `setAutoUpdate` | page:settings, page:welcome | **no** | `app_test.go` |
| `setFlag` | page:settings | **no** | `settings_messages_test.go` |
| `setFrameColor` | page:settings | **no** | `app_test.go` |
| `setHidden` | page:settings | **no** | `settings_messages_test.go` |
| `setPref` | page:settings, page:welcome | **no** | `app_test.go`, `bookmark_settings_test.go`, `download_folder_settings_test.go`, `downloads_test.go` and 6 more |
| `setProfileName` | page:settings | **no** | `profile_test.go` |
| `setReserve` | page:settings | **no** | `settings_messages_test.go` |
| `setSearchEngine` | page:settings, page:welcome | **no** | `settings_messages_test.go` |
| `setShell` | page:settings | **no** | `app_test.go`, `settings.cjs` |
| `setSiteSetting` | page:settings | yes | `settings_messages_test.go`, `sitefeatures_test.go`, `drive_first_app.py` |
| `setSparkOn` | page:welcome | **no** | `welcome_test.go` |
| `setStart` | page:settings | **no** | `settings_messages_test.go` |
| `setTabLayout` | page:settings, standard | **no** | `app_test.go` |
| `setTheme` | page:settings | **no** | `app_test.go` |
| `setWorkspaces` | page:welcome | **no** | `welcome_test.go` |
| `shareReport` | mobile, mobile-native, standard | **no** | `crash-report.cjs` |
| `shown` | mobile-native, page:picker | **no** | `app_test.go`, `picker_test.go` |
| `sparkAdd` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkExport` | page:settings | **no** | `sparks_import_test.go` |
| `sparkImport` | page:settings | **no** | `sparks_import_test.go`, `sparks_messages_test.go` |
| `sparkImportCancel` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkImportKeep` | page:settings | **no** | `sparks_import_test.go` |
| `sparkRemove` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkSetOn` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkSource` | page:settings | **no** | `sparks_messages_test.go` |
| `sparkUpdate` | page:settings | **no** | `sparks_messages_test.go` |
| `suggest` | fox, minimal, mobile, mobile-native, terminal | **no** | `app_test.go`, `suggest_test.go` |
| `syncCancel` | page:settings | **no** | `sync.cjs` |
| `syncDetails` | page:settings | **no** | `sync_settings_test.go`, `sync.cjs` |
| `syncDisconnect` | page:settings | **no** | `sync.cjs` |
| `syncNow` | page:settings | yes | `sync.cjs`, `drive_android.py` |
| `syncPropose` | page:settings | **no** | `sync_settings_test.go` |
| `syncStartFresh` | page:settings | **no** | `sync.cjs` |
| `timeline` | mobile-native | **no** | `timeline_test.go` |
| `ungroup` | glide, mobile, mobile-native | **no** | `terminal.cjs` |
| `useKeymap` | page:welcome | **no** | `welcome_test.go` |
| `useTemplate` | page:settings | **no** | `settings_messages_test.go` |
| `welcomeAgain` | page:settings | **no** | `welcome_test.go` |
