package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"

	"AreYouOK/config"
)

// 需要之后研究下 aes 加密是如何实现的
//返回对应的 encryptPhone 值

var errInvalidCipherText = errors.New("invalid ciphertext payload")

func EncryptPhone(plain string) (encoded string, err error) {
	key := []byte(config.Cfg.EncryptionKey)

	block, err := aes.NewCipher(key)
	// 加密的块
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)

	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)

	raw := append(nonce, ciphertext...)
	encoded = base64.StdEncoding.EncodeToString(raw)

	return encoded, nil
}

func DecryptPhone(raw []byte) (string, error) {
	key := []byte(config.Cfg.EncryptionKey)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errInvalidCipherText
	}

	nonce := raw[:nonceSize]
	ciphertext := raw[nonceSize:]

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}

// maskPhone 脱敏手机号，格式：138****0000
func MaskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}