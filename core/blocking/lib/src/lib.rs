//! pbblock: Project B's blocker, a plain C interface around Brave's adblock-rust
//! (docs/blocking.md). Go loads it and hands the engine the hot-path functions; nothing
//! in the engine knows of filter lists or of adblock-rust.
//!
//! A blocker holds one adblock engine per group of lists. Loading a group builds the new
//! set aside and swaps it in at once, so a check under way finishes on the rules it began
//! with, and the hot path never waits on a load.

use std::collections::HashSet;
use std::ffi::{c_char, c_int, CStr, CString};
use std::ptr;
use std::slice;
use std::sync::{Arc, Mutex};

use adblock::lists::{FilterSet, ParseOptions};
use adblock::request::Request;
use adblock::resources::Resource;
use adblock::Engine;
use arc_swap::ArcSwap;
use serde::Deserialize;
use serde_json::json;

/// What a request check answers.
pub const ALLOW: c_int = 0;
pub const BLOCK: c_int = 1;
pub const REDIRECT: c_int = 2;

struct Group {
    name: String,
    engine: Engine,
}

/// One blocker: its groups, swapped whole, and the resources each group's engine uses.
pub struct Blocker {
    groups: ArcSwap<Vec<Arc<Group>>>,
    resources: Mutex<Vec<Resource>>,
    // Loads come from one Go goroutine at a time, but nothing here relies on it.
    load: Mutex<()>,
}

impl Blocker {
    fn new() -> Self {
        Blocker {
            groups: ArcSwap::from_pointee(Vec::new()),
            resources: Mutex::new(Vec::new()),
            load: Mutex::new(()),
        }
    }

    fn put(&self, name: &str, group: Option<Group>) {
        let _hold = self.load.lock().unwrap();
        let mut next: Vec<Arc<Group>> = self
            .groups
            .load()
            .iter()
            .filter(|g| g.name != name)
            .cloned()
            .collect();
        if let Some(g) = group {
            next.push(Arc::new(g));
            next.sort_by(|a, b| a.name.cmp(&b.name));
        }
        self.groups.store(Arc::new(next));
    }

    fn load(&self, name: &str, data: &[u8]) -> bool {
        let mut engine = Engine::default();
        if engine.deserialize(data).is_err() {
            return false;
        }
        engine.use_resources(self.resources.lock().unwrap().iter().cloned());
        self.put(name, Some(Group { name: name.to_string(), engine }));
        true
    }

    /// The verdict for one request: blocked if any group blocks it. An exception in one
    /// group doesn't reach another, as in Brave's; an `important` rule ends the check.
    pub fn request(&self, url: &str, source: &str, kind: &str) -> (c_int, Option<String>) {
        let Ok(request) = Request::new(url, source, kind, "GET") else {
            return (ALLOW, None);
        };
        let mut verdict = (ALLOW, None);
        for g in self.groups.load().iter() {
            let r = g.engine.check_network_request(&request);
            if r.should_block() {
                verdict = match r.redirect {
                    Some(to) => (REDIRECT, Some(to)),
                    None => (BLOCK, None),
                };
                if r.important {
                    break;
                }
            }
        }
        verdict
    }

    /// What a page at url hides and runs, from every group.
    pub fn page(&self, url: &str) -> serde_json::Value {
        let mut hide = HashSet::new();
        let mut procedural = HashSet::new();
        let mut exceptions = HashSet::new();
        let mut scripts = Vec::new();
        let mut generichide = false;
        for g in self.groups.load().iter() {
            let r = g.engine.url_cosmetic_resources(url);
            hide.extend(r.hide_selectors);
            procedural.extend(r.procedural_actions);
            exceptions.extend(r.exceptions);
            if !r.injected_script.is_empty() {
                scripts.push(r.injected_script);
            }
            generichide |= r.generichide;
        }
        json!({
            "hide": sorted(hide),
            "procedural": sorted(procedural),
            "exceptions": sorted(exceptions),
            "scripts": scripts,
            "generichide": generichide,
        })
    }

