package blockingtest

import "projectb/blocking"

// CatalogueJSON is a small catalogue in catalog.json's shape: two lists in ads, one in
// trackers, two regional ones, one in annoyances, off, one in each Safe Browsing group,
// and the test group, never shown.
const CatalogueJSON = `{
  "groups": [
    {"id": "ads", "title": "Ads", "about": "Ads and their servers", "blocker": "network", "shown": true},
    {"id": "trackers", "title": "Trackers", "about": "What follows you", "blocker": "network", "shown": true},
    {"id": "regional", "title": "Regional", "about": "Ads of a language", "blocker": "network", "shown": true},
    {"id": "annoyances", "title": "Annoyances", "about": "Cookie notices", "blocker": "network", "shown": true},
    {"id": "malware", "title": "Malware", "about": "Sites that harm", "blocker": "safe", "shown": true},
    {"id": "phishing", "title": "Phishing", "about": "Sites that steal", "blocker": "safe", "shown": true},
    {"id": "test", "title": "Test", "about": "For the checks", "blocker": "network", "shown": false}
  ],
  "lists": [
    {"id": "easylist", "title": "EasyList", "about": "Ads", "group": "ads", "on": true, "sources": [{"url": "https://easylist.test/easylist.txt", "format": "standard"}], "licence": "GPL-3.0", "home": "https://easylist.test/"},
    {"id": "unbreak", "title": "Unbreak", "about": "Fixes", "group": "ads", "on": true, "sources": [{"url": "https://unbreak.test/u.txt", "format": "standard"}], "licence": "GPL-3.0", "home": "https://unbreak.test/"},
    {"id": "easyprivacy", "title": "EasyPrivacy", "about": "Trackers", "group": "trackers", "on": true, "sources": [{"url": "https://easylist.test/easyprivacy.txt", "format": "standard"}], "licence": "GPL-3.0", "home": "https://easylist.test/"},
    {"id": "de", "title": "German", "about": "German ads", "group": "regional", "on": false, "languages": ["de"], "sources": [{"url": "https://de.test/de.txt", "format": "standard"}], "licence": "GPL-3.0", "home": "https://de.test/"},
    {"id": "fr", "title": "French", "about": "French ads", "group": "regional", "on": false, "languages": ["fr"], "sources": [{"url": "https://fr.test/fr.txt", "format": "standard"}], "licence": "GPL-3.0", "home": "https://fr.test/"},
    {"id": "cookies", "title": "Cookie notices", "about": "Notices", "group": "annoyances", "on": false, "sources": [{"url": "https://cookies.test/c.txt", "format": "standard"}], "licence": "GPL-3.0", "home": "https://cookies.test/"},
    {"id": "urlhaus", "title": "URLhaus filter", "about": "Malware sites", "group": "malware", "on": true, "sources": [{"url": "https://urlhaus.test/u.txt", "format": "standard"}], "licence": "CC0-1.0", "home": "https://urlhaus.test/"},
    {"id": "phishing-filter", "title": "Phishing URL Blocklist", "about": "Phishing sites", "group": "phishing", "on": true, "sources": [{"url": "https://phish.test/p.txt", "format": "standard"}], "licence": "CC-BY-SA-4.0", "home": "https://phish.test/"},
    {"id": "pb-test", "title": "Project B's test list", "about": "One address per hook", "group": "test", "on": false, "sources": [{"url": "https://pb.test/t.txt", "format": "standard"}], "licence": "MIT", "home": "https://pb.test/"}
  ],
  "ignored": [{"brave": "0000", "title": "Some list", "reason": "for the test"}]
}`

// Catalogue is CatalogueJSON, read.
func Catalogue() *blocking.Catalogue {
	c, err := blocking.ParseCatalogue([]byte(CatalogueJSON))
	if err != nil {
		panic(err)
	}
	return c
}
