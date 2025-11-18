package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"AreYouOK/config"
)

// AlipayPhoneData 支付宝解密后的手机号数据结构
type AlipayPhoneData struct {
	Mobile string `json:"mobile"`
}

// DecryptAlipayPhone 解密支付宝手机号
// encryptedData: 加密的手机号数据（base64）
// iv: 初始化向量（base64）
func DecryptAlipayPhone(encryptedData, iv string) (string, error) {
	if config.Cfg.AlipayAESKey == "" {
		return "", errors.New("ALIPAY_AES_KEY is not configured")
	}

	// Base64 解码密钥
	key, err := base64.StdEncoding.DecodeString(config.Cfg.AlipayAESKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode AES key: %w", err)
	}

	// Base64 解码加密数据
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted data: %w", err)
	}

	// Base64 解码 IV
	ivBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return "", fmt.Errorf("failed to decode IV: %w", err)
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// 检查 IV 长度
	if len(ivBytes) != aes.BlockSize {
		return "", fmt.Errorf("invalid IV length: expected %d, got %d", aes.BlockSize, len(ivBytes))
	}

	// 创建 CBC 解密器
	mode := cipher.NewCBCDecrypter(block, ivBytes)

	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to unpad: %w", err)
	}

	var phoneData AlipayPhoneData
	if err := json.Unmarshal(plaintext, &phoneData); err != nil {
		return "", fmt.Errorf("failed to parse decrypted data: %w", err)
	}

	if phoneData.Mobile == "" {
		return "", errors.New("mobile number not found in decrypted data")
	}

	return phoneData.Mobile, nil
}

// 去除 pkc7 填充
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("data is empty")
	}

	padding := int(data[len(data)-1])
	if padding > len(data) || padding == 0 {
		return nil, errors.New("invalid padding")
	}

	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}
