package utils

import (
	"AreYouOK/config"
	"crypto/sha256"
	"encoding/hex"
)

// hash 化 phone 电话号码存储，密文核对，增加盐值，避免彩虹表攻击，盐 + “：” + phone

func HashPhone(phone string) string {
	key := config.Cfg.PhoneHashSalt

	sum := sha256.Sum256([]byte(key + ":" + phone))

	return hex.EncodeToString(sum[:])
}