    /// The selectors that hide any of the classes and ids the page's watcher found, less
    /// the page's exceptions.
    pub fn hide(&self, page_url: &str, query: &Query) -> Vec<String> {
        let groups = self.groups.load();
        let mut exceptions = HashSet::new();
        for g in groups.iter() {
            let r = g.engine.url_cosmetic_resources(page_url);
            if r.generichide {
                return Vec::new();
            }
            exceptions.extend(r.exceptions);
        }
        let mut out = Vec::new();
        for g in groups.iter() {
            out.extend(g.engine.hidden_class_id_selectors(
                query.classes.iter(),
                query.ids.iter(),
                &exceptions,
            ));
        }
        out.sort();
        out.dedup();
        out
    }
}

/// The watcher's question: the class and id names it saw.
#[derive(Deserialize, Default)]
pub struct Query {
    #[serde(default)]
    pub classes: Vec<String>,
    #[serde(default)]
    pub ids: Vec<String>,
}

fn sorted(set: HashSet<String>) -> Vec<String> {
    let mut v: Vec<String> = set.into_iter().collect();
    v.sort();
    v
}

/// Builds the serialized engine of one group from its lists' texts.
pub fn compile(lists: &[&[u8]]) -> Vec<u8> {
    let mut set = FilterSet::new(false);
    for text in lists {
        set.add_filter_list(String::from_utf8_lossy(text).into_owned(), ParseOptions::default());
    }
    Engine::new_with_filter_set(set).serialize()
}

// ---- The C interface (pbblock.h) ----

unsafe fn text<'a>(p: *const c_char) -> &'a str {
    if p.is_null() {
        return "";
    }
    CStr::from_ptr(p).to_str().unwrap_or("")
}

fn owned(s: String) -> *mut c_char {
    CString::new(s).map(CString::into_raw).unwrap_or(ptr::null_mut())
}

#[no_mangle]
pub extern "C" fn pbblock_new() -> *mut Blocker {
    Box::into_raw(Box::new(Blocker::new()))
}

/// # Safety
/// b from pbblock_new; lists and lens hold n pointers and lengths.
#[no_mangle]
pub unsafe extern "C" fn pbblock_compile(
    lists: *const *const u8,
    lens: *const usize,
    n: usize,
    out: *mut *mut u8,
    out_len: *mut usize,
) -> c_int {
    if (n > 0 && (lists.is_null() || lens.is_null())) || out.is_null() || out_len.is_null() {
        return 0;
    }
    let texts: Vec<&[u8]> = (0..n)
        .map(|i| slice::from_raw_parts(*lists.add(i), *lens.add(i)))
        .collect();
    let data = compile(&texts).into_boxed_slice();
    *out_len = data.len();
    *out = Box::into_raw(data) as *mut u8;
    1
}

/// # Safety
/// data and len from pbblock_compile.
#[no_mangle]
pub unsafe extern "C" fn pbblock_free_data(data: *mut u8, len: usize) {
    if !data.is_null() {
        drop(Box::from_raw(ptr::slice_from_raw_parts_mut(data, len)));
    }
}

/// # Safety
/// b from pbblock_new; group a C string; data len bytes.
#[no_mangle]
pub unsafe extern "C" fn pbblock_load(
    b: *const Blocker,
    group: *const c_char,
    data: *const u8,
    len: usize,
) -> c_int {
    if b.is_null() || data.is_null() {
        return 0;
    }
    (*b).load(text(group), slice::from_raw_parts(data, len)) as c_int
}

/// # Safety
/// b from pbblock_new; group a C string.
#[no_mangle]
pub unsafe extern "C" fn pbblock_unload(b: *const Blocker, group: *const c_char) {
    if !b.is_null() {
        (*b).put(text(group), None);
    }
}

/// The scriptlets and stand-ins, as adblock-rust's resources JSON; groups loaded after it
/// use them.
///
/// # Safety
/// b from pbblock_new; json len bytes.
#[no_mangle]
pub unsafe extern "C" fn pbblock_resources(b: *const Blocker, json: *const u8, len: usize) -> c_int {
    if b.is_null() || json.is_null() {
        return 0;
    }
    match serde_json::from_slice::<Vec<Resource>>(slice::from_raw_parts(json, len)) {
        Ok(r) => {
            *(*b).resources.lock().unwrap() = r;
            1
        }
        Err(_) => 0,
    }
}

