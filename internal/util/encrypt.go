package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"strconv"
)

// encrypt/decrypt key
var encdecKey []byte

func init() {
	uintMac := MacUInt64()
	encdecKey = []byte(strconv.FormatUint(uintMac, 8)) // unique in same mac address
}

// Encrypt encrypts input byte slice
func Encrypt(input []byte) ([]byte, error) {
	c, err := aes.NewCipher(encdecKey)
	if nil != err {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if nil != err {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, input, nil), nil
}

// Decrypt decrypts input byte slice
func Decrypt(input []byte) ([]byte, error) {
	c, err := aes.NewCipher(encdecKey)
	if nil != err {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if nil != err {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(input) < nonceSize {
		return nil, fmt.Errorf("given bad data")
	}

	nonce, input := input[:nonceSize], input[nonceSize:]
	return gcm.Open(nil, nonce, input, nil)
}
