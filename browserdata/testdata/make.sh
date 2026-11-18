#!/bin/bash
# Makes testdata/home: a home folder with a Chrome of two profiles, a Firefox of one and a
# Safari, in the browsers' own formats, for this package's tests and the app's (shell/e2e).
# Run it again after changing it; the files it makes are kept in git.
set -euo pipefail
cd "$(dirname "$0")"
HOME_DIR="$PWD/home"
rm -rf "${HOME_DIR:?}"
AS="$HOME_DIR/Library/Application Support"

# Chrome: Personal (Default) with bookmarks in folders and history; Work
# (Profile 1) with one bookmark and no history. Times are microseconds since 1601.
C="$AS/Google/Chrome"
mkdir -p "$C/Default" "$C/Profile 1" "$C/System Profile"
cat > "$C/Local State" <<'EOF'
{"profile": {"info_cache": {"Default": {"name": "Personal"}, "Profile 1": {"name": "Work"}}}}
EOF
cat > "$C/Default/Bookmarks" <<'EOF'
{
  "roots": {
    "bookmark_bar": {"name": "Bookmarks bar", "type": "folder", "children": [
      {"name": "Docs", "type": "url", "url": "https://example.com/docs", "date_added": "13370000000000000"},
      {"name": "Work", "type": "folder", "date_added": "13370000000000000", "children": [
        {"name": "Tracker", "type": "url", "url": "https://example.com/tracker", "date_added": "13370000000000000"},
        {"name": "Deep", "type": "folder", "children": [
          {"name": "Deep link", "type": "url", "url": "https://example.com/deep", "date_added": "13370000000000000"}
        ]}
      ]}
    ]},
    "other": {"name": "Other bookmarks", "type": "folder", "children": [
      {"name": "News", "type": "url", "url": "https://example.org/news", "date_added": "13370000000000000"}
    ]},
    "synced": {"name": "Mobile bookmarks", "type": "folder", "children": []}
  },
  "version": 1
}
EOF
# Its extensions, in Secure Preferences as Chrome keeps them: two from the Chrome Web Store,
# one whose manifest is kept there and one whose manifest is in its folder, named through
# its messages; one of Manifest V2, one from another store, one loaded unpacked from a
# folder that is gone; and three it didn't install itself: by default, Chrome's own and by
# policy.
cat > "$C/Default/Secure Preferences" <<'EOF'
{"extensions": {"settings": {
  "oepjjojhhaclfhmopocgmdfbeiffpheh": {"location": 1, "from_webstore": true, "path": "oepjjojhhaclfhmopocgmdfbeiffpheh/1.0_0",
    "manifest": {"manifest_version": 3, "name": "Project B test extension", "version": "1.0", "update_url": "https://clients2.google.com/service/update2/crx"}},
  "ddkjiahejlhfcafbddmgiahcphecmpfh": {"location": 1, "from_webstore": true, "path": "ddkjiahejlhfcafbddmgiahcphecmpfh/2025.1_0"},
  "nngceckbapebfimnlniiiahkandclblb": {"location": 1, "from_webstore": true, "path": "nngceckbapebfimnlniiiahkandclblb/1.9_0",
    "manifest": {"manifest_version": 2, "name": "Old Notes", "version": "1.9", "update_url": "https://clients2.google.com/service/update2/crx"}},
  "abcdefghijklmnopabcdefghijklmnop": {"location": 1, "from_webstore": false, "path": "abcdefghijklmnopabcdefghijklmnop/3.0_0",
    "manifest": {"manifest_version": 3, "name": "Other Store Helper", "version": "3.0", "update_url": "https://edge.microsoft.com/extensionwebstorebase/v1/crx"}},
  "pponmlkjihgfedcbaponmlkjihgfedcb": {"location": 4, "path": "/nowhere/my-extension"},
  "ghbmnnjooekpmoecnnnilnnbdlolhkhi": {"location": 1, "from_webstore": true, "was_installed_by_default": true,
    "manifest": {"manifest_version": 3, "name": "Docs Offline", "version": "1.9"}},
  "mhjfbmdgcfjbbpaeojofohoefgiehjai": {"location": 5, "manifest": {"manifest_version": 2, "name": "PDF Viewer", "version": "1"}},
  "aaaabbbbccccddddeeeeffffgggghhhh": {"location": 7, "manifest": {"manifest_version": 3, "name": "Company Policy", "version": "1"}}
}}}
EOF
U="$C/Default/Extensions/ddkjiahejlhfcafbddmgiahcphecmpfh/2025.1_0"
mkdir -p "$U/_locales/en"
cat > "$U/manifest.json" <<'EOF'
{"manifest_version": 3, "name": "__MSG_extName__", "version": "2025.1", "default_locale": "en", "update_url": "https://clients2.google.com/service/update2/crx"}
EOF
cat > "$U/_locales/en/messages.json" <<'EOF'
{"extName": {"message": "uBlock Origin Lite"}}
EOF
sqlite3 "$C/Default/History" <<'EOF'
CREATE TABLE urls(id INTEGER PRIMARY KEY AUTOINCREMENT, url LONGVARCHAR, title LONGVARCHAR, visit_count INTEGER DEFAULT 0 NOT NULL, typed_count INTEGER DEFAULT 0 NOT NULL, last_visit_time INTEGER NOT NULL, hidden INTEGER DEFAULT 0 NOT NULL);
INSERT INTO urls(url, title, visit_count, last_visit_time, hidden) VALUES
  ('https://example.com/docs', 'Docs', 12, 13370000000000000, 0),
  ('https://example.com/tracker', 'Tracker', 3, 13370100000000000, 0),
  ('http://example.org/old', 'Old', 1, 13360000000000000, 0),
  ('chrome://settings/', 'Settings', 4, 13370000000000000, 0),
  ('https://example.com/hidden', 'Hidden', 2, 13370000000000000, 1);
