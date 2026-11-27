//! cargo run --release --features assembler --example resources -- <uBlock Origin checkout>
//!     <Brave's resources.json> <uBO's scriptlets.json>: the resources JSON the lists name,
//! put together as Brave does (brave/brave-core-crx-packager, lib/adBlockRustUtils.js), on
//! stdout. In order: uBO's redirect resources, read by adblock-rust's own assembler from
//! src/web_accessible_resources and src/js/redirect-resources.js, less google-ima-dai.js as
//! Brave leaves it out; Brave's own, its adblock-resources' dist/resources.json; uBO's
//! scriptlets, from tools/lists/ubo-scriptlets.mjs. .github/workflows/lists.yml runs it, and
//! tools/lists adds our own scriptlets.
//!
//! It fails, and prints nothing, unless every resource goes into adblock-rust's store and,
//! loaded into a blocker as the app loads them, a uBO scriptlet rule gives the page a script
//! and a redirect rule gives a redirect.
use std::path::Path;

use adblock::resources::resource_assembler::assemble_web_accessible_resources;
use adblock::resources::{InMemoryResourceStorage, Resource};

fn fail(msg: String) -> ! {
    eprintln!("resources: {msg}");
    std::process::exit(1);
}

fn read(path: &str) -> Vec<Resource> {
    let b = std::fs::read(path).unwrap_or_else(|e| fail(format!("{path}: {e}")));
    serde_json::from_slice(&b).unwrap_or_else(|e| fail(format!("{path}: {e}")))
}

fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let [ubo, brave, scriptlets] = args.as_slice() else {
        fail("usage: resources <uBlock Origin checkout> <Brave's resources.json> <uBO's scriptlets.json>".into());
    };
    let ubo = Path::new(ubo);
    let redirects: Vec<Resource> = assemble_web_accessible_resources(
        &ubo.join("src/web_accessible_resources"),
        &ubo.join("src/js/redirect-resources.js"),
    )
    .into_iter()
    .filter(|r| r.name != "google-ima-dai.js")
    .collect();
    let brave = read(brave);
    let scriptlets = read(scriptlets);
    eprintln!("resources: {} uBO redirects, {} Brave's, {} uBO scriptlets", redirects.len(), brave.len(), scriptlets.len());
    if redirects.is_empty() || brave.is_empty() || scriptlets.is_empty() {
        fail("a part is empty".into());
    }
    // uBO's trusted-* scriptlets set cookies and rewrite what a page fetches: only a trusted
    // list's rules may use them, as in uBO (pbblock::TRUSTED).
    let scriptlets: Vec<Resource> = scriptlets
        .into_iter()
        .map(|mut r| {
            if r.name.starts_with("trusted-") || r.aliases.iter().any(|a| a.starts_with("trusted-")) {
                r.permission = pbblock::TRUSTED;
            }
            r
        })
        .collect();
    let trusted_count = scriptlets.iter().filter(|r| r.name.starts_with("trusted-")).count();
    if trusted_count == 0 {
        fail("no trusted-* scriptlet found to restrict".into());
    }
    let all: Vec<Resource> = redirects.into_iter().chain(brave).chain(scriptlets).collect();

    let mut store = InMemoryResourceStorage::default();
    for r in &all {
        let name = r.name.clone();
        store.add_resource(r.clone()).unwrap_or_else(|e| fail(format!("{name}: {e:?}")));
    }
    let json = serde_json::to_vec(&all).unwrap();

    let b = unsafe { &*pbblock::pbblock_new() };
    if unsafe { pbblock::pbblock_resources(b, json.as_ptr(), json.len()) } != 1 {
        fail("the blocker refused them".into());
    }
    let list = b"example.com##+js(set-constant, foo, true)\n||example.com/ads.js$script,redirect=noopjs\n";
    let data = pbblock::compile(&[list], &[]);
    if unsafe { pbblock::pbblock_load(b, c"ads".as_ptr(), data.as_ptr(), data.len()) } != 1 {
        fail("the check's list didn't load".into());
    }
    let page = b.page("https://example.com/");
    let script = page["scripts"][0].as_str().unwrap_or("");
    if !script.contains("setConstant(\"foo\", \"true\")") {
        fail(format!("example.com##+js(set-constant, foo, true) gave no script: {page}"));
    }
    match b.request("https://example.com/ads.js", "https://example.com/", "script") {
        (pbblock::REDIRECT, Some(to)) if to.starts_with("data:application/javascript") => {}
        got => fail(format!("||example.com/ads.js$script,redirect=noopjs gave {got:?}")),
    }
    eprintln!("resources: {} in all; a scriptlet rule gave a {}-byte script, a redirect rule a redirect", all.len(), script.len());
    println!("{}", String::from_utf8(json).unwrap());
}
