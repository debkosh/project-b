package browserdata

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Firefox keeps saved passwords in its profile's logins.json, each username and password
// sealed by NSS's "secret decoder ring" with a key kept in key4.db (SQLite): the key itself
// is sealed under the primary password, "" when the person set none. key4.db's metaData
// row "password" holds the global salt and a sealed "password-check" that says whether a
// primary password is the one; nssPrivate holds the sealed key. Sealing is PBES2 (PBKDF2
// with HMAC-SHA256, then AES-256-CBC) in a profile made since Firefox 72, or the older
// PKCS#12 SHA1-and-3DES; the logins themselves are 3DES-CBC, or AES-256-CBC in newer ones.

var (
	oidPBES2          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidPBKDF2         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidSHA1And3DES    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 12, 5, 1, 3}
	oidAES256CBC      = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
	oidDES3CBC        = asn1.ObjectIdentifier{1, 2, 840, 113549, 3, 7}
	firefoxPrivateKey = []byte{0xf8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
)

// sealedItem is a value sealed under the primary password: how, and the sealed bytes.
type sealedItem struct {
	Algorithm struct {
		ID     asn1.ObjectIdentifier
		Params asn1.RawValue
	}
	Data []byte
}

type pbes2Params struct {
	KDF struct {
		ID     asn1.ObjectIdentifier
		Params struct {
			Salt       []byte
			Iterations int
			KeyLength  int `asn1:"optional"`
			PRF        asn1.RawValue `asn1:"optional"`
		}
	}
	Cipher struct {
		ID asn1.ObjectIdentifier
		IV []byte
	}
}

type sha1And3DESParams struct {
	Salt       []byte
	Iterations int
}

// open unseals a key4.db item with the primary password and the global salt.
func (s sealedItem) open(globalSalt []byte, primary string) ([]byte, error) {
	h := sha1.Sum(append(append([]byte(nil), globalSalt...), primary...))
	switch {
	case s.Algorithm.ID.Equal(oidPBES2):
		var p pbes2Params
		if _, err := asn1.Unmarshal(s.Algorithm.Params.FullBytes, &p); err != nil {
			return nil, err
		}
		if !p.KDF.ID.Equal(oidPBKDF2) || !p.Cipher.ID.Equal(oidAES256CBC) {
			return nil, errors.New("browserdata: key4.db seals with something unknown")
		}
		key, err := pbkdf2.Key(sha256.New, string(h[:]), p.KDF.Params.Salt, p.KDF.Params.Iterations, 32)
		if err != nil {
			return nil, err
		}
		// NSS keeps 14 of the IV's 16 bytes; the first two are always 04 0e.
		iv := p.Cipher.IV
		if len(iv) == 14 {
			iv = append([]byte{0x04, 0x0e}, iv...)
		}
		return cbcOpen(aes.NewCipher, key, iv, s.Data)
	case s.Algorithm.ID.Equal(oidSHA1And3DES):
		var p sha1And3DESParams
		if _, err := asn1.Unmarshal(s.Algorithm.Params.FullBytes, &p); err != nil {
			return nil, err
		}
		key := sha1And3DESKey(h[:], p.Salt)
		return cbcOpen(des.NewTripleDESCipher, key[:24], key[32:], s.Data)
	}
	return nil, errors.New("browserdata: key4.db seals with something unknown")
}

// sha1And3DESKey is NSS's PKCS#12-style key for SHA1-and-3DES: 40 bytes, the first 24 the
// key, the last 8 the IV.
func sha1And3DESKey(hashedPassword, salt []byte) []byte {
	pes := append(append([]byte(nil), salt...), make([]byte, max(0, 20-len(salt)))...)
	chp := sha1.Sum(append(append([]byte(nil), hashedPassword...), salt...))
	mac := func(data ...[]byte) []byte {
		m := hmac.New(sha1.New, chp[:])
		for _, d := range data {
			m.Write(d)
		}
		return m.Sum(nil)
	}
	k1 := mac(pes, salt)
	tk := mac(pes)
	k2 := mac(tk, salt)
	return append(k1, k2...)
}

