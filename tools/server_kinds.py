"""Public test pages for the kinds of web server a browser meets, and what a person should
see at each: the page, by words it shows, or our error page, by words of its title. The
drivers' web-server-kinds parts open each (tools/drive_android.py, tools/drive_first_app.py).

badssl.com for TLS and certificates, httpbin.org for redirects, encodings, a slow first byte
and a streamed body, Cloudflare's QUIC page for HTTP/2 and 3, and robu.in, whose optional
client-certificate request our engine once refused (ERR_SSL_CLIENT_AUTH_CERT_NEEDED,
2026-11-21). Plain http and other ports aren't here: office networks pass 443 only. A public
page can be down, so a driver tries a miss once more and names the site.
"""

SERVER_KINDS = [
    ("https://client.badssl.com/", "page", "No required SSL certificate"),
    ("https://client-cert-missing.badssl.com/", "page", "No required SSL certificate"),
    ("https://expired.badssl.com/", "error", "certificate has expired"),
    ("https://self-signed.badssl.com/", "error", "self-signed"),
    ("https://wrong.host.badssl.com/", "error", "is for another site"),
    ("https://untrusted-root.badssl.com/", "error", "certificate"),
    ("https://incomplete-chain.badssl.com/", "page", "incomplete-chain.badssl.com"),
    ("https://mixed.badssl.com/", "page", "mixed.badssl.com"),
    ("https://hsts.badssl.com/", "page", "hsts.badssl.com"),
    ("https://cloudflare-quic.com/", "page", "QUIC"),
    ("https://httpbin.org/redirect/5", "page", "httpbin.org/get"),
    ("https://httpbin.org/gzip", "page", '"gzipped": true'),
    ("https://httpbin.org/brotli", "page", '"brotli": true'),
    ("https://httpbin.org/delay/3", "page", "httpbin.org/delay/3"),
    ("https://httpbin.org/stream/5", "page", '"id":4'),
    ("https://robu.in/", "page", "robu"),
]
