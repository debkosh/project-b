//! cargo run --release --example check -- <list.txt>...: that each list parses with this
//! library, as tools/lists checks each list before publishing it (docs/blocking.md, "Building
//! and publishing them"). For each, its count of network and cosmetic rules; a list that has
//! lines other than comments but no rule, or whose engine doesn't load, fails it (exit 1).
use adblock::lists::{parse_filter, ParseOptions, ParsedLine};

fn main() {
    let paths: Vec<String> = std::env::args().skip(1).collect();
    if paths.is_empty() {
        eprintln!("usage: check <list.txt>...");
        std::process::exit(2);
    }
    let mut failed = false;
    for path in &paths {
        let bytes = match std::fs::read(path) {
            Ok(b) => b,
            Err(e) => {
                println!("FAIL {path}: {e}");
                failed = true;
                continue;
            }
        };
        let text = String::from_utf8_lossy(&bytes);
        let (mut network, mut cosmetic, mut other) = (0, 0, 0);
        for line in text.lines() {
            let l = line.trim();
            if l.is_empty() || l.starts_with('!') || l.starts_with('[') || l.starts_with("# ") || l == "#" {
                continue;
            }
            match parse_filter(l, false, ParseOptions::default()) {
                Ok(ParsedLine::Network(_)) => network += 1,
                Ok(ParsedLine::Cosmetic(_)) => cosmetic += 1,
                Err(_) => other += 1,
            }
        }
        // The engine the app would compile, and that it loads.
        let data = pbblock::compile(&[&bytes], &[]);
        let b = unsafe { &*pbblock::pbblock_new() };
        let loads = unsafe { pbblock::pbblock_load(b, c"check".as_ptr(), data.as_ptr(), data.len()) } == 1;
        let line = format!("{path}: {network} network, {cosmetic} cosmetic, {other} not taken");
        if !loads {
            println!("FAIL {line}; its engine doesn't load");
            failed = true;
        } else if network + cosmetic == 0 && other > 0 {
            println!("FAIL {line}; no rule in a list that isn't empty");
            failed = true;
        } else {
            println!("ok   {line}");
        }
    }
    std::process::exit(if failed { 1 } else { 0 });
}