/// Any thread, never blocking. 0 allow, 1 block, 2 redirect with *redirect set (free it
/// with pbblock_free).
///
/// # Safety
/// b from pbblock_new; the strings C strings; redirect may be null.
#[no_mangle]
pub unsafe extern "C" fn pbblock_request(
    b: *const Blocker,
    url: *const c_char,
    source: *const c_char,
    kind: *const c_char,
    redirect: *mut *mut c_char,
) -> c_int {
    if b.is_null() {
        return ALLOW;
    }
    let (verdict, to) = (*b).request(text(url), text(source), text(kind));
    if !redirect.is_null() {
        *redirect = match to {
            Some(to) => owned(to),
            None => ptr::null_mut(),
        };
    }
    verdict
}

/// Any thread. The page's parts as JSON: hide, procedural, exceptions, scripts, generichide.
///
/// # Safety
/// b from pbblock_new; url a C string.
#[no_mangle]
pub unsafe extern "C" fn pbblock_page(b: *const Blocker, url: *const c_char) -> *mut c_char {
    if b.is_null() {
        return ptr::null_mut();
    }
    owned((*b).page(text(url)).to_string())
}

/// Any thread. query is JSON {classes, ids}; the answer a JSON array of selectors.
///
/// # Safety
/// b from pbblock_new; the strings C strings.
#[no_mangle]
pub unsafe extern "C" fn pbblock_hide(
    b: *const Blocker,
    page_url: *const c_char,
    query: *const c_char,
) -> *mut c_char {
    if b.is_null() {
        return ptr::null_mut();
    }
    let q: Query = serde_json::from_str(text(query)).unwrap_or_default();
    owned(serde_json::to_string(&(*b).hide(text(page_url), &q)).unwrap_or_default())
}

/// # Safety
/// s from pbblock_page, pbblock_hide or a redirect.
#[no_mangle]
pub unsafe extern "C" fn pbblock_free(s: *mut c_char) {
    if !s.is_null() {
        drop(CString::from_raw(s));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn blocker(list: &str) -> Blocker {
        let b = Blocker::new();
        assert!(b.load("ads", &compile(&[list.as_bytes()])));
        b
    }

    #[test]
    fn blocks_a_listed_third_party_request() {
        let b = blocker("||ads.test^$third-party\n");
        assert_eq!(b.request("https://ads.test/a.js", "https://site.test/", "script").0, BLOCK);
        assert_eq!(b.request("https://ads.test/a.js", "https://ads.test/", "script").0, ALLOW);
        assert_eq!(b.request("https://cdn.test/a.js", "https://site.test/", "script").0, ALLOW);
    }

    #[test]
    fn an_exception_in_one_group_stays_there() {
        let b = blocker("||ads.test^\n");
        assert!(b.load("unbreak", &compile(&[b"@@||ads.test/ok.js\n"])));
        assert_eq!(b.request("https://ads.test/ok.js", "https://site.test/", "script").0, BLOCK);
    }

    #[test]
    fn hides_by_page_and_by_class() {
        let b = blocker("site.test##.banner\n##.ad-slot\n");
        let page = b.page("https://site.test/");
        assert_eq!(page["hide"], json!([".banner"]));
        let q = Query { classes: vec!["ad-slot".into(), "story".into()], ids: vec![] };
        assert_eq!(b.hide("https://site.test/", &q), vec![".ad-slot".to_string()]);
    }

    #[test]
    fn a_check_under_way_keeps_its_rules_across_a_swap() {
        let b = blocker("||ads.test^\n");
        let held = b.groups.load_full();
        b.put("ads", None);
        assert_eq!(b.request("https://ads.test/", "https://site.test/", "script").0, ALLOW);
        assert_eq!(held.len(), 1);
    }

    #[test]
    fn a_damaged_engine_is_refused() {
        let b = Blocker::new();
        assert!(!b.load("ads", b"not an engine"));
    }
}
