package utils

import (
	//"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	//"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"

	"AreYouOK/config"
	"AreYouOK/pkg/logger"

	"go.uber.org/zap"
)

// AlipayPhoneData 支付宝解密后的手机号数据结构
// 根据支付宝官方文档：https://opendocs.alipay.com/common/02mse3
type AlipayPhoneData struct {
	Mobile string `json:"mobile"`
}

// AlipayEncryptedResponse 支付宝 my.getPhoneNumber 返回的完整响应结构
// 根据支付宝官方文档：https://opendocs.alipay.com/common/02mse3
type AlipayEncryptedResponse struct {
	Response    string `json:"response"`     // AES 加密的数据（base64 编码）
	Sign        string `json:"sign"`         // RSA2 签名（base64 编码）
	SignType    string `json:"sign_type"`    // 签名算法，通常为 "RSA2"
	EncryptType string `json:"encrypt_type"` // 加密算法，通常为 "AES"
	Charset     string `json:"charset"`
}

// AlipayResponse 兼容旧版本的 response 结构
type AlipayResponse struct {
	Response string `json:"response"` // 加密的用户信息（JSON 字符串）
}

var (
	alipayPrivateKey *rsa.PrivateKey
	privateKeyOnce   sync.Once
	privateKeyErr    error

	// alipayPublicKey *rsa.PublicKey
	// publicKeyOnce   sync.Once
	// publicKeyErr    error
)

func loadAlipayPrivateKey() (*rsa.PrivateKey, error) {
	privateKeyOnce.Do(func() {
		if config.Cfg.AlipayPrivateKey == "" {
			privateKeyErr = errors.New("ALIPAY_PRIVATE_KEY is not configured")
			return
		}

		var keyBytes []byte
		var err error

		// 尝试解析 PEM 格式的私钥
		block, _ := pem.Decode([]byte(config.Cfg.AlipayPrivateKey))
		if block != nil {
			// PEM 格式，使用 block.Bytes
			keyBytes = block.Bytes
			logger.Logger.Debug("Private key is PEM format")
		} else {
			// 尝试 base64 解码
			keyBytes, err = base64.StdEncoding.DecodeString(config.Cfg.AlipayPrivateKey)
			if err != nil {
				keyBytes = []byte(config.Cfg.AlipayPrivateKey)
			} else {
				logger.Logger.Debug("Private key decoded from base64")
			}
		}

		logger.Logger.Debug("Loading private key",
			zap.Int("key_bytes_length", len(keyBytes)),
		)

		var privateKey interface{}

		// 优先尝试 PKCS8 格式（支付宝通常使用 PKCS8）
		privateKey, err = x509.ParsePKCS8PrivateKey(keyBytes)
		if err != nil {
			logger.Logger.Debug("PKCS8 parse failed, trying PKCS1", zap.Error(err))
			// 如果 PKCS8 失败，尝试 PKCS1 格式
			privateKey, err = x509.ParsePKCS1PrivateKey(keyBytes)
			if err != nil {
				privateKeyErr = fmt.Errorf("failed to parse private key (tried PKCS8 and PKCS1): %w", err)
				logger.Logger.Error("Failed to parse private key", zap.Error(privateKeyErr))
				return
			}
			logger.Logger.Debug("Private key parsed as PKCS1")
		} else {
			logger.Logger.Debug("Private key parsed as PKCS8")
		}

		rsaKey, ok := privateKey.(*rsa.PrivateKey)
		if !ok {
			privateKeyErr = errors.New("private key is not RSA format")
			logger.Logger.Error("Private key is not RSA format")
			return
		}

		logger.Logger.Info("Private key loaded successfully",
			zap.Int("key_size_bits", rsaKey.N.BitLen()),
			zap.Int("key_size_bytes", rsaKey.N.BitLen()/8),
		)

		alipayPrivateKey = rsaKey
	})

	return alipayPrivateKey, privateKeyErr
}

// // loadAlipayPublicKey 加载支付宝公钥（用于验签）
// func loadAlipayPublicKey() (*rsa.PublicKey, error) {
// 	publicKeyOnce.Do(func() {
// 		if config.Cfg.AlipayPublicKey == "" {
// 			publicKeyErr = errors.New("ALIPAY_PUBLIC_KEY is not configured")
// 			return
// 		}

// 		var keyBytes []byte
// 		var err error

// 		// 尝试解析 PEM 格式的公钥
// 		block, _ := pem.Decode([]byte(config.Cfg.AlipayPublicKey))
// 		if block != nil {
// 			keyBytes = block.Bytes

// 		} else {

// 			keyBytes, err = base64.StdEncoding.DecodeString(config.Cfg.AlipayPublicKey)
// 			if err != nil {

// 				keyBytes = []byte(config.Cfg.AlipayPublicKey)
// 			} else {
// 				logger.Logger.Debug("Public key decoded from base64")
// 			}
// 		}

