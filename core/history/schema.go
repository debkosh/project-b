package history

// tables are history's tables (docs/store.md), and triggers what keeps visits, typed, sites
// and the trigram index in step with history's rows. Go writes low_url and low_title,
// lowercased as a search compares them, since SQLite's lower() knows only ASCII; bare and
// rank are made from them. Only web pages are kept, so every address starts http:// or
// https://.
const tables = `
CREATE TABLE history (
  page INTEGER PRIMARY KEY,
  id TEXT NOT NULL UNIQUE,
  url TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  visits INTEGER NOT NULL,
  last_visit INTEGER NOT NULL,
  edited INTEGER NOT NULL,
  low_url TEXT NOT NULL,
  low_title TEXT NOT NULL,
  bare TEXT AS (CASE
    WHEN low_url GLOB 'https://www.*' THEN substr(low_url, 13)
    WHEN low_url GLOB 'https://*' THEN substr(low_url, 9)
    WHEN low_url GLOB 'http://www.*' THEN substr(low_url, 12)
    WHEN low_url GLOB 'http://*' THEN substr(low_url, 8)
    ELSE low_url END) VIRTUAL,
  rank INTEGER AS (min(visits, 50)) VIRTUAL
);
CREATE INDEX history_rank ON history (rank DESC, last_visit DESC, url);
CREATE INDEX history_bare ON history (bare);
CREATE INDEX history_last ON history (last_visit DESC, url);

-- Each visit made here. Local only: it never syncs.
CREATE TABLE visits (page INTEGER NOT NULL, time INTEGER NOT NULL);
CREATE INDEX visits_page ON visits (page);

-- The addresses reached from the address field.
CREATE TABLE typed (
  url TEXT PRIMARY KEY,
  id TEXT NOT NULL UNIQUE,
  count INTEGER NOT NULL,
  last INTEGER NOT NULL,
  edited INTEGER NOT NULL
) WITHOUT ROWID;

-- Every address and title by its three-letter pieces, for a search that holds three or more.
CREATE VIRTUAL TABLE history_tri USING fts5 (low_url, low_title, content = 'history', content_rowid = 'page', tokenize = 'trigram');

-- One row per site, a host without www., as TopSites counts them: its pages' visits added
-- up, and its page, the one visited most, then latest, then the lowest address.
CREATE TABLE sites (
  host TEXT PRIMARY KEY,
  visits INTEGER NOT NULL,
  url TEXT NOT NULL,
  page_visits INTEGER NOT NULL,
  page_last INTEGER NOT NULL
) WITHOUT ROWID;
CREATE INDEX sites_top ON sites (visits DESC, page_last DESC, url);
`

const triggers = `
CREATE TRIGGER history_gone AFTER DELETE ON history BEGIN
  DELETE FROM visits WHERE page = old.page;
  DELETE FROM typed WHERE url = old.url;
END;

CREATE TRIGGER history_tri_in AFTER INSERT ON history BEGIN
  INSERT INTO history_tri (rowid, low_url, low_title) VALUES (new.page, new.low_url, new.low_title);
END;

CREATE TRIGGER history_tri_out AFTER DELETE ON history BEGIN
  INSERT INTO history_tri (history_tri, rowid, low_url, low_title) VALUES ('delete', old.page, old.low_url, old.low_title);
END;

CREATE TRIGGER history_tri_title AFTER UPDATE OF low_title ON history WHEN old.low_title IS NOT new.low_title BEGIN
  INSERT INTO history_tri (history_tri, rowid, low_url, low_title) VALUES ('delete', old.page, old.low_url, old.low_title);
  INSERT INTO history_tri (rowid, low_url, low_title) VALUES (new.page, new.low_url, new.low_title);
END;

CREATE TRIGGER sites_in AFTER INSERT ON history BEGIN
  INSERT INTO sites VALUES (substr(new.bare, 1, instr(new.bare || '/', '/') - 1), new.visits, new.url, new.visits, new.last_visit)
  ON CONFLICT (host) DO UPDATE SET
    visits = visits + excluded.visits,
    url = iif(` + betterPage + `, excluded.url, url),
    page_last = iif(` + betterPage + `, excluded.page_last, page_last),
    page_visits = iif(` + betterPage + `, excluded.page_visits, page_visits);
END;

-- Visits only go up, so a page visited stays its site's page or takes its place.
CREATE TRIGGER sites_visited AFTER UPDATE OF visits, last_visit ON history BEGIN
  UPDATE sites SET
    visits = visits + new.visits - old.visits,
    url = iif(` + betterNew + `, new.url, url),
    page_last = iif(` + betterNew + `, new.last_visit, page_last),
    page_visits = iif(` + betterNew + `, new.visits, page_visits)
  WHERE host = substr(new.bare, 1, instr(new.bare || '/', '/') - 1);
END;

-- A page taken out: its visits leave the site, and a site left with none goes; one that
-- lost its page picks it again.
CREATE TRIGGER sites_out AFTER DELETE ON history BEGIN
  UPDATE sites SET visits = visits - old.visits WHERE host = substr(old.bare, 1, instr(old.bare || '/', '/') - 1);
  DELETE FROM sites WHERE host = substr(old.bare, 1, instr(old.bare || '/', '/') - 1) AND visits <= 0;
  UPDATE sites SET (url, page_visits, page_last) = (SELECT h.url, h.visits, h.last_visit FROM history h
      WHERE h.bare = sites.host OR h.bare >= sites.host || '/' AND h.bare < sites.host || '0'
      ORDER BY h.visits DESC, h.last_visit DESC, h.url LIMIT 1)
    WHERE host = substr(old.bare, 1, instr(old.bare || '/', '/') - 1) AND url = old.url;
END;
`

// filled makes the trigram index and the sites from the rows history.jsonl gave, at once,
// before the triggers that keep them in step are made: row by row, 50,000 pages took 6 s.
const filled = `
INSERT INTO history_tri (history_tri) VALUES ('rebuild');
INSERT INTO sites
  SELECT host, total, url, visits, last_visit FROM (
    SELECT substr(bare, 1, instr(bare || '/', '/') - 1) AS host, url, visits, last_visit,
      sum(visits) OVER (PARTITION BY substr(bare, 1, instr(bare || '/', '/') - 1)) AS total,
      row_number() OVER (PARTITION BY substr(bare, 1, instr(bare || '/', '/') - 1) ORDER BY visits DESC, last_visit DESC, url) AS n
    FROM history)
  WHERE n = 1;
`

const betterPage = `excluded.page_visits > page_visits OR excluded.page_visits = page_visits AND
  (excluded.page_last > page_last OR excluded.page_last = page_last AND excluded.url < url)`

const betterNew = `url = new.url OR new.visits > page_visits OR new.visits = page_visits AND
  (new.last_visit > page_last OR new.last_visit = page_last AND new.url < url)`
