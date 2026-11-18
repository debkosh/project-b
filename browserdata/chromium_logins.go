package browserdata

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"errors"
	"path/filepath"
	"strconv"
)

// A Chromium browser on the Mac keeps saved passwords in its profile's Login Data (and,
// for those saved to the person's account, Login Data For Account), SQLite, each password
// "v10" then AES-128-CBC. The key is PBKDF2-SHA1 of a random secret the browser keeps in the
// login keychain as "<name> Safe Storage", with the salt "saltysalt", 1003 rounds; the IV
// is sixteen spaces. Firefox's own importer reads them so (ChromeMacOSLoginCrypto.sys.mjs).

var chromiumLoginFiles = []string{"Login Data", "Login Data For Account"}

// A site's login: neither "never save" nor federated (signed in with another site), and
// a form's or a page's own (scheme 0 is a form; HTTP auth logins have a realm the store
// doesn't keep).
const chromiumLoginsWhere = `FROM logins WHERE blacklisted_by_user = 0 AND scheme = 0 AND (federation_url IS NULL OR federation_url = '')`

func chromiumCountLogins(dir string) (int, error) {
	n := 0
	for _, f := range chromiumLoginFiles {
		path := filepath.Join(dir, f)
		if !exists(path) {
			continue
		}
		if err := query(path, `SELECT COUNT(*) `+chromiumLoginsWhere, func(r row) { n += int(r.int(0)) }); err != nil {
			return n, err
		}
	}
	return n, nil
}

func chromiumLogins(dir string, k kind, keys Keys) ([]Login, error) {
	type sealed struct {
		origin, username  string
		password          []byte
		created, lastUsed int64
	}
	var rows []sealed
	for _, f := range chromiumLoginFiles {
		path := filepath.Join(dir, f)
		if !exists(path) {
			continue
		}
		err := query(path, `SELECT origin_url, username_value, password_value, date_created, date_last_used `+chromiumLoginsWhere, func(r row) {
			rows = append(rows, sealed{r.text(0), r.text(1), r.blob(2), r.int(3), r.int(4)})
		})
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	key, err := chromiumKey(k, keys)
	if err != nil {
		return nil, err
	}
	var out []Login
	var bad int
	for _, s := range rows {
		password, err := chromiumDecrypt(key, s.password)
		if err != nil {
			bad++
			continue
		}
		out = append(out, Login{
			Origin: s.origin, Username: s.username, Password: password,
			Created: webkitTime(strconv.FormatInt(s.created, 10)), LastUsed: webkitTime(strconv.FormatInt(s.lastUsed, 10)),
		})
	}
	if bad > 0 && len(out) == 0 {
		return nil, errors.New("browserdata: the keychain's key opens none of the passwords")
	}
	return out, nil
}

// chromiumKey is the AES key, from the first of the browser's keychain names the keychain
// has.
func chromiumKey(k kind, keys Keys) ([]byte, error) {
	if keys.Secret == nil {
		return nil, ErrNoSecret
	}
	names := k.keychain
	if len(names) == 0 {
		names = []string{"Chromium"}
	}
	for _, name := range names {
		secret, err := keys.Secret(name+" Safe Storage", name)
		if err != nil || len(secret) == 0 {
			continue
		}
		return pbkdf2.Key(sha1.New, string(secret), []byte("saltysalt"), 1003, 16)
	}
	return nil, ErrNoSecret
}

// chromiumDecrypt opens one "v10" password.
func chromiumDecrypt(key, sealed []byte) (string, error) {
	data, ok := bytes.CutPrefix(sealed, []byte("v10"))
	if !ok || len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return "", errors.New("browserdata: not a v10 password")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, bytes.Repeat([]byte(" "), aes.BlockSize)).CryptBlocks(out, data)
	out, err = unpad(out, aes.BlockSize)
	return string(out), err
}

// unpad takes PKCS#7 padding off.
func unpad(b []byte, size int) ([]byte, error) {
	if len(b) == 0 {
		return nil, errors.New("browserdata: nothing to unpad")
	}
	n := int(b[len(b)-1])
	if n == 0 || n > size || n > len(b) || !bytes.Equal(b[len(b)-n:], bytes.Repeat([]byte{byte(n)}, n)) {
		return nil, errors.New("browserdata: bad padding: the key isn't this one")
	}
	return b[:len(b)-n], nil
}
