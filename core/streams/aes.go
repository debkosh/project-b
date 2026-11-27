package streams

import (
	"crypto/aes"
	"crypto/cipher"
)

// unlock opens a piece locked whole with AES-128 in CBC, as HLS's METHOD=AES-128 locks one,
// with the key the playlist names and the piece's IV, and takes off its padding (PKCS#7).
// It works in place and returns what is left.
func unlock(data, key, iv []byte) ([]byte, error) {
	if len(key) != 16 || len(iv) != 16 || len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, ErrFormat
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrFormat
	}
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(data, data)
	pad := int(data[len(data)-1])
	if pad == 0 || pad > aes.BlockSize || pad > len(data) {
		return nil, ErrFormat
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, ErrFormat
		}
	}
	return data[:len(data)-pad], nil
}