EOF
cat > "$C/Profile 1/Bookmarks" <<'EOF'
{"roots": {"bookmark_bar": {"name": "Bookmarks bar", "type": "folder", "children": [
  {"name": "Wiki", "type": "url", "url": "https://example.com/wiki", "date_added": "13370000000000000"}
]}, "other": {"name": "Other bookmarks", "type": "folder", "children": []}}}
EOF
echo '{}' > "$C/System Profile/Bookmarks"

# Firefox: default-release with bookmarks in its toolbar and menu, a tag and a separator
# that aren't bookmarks, and history. Times are microseconds since 1970.
F="$AS/Firefox"
P="$F/Profiles/test.default-release"
mkdir -p "$P"
cat > "$F/profiles.ini" <<'EOF'
[Install4F96D1932A9F858E]
Default=Profiles/test.default-release

[Profile0]
Name=default-release
IsRelative=1
Path=Profiles/test.default-release
Default=1

[Profile1]
Name=gone
IsRelative=1
Path=Profiles/gone.default
EOF
sqlite3 "$P/places.sqlite" <<'EOF'
CREATE TABLE moz_places(id INTEGER PRIMARY KEY, url LONGVARCHAR, title LONGVARCHAR, visit_count INTEGER DEFAULT 0, hidden INTEGER DEFAULT 0 NOT NULL, last_visit_date INTEGER);
CREATE TABLE moz_bookmarks(id INTEGER PRIMARY KEY, type INTEGER, fk INTEGER DEFAULT NULL, parent INTEGER, position INTEGER, title LONGVARCHAR, dateAdded INTEGER, lastModified INTEGER, guid TEXT);
INSERT INTO moz_places VALUES
  (1, 'https://example.net/fx', 'Fx', 5, 0, 1700000000000000),
  (2, 'https://example.net/read', 'Read', 2, 0, 1700000100000000),
  (3, 'place:sort=8&maxResults=10', 'Most visited', 0, 1, NULL),
  (4, 'https://example.net/unvisited', 'Unvisited', 0, 0, NULL);
INSERT INTO moz_bookmarks VALUES
  (1, 2, NULL, 0, 0, '', 1700000000000000, 0, 'root________'),
  (2, 2, NULL, 1, 0, 'menu', 1700000000000000, 0, 'menu________'),
  (3, 2, NULL, 1, 1, 'toolbar', 1700000000000000, 0, 'toolbar_____'),
  (4, 2, NULL, 1, 2, 'tags', 1700000000000000, 0, 'tags________'),
  (5, 2, NULL, 1, 3, 'unfiled', 1700000000000000, 0, 'unfiled_____'),
  (6, 2, NULL, 1, 4, 'mobile', 1700000000000000, 0, 'mobile______'),
  (7, 1, 1, 3, 0, 'Fx', 1700000000000000, 0, 'aaaaaaaaaaaa'),
  (8, 1, 3, 3, 1, 'Most visited', 1700000000000000, 0, 'bbbbbbbbbbbb'),
  (9, 2, NULL, 2, 0, 'Reading', 1700000000000000, 0, 'cccccccccccc'),
  (10, 3, NULL, 2, 1, '', 1700000000000000, 0, 'dddddddddddd'),
  (11, 1, 2, 9, 0, 'Read', 1700000000000000, 0, 'eeeeeeeeeeee'),
  (12, 2, NULL, 4, 0, 'a tag', 1700000000000000, 0, 'ffffffffffff'),
  (13, 1, 4, 12, 0, '', 1700000000000000, 0, 'gggggggggggg');