// cbcOpen decrypts CBC and takes the padding off.
func cbcOpen(newCipher func([]byte) (cipher.Block, error), key, iv, data []byte) ([]byte, error) {
	block, err := newCipher(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != block.BlockSize() || len(data) == 0 || len(data)%block.BlockSize() != 0 {
		return nil, errors.New("browserdata: a sealed value of the wrong size")
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, data)
	return unpad(out, block.BlockSize())
}

// firefoxKey is the key logins.json is sealed with, from key4.db, unlocked by the primary
// password: ErrPrimaryPassword when it isn't the one.
func firefoxKey(dir, primary string) ([]byte, error) {
	path := filepath.Join(dir, "key4.db")
	if !exists(path) {
		return nil, errors.New("browserdata: no key4.db")
	}
	var globalSalt, check []byte
	if err := query(path, `SELECT item1, item2 FROM metaData WHERE id = 'password'`, func(r row) {
		globalSalt, check = r.blob(0), r.blob(1)
	}); err != nil {
		return nil, err
	}
	var item sealedItem
	if _, err := asn1.Unmarshal(check, &item); err != nil {
		return nil, err
	}
	if clear, err := item.open(globalSalt, primary); err != nil || !bytes.Equal(clear, []byte("password-check")) {
		return nil, ErrPrimaryPassword
	}
	var sealed []byte
	if err := query(path, `SELECT a11, a102 FROM nssPrivate`, func(r row) {
		if bytes.Equal(r.blob(1), firefoxPrivateKey) {
			sealed = r.blob(0)
		}
	}); err != nil {
		return nil, err
	}
	if sealed == nil {
		return nil, errors.New("browserdata: key4.db holds no key")
	}
	if _, err := asn1.Unmarshal(sealed, &item); err != nil {
		return nil, err
	}
	return item.open(globalSalt, primary)
}

// sdrSealed is a username or password in logins.json: the key's id, how, and the bytes.
type sdrSealed struct {
	KeyID     []byte
	Algorithm struct {
		ID asn1.ObjectIdentifier
		IV []byte
	}
	Data []byte
}

// sdrOpen opens one of logins.json's sealed values with the key.
func sdrOpen(key []byte, value string) (string, error) {
	der, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	var s sdrSealed
	if _, err := asn1.Unmarshal(der, &s); err != nil {
		return "", err
	}
	var clear []byte
	switch {
	case s.Algorithm.ID.Equal(oidDES3CBC) && len(key) >= 24:
		clear, err = cbcOpen(des.NewTripleDESCipher, key[:24], s.Algorithm.IV, s.Data)
	case s.Algorithm.ID.Equal(oidAES256CBC) && len(key) >= 32:
		clear, err = cbcOpen(aes.NewCipher, key[:32], s.Algorithm.IV, s.Data)
	default:
		err = errors.New("browserdata: a login sealed with something unknown")
	}
	return string(clear), err
}

type firefoxLoginsFile struct {
	Logins []struct {
		Hostname          string `json:"hostname"`
		HTTPRealm         string `json:"httpRealm"`
		EncryptedUsername string `json:"encryptedUsername"`
		EncryptedPassword string `json:"encryptedPassword"`
		TimeCreated       int64  `json:"timeCreated"`
		TimeLastUsed      int64  `json:"timeLastUsed"`
	} `json:"logins"`
}

func readFirefoxLogins(dir string) (firefoxLoginsFile, error) {
	var f firefoxLoginsFile
	b, err := os.ReadFile(filepath.Join(dir, "logins.json"))
	if os.IsNotExist(err) {
		return f, nil
	}
	if err == nil {
		err = json.Unmarshal(b, &f)
	}
	return f, err
}

func firefoxCountLogins(dir string) (int, error) {
	f, err := readFirefoxLogins(dir)
	n := 0
	for _, l := range f.Logins {
		if l.HTTPRealm == "" {
			n++
		}
	}
	return n, err
}

func firefoxLogins(dir, primary string) ([]Login, error) {
	f, err := readFirefoxLogins(dir)
	if err != nil || len(f.Logins) == 0 {
		return nil, err
	}
	key, err := firefoxKey(dir, primary)
	if err != nil {
		return nil, err
	}
	var out []Login
	for _, l := range f.Logins {
		// An HTTP auth login has a realm, which the store doesn't keep.
		if l.HTTPRealm != "" {
			continue
		}
		username, err1 := sdrOpen(key, l.EncryptedUsername)
		password, err2 := sdrOpen(key, l.EncryptedPassword)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, Login{Origin: l.Hostname, Username: username, Password: password, Created: millis(l.TimeCreated), LastUsed: millis(l.TimeLastUsed)})
	}
	return out, nil
}

func millis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}
