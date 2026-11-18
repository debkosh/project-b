"""Saved passwords for testdata/home, sealed as each browser seals them; make.sh runs it.

    python3 -I logins.py <Chrome profile> <Firefox profile> <Zen profile>

Python's hashlib and the openssl tool stand in for the browsers' crypto. Chrome · Personal's
Login Data has two of a site's logins, one "never save", one federated and one HTTP auth,
which aren't; Login Data For Account has one. Its key is PBKDF2 of the keychain's "Chrome
Safe Storage", here "test-safe-storage". Firefox's key4.db has no primary password and its
logins.json two logins and one with a realm; Zen's are behind the primary password
"letmein".
"""

import base64
import hashlib
import json
import os
import sqlite3
import subprocess
import sys

chrome, firefox, zen = sys.argv[1:4]


def enc(cipher, key, iv, data):
    return subprocess.run(["openssl", "enc", "-" + cipher, "-K", key.hex(), "-iv", iv.hex()], input=data, capture_output=True, check=True).stdout


# Chrome on the Mac: "v10", then AES-128-CBC, the key PBKDF2-SHA1 of the keychain's secret
# with "saltysalt", 1003 rounds, the IV sixteen spaces.
KEY = hashlib.pbkdf2_hmac("sha1", b"test-safe-storage", b"saltysalt", 1003, 16)


def v10(password):
    return b"v10" + enc("aes-128-cbc", KEY, b" " * 16, password.encode())


SCHEMA = (
    "CREATE TABLE logins (origin_url VARCHAR NOT NULL, action_url VARCHAR, username_value VARCHAR, password_value BLOB, "
    "signon_realm VARCHAR NOT NULL, date_created INTEGER NOT NULL, blacklisted_by_user INTEGER NOT NULL, scheme INTEGER NOT NULL, "
    "date_last_used INTEGER NOT NULL DEFAULT 0, federation_url VARCHAR)"
)


def chrome_logins(path, rows):
    db = sqlite3.connect(path)
    db.execute(SCHEMA)
    db.executemany("INSERT INTO logins VALUES (?, ?, ?, ?, ?, 13370000000000000, ?, ?, 13370100000000000, ?)", rows)
    db.commit()
    db.close()


chrome_logins(os.path.join(chrome, "Login Data"), [
    ("https://example.com/login", "", "alice", v10("s3cret!"), "https://example.com/", 0, 0, None),
    ("https://example.org/", "", "bob", v10("hunter2"), "https://example.org/", 0, 0, ""),
    ("https://never.example.com/", "", "", b"", "https://never.example.com/", 1, 0, None),
    ("https://fed.example.com/", "", "carol", b"", "federation://fed.example.com/idp.example.com", 0, 0, "https://idp.example.com"),
    ("https://auth.example.com/", "", "dave", v10("basic"), "https://auth.example.com/ Realm", 0, 1, None),
])
chrome_logins(os.path.join(chrome, "Login Data For Account"), [
    ("https://account.example.com/", "", "erin", v10("synced!"), "https://account.example.com/", 0, 0, None),
])


# ASN.1 DER, enough for NSS's.
def tlv(tag, body):
    n = len(body)
    if n < 128:
        length = bytes([n])
    else:
        size = n.to_bytes((n.bit_length() + 7) // 8, "big")
        length = bytes([0x80 | len(size)]) + size
    return bytes([tag]) + length + body


def seq(*parts):
    return tlv(0x30, b"".join(parts))


def octets(b):
    return tlv(0x04, b)


def integer(i):
    return tlv(0x02, i.to_bytes(max(1, (i.bit_length() + 8) // 8), "big"))


def oid(dotted):
    parts = [int(x) for x in dotted.split(".")]
    out = bytes([parts[0] * 40 + parts[1]])
    for p in parts[2:]:
        chunk = [p & 0x7F]
        p >>= 7
        while p:
            chunk.insert(0, 0x80 | (p & 0x7F))
            p >>= 7
        out += bytes(chunk)
    return tlv(0x06, out)


# key4.db's items: PBES2, PBKDF2 with HMAC-SHA256 of SHA1(global salt + primary password),
# then AES-256-CBC with NSS's 14-byte IV after 04 0e.
def pbes2(global_salt, primary, salt, plain):
    k = hashlib.sha1(global_salt + primary.encode()).digest()
    aes_key = hashlib.pbkdf2_hmac("sha256", k, salt, 1000, 32)
    iv14 = bytes(range(14))
    data = enc("aes-256-cbc", aes_key, b"\x04\x0e" + iv14, plain)
    algorithm = seq(oid("1.2.840.113549.1.5.13"), seq(
        seq(oid("1.2.840.113549.1.5.12"), seq(octets(salt), integer(1000), integer(32), seq(oid("1.2.840.113549.2.9")))),
        seq(oid("2.16.840.1.101.3.4.1.42"), octets(iv14))))
    return seq(algorithm, octets(data))


def firefox_logins(folder, primary, entries):
    global_salt = b"global-salt-0123456789"
    des_key = bytes(range(1, 25))
    key_id = bytes([0xF8] + [0] * 14 + [1])
    path = os.path.join(folder, "key4.db")
    if os.path.exists(path):
        os.remove(path)
    db = sqlite3.connect(path)
    db.execute("CREATE TABLE metaData (id PRIMARY KEY UNIQUE ON CONFLICT REPLACE, item1, item2)")
    db.execute("CREATE TABLE nssPrivate (id PRIMARY KEY UNIQUE ON CONFLICT ABORT, a11, a102)")
    db.execute("INSERT INTO metaData VALUES ('password', ?, ?)", (global_salt, pbes2(global_salt, primary, b"check-salt-0123456789abcdefghijk", b"password-check")))
    db.execute("INSERT INTO nssPrivate VALUES (1, ?, ?)", (pbes2(global_salt, primary, b"key-salt-0123456789abcdefghijklm", des_key), key_id))
    db.commit()
    db.close()

    # logins.json's values: the key's id, 3DES-CBC with its IV, the sealed bytes.
    def sdr(text):
        iv = b"iv-8byte"
        sealed = enc("des-ede3-cbc", des_key, iv, text.encode())
        return base64.b64encode(seq(octets(key_id), seq(oid("1.2.840.113549.3.7"), octets(iv)), octets(sealed))).decode()

    out = {"nextId": len(entries) + 1, "logins": []}
    for i, (host, realm, user, password) in enumerate(entries):
        out["logins"].append({
            "id": i + 1, "hostname": host, "httpRealm": realm, "formSubmitURL": None if realm else host,
            "encryptedUsername": sdr(user), "encryptedPassword": sdr(password), "encType": 1,
            "timeCreated": 1700000000000, "timeLastUsed": 1700000100000,
        })
    with open(os.path.join(folder, "logins.json"), "w") as f:
        json.dump(out, f)


firefox_logins(firefox, "", [
    ("https://example.net", None, "frank", "correct horse"),
    ("https://example.com", None, "alice", "s3cret!"),
    ("https://auth.example.net", "Realm", "gina", "basic"),
])
firefox_logins(zen, "letmein", [("https://zen.example.com", None, "zed", "z3n")])
