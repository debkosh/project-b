#!/bin/bash
# Makes testdata/home: a home folder with a Chrome of two profiles and a Firefox of one, in
# the browsers' own formats, for this package's tests and the app's (shell/e2e). Run it again
# after changing it; the files it makes are kept in git.
set -euo pipefail
cd "$(dirname "$0")"
HOME_DIR="$PWD/home"
rm -rf "${HOME_DIR:?}"
AS="$HOME_DIR/Library/Application Support"

# Chrome: Personal (Default) with bookmarks in folders, history and Bing chosen; Work
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
cat > "$C/Default/Preferences" <<'EOF'
{"default_search_provider_data": {"template_url_data": {"short_name": "Bing", "url": "https://www.bing.com/search?q={searchTerms}&form=QBLH"}}}
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
# that aren't bookmarks, history, and DuckDuckGo chosen. Times are microseconds since 1970.
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
# search.json.mozlz4: the JSON as one LZ4 block of literals alone.
python3 - "$P/search.json.mozlz4" <<'EOF'
import json, struct, sys
data = json.dumps({"version": 10, "metaData": {"defaultEngineId": "ddg"},
                   "engines": [{"id": "google", "_name": "Google"}, {"id": "ddg", "_name": "DuckDuckGo"}]}).encode()
n, token, length = len(data), b"", b""
if n < 15:
    token = bytes([n << 4])
else:
    token, rest = bytes([0xF0]), n - 15
    while rest >= 255:
        length += b"\xff"
        rest -= 255
    length += bytes([rest])
open(sys.argv[1], "wb").write(b"mozLz40\0" + struct.pack("<I", n) + token + length + data)
EOF
echo "made $HOME_DIR"
