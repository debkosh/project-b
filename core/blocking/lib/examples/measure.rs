//! cargo run --release --example measure -- <list.txt>...: compile time, size, check time.
use std::time::Instant;
fn main() {
    let texts: Vec<Vec<u8>> = std::env::args().skip(1).map(|p| std::fs::read(p).unwrap()).collect();
    let refs: Vec<&[u8]> = texts.iter().map(|t| t.as_slice()).collect();
    let t = Instant::now();
    let data = pbblock::compile(&refs, &[]);
    println!("compile {:?}, serialized {} KB", t.elapsed(), data.len() / 1024);
    let b = unsafe { &*pbblock::pbblock_new() };
    let t = Instant::now();
    assert!(unsafe { pbblock::pbblock_load(b, c"ads".as_ptr(), data.as_ptr(), data.len()) } == 1);
    println!("load {:?}", t.elapsed());
    let urls = ["https://www.googletagmanager.com/gtm.js?id=X", "https://cdn.site.test/app.js",
        "https://securepubads.g.doubleclick.net/tag/js/gpt.js", "https://images.site.test/a/b/c.jpg"];
    let n = 100_000;
    let t = Instant::now();
    let mut blocked = 0;
    for i in 0..n { if b.request(urls[i % 4], "https://news.site.test/", "script").0 != 0 { blocked += 1; } }
    println!("check {:?} each ({} of {} blocked)", t.elapsed() / n as u32, blocked, n);
    let t = Instant::now();
    for _ in 0..1000 { let _ = b.page("https://news.site.test/story"); }
    println!("page {:?} each", t.elapsed() / 1000);
}