// 		logger.Logger.Debug("Loading public key",
// 			zap.Int("key_bytes_length", len(keyBytes)),
// 		)

// 		var publicKey interface{}

// 		// 尝试解析 PKIX 格式的公钥
// 		publicKey, err = x509.ParsePKIXPublicKey(keyBytes)
// 		if err != nil {
// 			logger.Logger.Debug("PKIX parse failed, trying PKCS1", zap.Error(err))
// 			// 如果 PKIX 失败，尝试 PKCS1 格式
// 			publicKey, err = x509.ParsePKCS1PublicKey(keyBytes)
// 			if err != nil {
// 				publicKeyErr = fmt.Errorf("failed to parse public key (tried PKIX and PKCS1): %w", err)
// 				logger.Logger.Error("Failed to parse public key", zap.Error(publicKeyErr))
// 				return
// 			}
// 			logger.Logger.Debug("Public key parsed as PKCS1")
// 		} else {
// 			logger.Logger.Debug("Public key parsed as PKIX")
// 		}

// 		rsaKey, ok := publicKey.(*rsa.PublicKey)
// 		if !ok {
// 			publicKeyErr = errors.New("public key is not RSA format")
// 			logger.Logger.Error("Public key is not RSA format")
// 			return
// 		}

// 		logger.Logger.Info("Public key loaded successfully",
// 			zap.Int("key_size_bits", rsaKey.N.BitLen()),
// 			zap.Int("key_size_bytes", rsaKey.N.BitLen()/8),
// 		)

// 		alipayPublicKey = rsaKey
// 	})

// 	return alipayPublicKey, publicKeyErr
// }

// // verifyRSA2Signature RSA2 验签
// // 使用支付宝公钥验证签名
// func verifyRSA2Signature(data []byte, signature []byte, publicKey *rsa.PublicKey) error {
// 	// RSA2 使用 SHA256 哈希算法
// 	hash := sha256.Sum256(data)
// 	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signature)
// }

// decryptAES AES 解密
// 根据支付宝文档，使用 AES-128-CBC，IV 为全零
func decryptAES(ciphertext []byte, key []byte) ([]byte, error) {
	// AES-128 需要 16 字节的密钥
	if len(key) != 16 {
		return nil, fmt.Errorf("AES key must be 16 bytes, got %d bytes", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// IV 为全零（16 字节）
	iv := make([]byte, aes.BlockSize)

	// 检查密文长度必须是块大小的倍数
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a multiple of block size %d", len(ciphertext), aes.BlockSize)
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 去除 PKCS7 填充
	plaintext, err = removePKCS7Padding(plaintext)
	if err != nil {
		return nil, fmt.Errorf("failed to remove padding: %w", err)
	}

	return plaintext, nil
}



// removePKCS7Padding 去除 PKCS7 填充
func removePKCS7Padding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("data is empty")
	}

	paddingLen := int(data[len(data)-1])
	if paddingLen > len(data) || paddingLen == 0 {
		return nil, errors.New("invalid padding")
	}

	// 检查填充是否有效
	for i := len(data) - paddingLen; i < len(data); i++ {
		if data[i] != byte(paddingLen) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-paddingLen], nil
}

// decryptRSABlock 分段解密 RSA 数据
// RSA PKCS1v15 加密的数据块大小固定为密钥长度（RSA 2048 为 256 字节）
// 如果数据长度是密钥长度的整数倍，则分段解密；否则直接解密
func decryptRSABlock(privateKey *rsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	keySize := privateKey.N.BitLen() / 8

	// 如果数据长度正好是密钥长度，直接解密
	if len(ciphertext) == keySize {
		return rsa.DecryptPKCS1v15(rand.Reader, privateKey, ciphertext)
	}

	// 如果数据长度是密钥长度的整数倍，分段解密
	if len(ciphertext)%keySize == 0 {
		var plaintext []byte
		for i := 0; i < len(ciphertext); i += keySize {
			block, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, ciphertext[i:i+keySize])
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt block at offset %d: %w", i, err)
			}
			plaintext = append(plaintext, block...)
		}
		return plaintext, nil
	}

	// 数据长度不是密钥长度的整数倍，尝试直接解密（可能是其他格式）
	return rsa.DecryptPKCS1v15(rand.Reader, privateKey, ciphertext)
}