EOF
# Forks, which keep their parent's files: an Opera with its one profile in its own folder,
# as Opera can; a Zen, Firefox's, with one profile; and, outside Application Support, a
# Chromium browser this package doesn't know, Thorium, whose folder a person would choose.
O="$AS/com.operasoftware.Opera"
mkdir -p "$O"
cp "$C/Profile 1/Bookmarks" "$O/Bookmarks"
Z="$AS/zen"
mkdir -p "$Z/Profiles/zen.default"
cp "$P/places.sqlite" "$Z/Profiles/zen.default/places.sqlite"
printf '[Profile0]\nName=Default (release)\nIsRelative=1\nPath=Profiles/zen.default\n' > "$Z/profiles.ini"
T="$HOME_DIR/Elsewhere/Thorium/User Data"
mkdir -p "$T/Default"
cp "$C/Profile 1/Bookmarks" "$T/Default/Bookmarks"
echo '{"profile": {"info_cache": {"Default": {"name": "Me"}}}}' > "$T/Local State"

# Safari, in Library/Safari: bookmarks in Favorites, a folder and the Reading List, Safari's
# History proxy that isn't one, an empty Bookmarks Menu; history, with a page not visited
# and one not on the web. Its History.db has
# Safari's own tables, as macOS 26 makes them. Times are seconds since 2001.
S="$HOME_DIR/Library/Safari"
mkdir -p "$S"
python3 - "$S/Bookmarks.plist" <<'EOF'
import datetime, plistlib, sys
def leaf(title, url, **more):
    return {"WebBookmarkType": "WebBookmarkTypeLeaf", "URLString": url, "URIDictionary": {"title": title}, **more}
def folder(title, children):
    return {"WebBookmarkType": "WebBookmarkTypeList", "Title": title, "Children": children}
added = datetime.datetime(2026, 11, 1, 12, 0, 0)
plistlib.dump({"WebBookmarkFileVersion": 1, "Children": [
    {"WebBookmarkType": "WebBookmarkTypeProxy", "Title": "History", "WebBookmarkIdentifier": "History"},
    folder("BookmarksBar", [
        leaf("Apple", "https://www.apple.com/"),
        folder("Recipes", [leaf("Soup", "https://example.com/soup", dateAdded=added), leaf("Bread", "https://example.com/bread")]),
    ]),
    folder("BookmarksMenu", []),
    folder("com.apple.ReadingList", [leaf("Article", "https://example.com/article", ReadingList={"DateAdded": added})]),
]}, open(sys.argv[1], "wb"), fmt=plistlib.FMT_BINARY)
EOF
sqlite3 "$S/History.db" <<'EOF'
CREATE TABLE history_items (id INTEGER PRIMARY KEY AUTOINCREMENT,url TEXT NOT NULL UNIQUE,domain_expansion TEXT NULL,visit_count INTEGER NOT NULL,daily_visit_counts BLOB NOT NULL,weekly_visit_counts BLOB NULL,autocomplete_triggers BLOB NULL,should_recompute_derived_visit_counts INTEGER NOT NULL,visit_count_score INTEGER NOT NULL,status_code INTEGER NOT NULL DEFAULT 0);
CREATE TABLE history_visits (id INTEGER PRIMARY KEY AUTOINCREMENT,history_item INTEGER NOT NULL REFERENCES history_items(id) ON DELETE CASCADE,visit_time REAL NOT NULL,title TEXT NULL,load_successful BOOLEAN NOT NULL DEFAULT 1,http_non_get BOOLEAN NOT NULL DEFAULT 0,synthesized BOOLEAN NOT NULL DEFAULT 0,redirect_source INTEGER NULL UNIQUE REFERENCES history_visits(id) ON DELETE CASCADE,redirect_destination INTEGER NULL UNIQUE REFERENCES history_visits(id) ON DELETE CASCADE,origin INTEGER NOT NULL DEFAULT 0,generation INTEGER NOT NULL DEFAULT 0,attributes INTEGER NOT NULL DEFAULT 0,score INTEGER NOT NULL DEFAULT 0);
INSERT INTO history_items (id, url, domain_expansion, visit_count, daily_visit_counts, should_recompute_derived_visit_counts, visit_count_score) VALUES
  (1, 'https://example.com/soup', 'example', 3, x'', 0, 300),
  (2, 'http://example.org/old', 'example', 1, x'', 0, 100),
  (3, 'file:///Users/test/page.html', NULL, 2, x'', 0, 200),
  (4, 'https://example.com/never', 'example', 0, x'', 0, 0);
INSERT INTO history_visits (history_item, visit_time, title) VALUES
  (1, 815000000.5, 'Soup, first'),
  (1, 815100000.25, 'Soup'),
  (1, 815200000.75, ''),
  (2, 814000000, NULL),
  (3, 815000000, 'A file');
EOF
echo "made $HOME_DIR"