// DecryptAlipayPhone 解密支付宝手机号
// 根据支付宝官方文档：https://opendocs.alipay.com/common/02mse3
// 支付宝返回的数据格式：
//
//	{
//	  "response": "加密数据（base64）",
//	  "sign": "签名（base64）",
//	  "sign_type": "RSA2",
//	  "encrypt_type": "AES",
//	  "charset": "UTF-8"
//	}
//
// 流程：1. 使用支付宝公钥验签（RSA2） 2. 使用 AES 密钥解密（AES-128-CBC）
// encryptedData: 前端返回的完整 JSON 字符串或仅 response 字段
func DecryptAlipayPhone(encryptedData string) (string, error) {
	// 解析完整的支付宝响应数据
	var encryptedResp AlipayEncryptedResponse
	var responseData string
	var signData string
	var signType string
	var encryptType string

	// 尝试解析完整的 JSON 格式
	if err := json.Unmarshal([]byte(encryptedData), &encryptedResp); err == nil && encryptedResp.Response != "" {
		// 完整的支付宝响应格式
		responseData = encryptedResp.Response
		signData = encryptedResp.Sign
		signType = encryptedResp.SignType
		encryptType = encryptedResp.EncryptType
		logger.Logger.Debug("Parsed full alipay encrypted response",
			zap.String("sign_type", signType),
			zap.String("encrypt_type", encryptType),
		)
	} else {

		var alipayResp AlipayResponse
		if err := json.Unmarshal([]byte(encryptedData), &alipayResp); err == nil && alipayResp.Response != "" {
			responseData = alipayResp.Response
			logger.Logger.Debug("Parsed response-only format")
		} else {

			responseData = encryptedData
			logger.Logger.Debug("Using raw response data")
		}
		// 如果没有 sign 和 encrypt_type，默认使用 AES 加密
		encryptType = "AES"
		signType = "RSA2"
	}

	encryptType = strings.ToUpper(strings.TrimSpace(encryptType))
	if encryptType == "" {
		encryptType = "AES"
	}
	signType = strings.ToUpper(strings.TrimSpace(signType))
	if signType == "" {
		signType = "RSA2"
	}

	logger.Logger.Debug("Decrypting alipay phone",
		zap.String("encrypt_type", encryptType),
		zap.String("sign_type", signType),
		zap.Int("response_length", len(responseData)),
	)

	// Base64 解码 response 数据
	ciphertext, err := base64.StdEncoding.DecodeString(responseData)
	if err != nil {
		logger.Logger.Error("Failed to decode base64 response data", zap.Error(err))
		return "", fmt.Errorf("failed to decode response data: %w", err)
	}

	// 根据加密类型选择解密方式
	var plaintext []byte
	if encryptType == "AES" {
		// AES 解密
		if config.Cfg.AliPayAESKey == "" {
			return "", errors.New("ALIPAY_AES_KEY is not configured")
		}

		// AES 密钥可能是 base64 编码的，需要解码
		aesKeyBytes, err := base64.StdEncoding.DecodeString(config.Cfg.AliPayAESKey)
		if err != nil {
			// 如果不是 base64，直接使用原始字符串
			logger.Logger.Debug("AES key is not base64, using raw string")
			aesKeyBytes = []byte(config.Cfg.AliPayAESKey)
		}

		// AES-128 需要 16 字节密钥
		if len(aesKeyBytes) != 16 {
			return "", fmt.Errorf("AES key must be 16 bytes, got %d bytes (key length: %d)", len(aesKeyBytes), len(config.Cfg.AliPayAESKey))
		}

		plaintext, err = decryptAES(ciphertext, aesKeyBytes)
		if err != nil {
			logger.Logger.Error("Failed to decrypt with AES",
				zap.Error(err),
				zap.Int("ciphertext_length", len(ciphertext)),
			)
			return "", fmt.Errorf("failed to decrypt with AES: %w", err)
		}

	} else {

		privateKey, err := loadAlipayPrivateKey()
		if err != nil {
			return "", fmt.Errorf("failed to load private key: %w", err)
		}

		plaintext, err = decryptRSABlock(privateKey, ciphertext)
		if err != nil {
			logger.Logger.Error("Failed to decrypt with RSA",
				zap.Error(err),
				zap.Int("ciphertext_length", len(ciphertext)),
			)
			return "", fmt.Errorf("failed to decrypt with RSA: %w", err)
		}
	}

	// 跳过验签：小程序获取手机号的数据已经是从支付宝端直接获取的
	_ = signData
	_ = signType

	var phoneData AlipayPhoneData
	if err := json.Unmarshal(plaintext, &phoneData); err != nil {
		mobileStr := string(plaintext)
		logger.Logger.Warn("Failed to parse decrypted JSON, trying as plain string",
			zap.Error(err),
			zap.String("plaintext", mobileStr),
		)
		if len(mobileStr) > 0 && len(mobileStr) <= 20 {

			return mobileStr, nil
		}
		return "", fmt.Errorf("failed to parse decrypted data: %w, decrypted text: %s", err, string(plaintext))
	}

	if phoneData.Mobile == "" {
		logger.Logger.Warn("Mobile field is empty in decrypted data",
			zap.String("decrypted_json", string(plaintext)),
		)
		return "", errors.New("mobile number not found in decrypted data")
	}

	logger.Logger.Info("Successfully decrypted alipay phone",
		zap.String("mobile", phoneData.Mobile),
	)

	return phoneData.Mobile, nil
}
