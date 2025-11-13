package token

import (
	"AreYouOK/config"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/hertz-contrib/jwt"
)

const (
	IdentityKey = "uid"
)

var (

	// 这个实例会被 middleware 和 token 包共同使用
	sharedGenerator *jwt.HertzJWTMiddleware
)


func Init() error {
	var err error
	sharedGenerator, err = jwt.New(&jwt.HertzJWTMiddleware{
		Key:         []byte(config.Cfg.JWTSecret),
		Timeout:     time.Duration(config.Cfg.JWTExpireMinutes) * time.Minute,
		MaxRefresh:  time.Duration(config.Cfg.JWTRefreshDays) * 24 * time.Hour,
		IdentityKey: IdentityKey,
		TimeFunc:    time.Now,
	})

	if err != nil {
		return fmt.Errorf("failed to initialize token generator: %w", err)
	}

	return nil
}

// GetGenerator 获取共享的 token 生成器（供 middleware 使用）
func GetGenerator() *jwt.HertzJWTMiddleware {
	return sharedGenerator
}

// GenerateTokenPair 生成 access token 和 refresh token
func GenerateTokenPair(userID string) (accessToken, refreshToken string, expiresIn int, err error) {
	if sharedGenerator == nil {
		return "", "", 0, fmt.Errorf("token generator not initialized, call token.Init() first")
	}


	claims := jwt.MapClaims{
		IdentityKey: userID,
		"iat":       time.Now().Unix(),
	}

	var expiresAt time.Time
	accessToken, expiresAt, err = sharedGenerator.TokenGenerator(claims)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to generate access token: %w", err)
	}

	expiresIn = int(time.Until(expiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}


	refreshClaims := jwtv5.MapClaims{
		IdentityKey: userID,
		"iat":       time.Now().Unix(),
		"type":      "refresh",
		"exp":       time.Now().Add(time.Duration(config.Cfg.JWTRefreshDays) * 24 * time.Hour).Unix(),
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, refreshClaims)
	refreshToken, err = token.SignedString([]byte(config.Cfg.JWTSecret))
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, expiresIn, nil
}

// ValidateRefreshToken 验证 refresh token 并返回用户 ID
func ValidateRefreshToken(tokenString string) (userID string, err error) {
	token, err := jwtv5.ParseWithClaims(tokenString, jwtv5.MapClaims{}, func(token *jwtv5.Token) (interface{}, error) {
		if token.Method != jwtv5.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v, expected HS256", token.Header["alg"])
		}
		return []byte(config.Cfg.JWTSecret), nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	// 从 token 中提取 claims
	claims, ok := token.Claims.(jwtv5.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid token claims")
	}

	// 验证 token 类型是否为 refresh
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return "", fmt.Errorf("invalid token type, expected refresh token")
	}


	uid, ok := claims[IdentityKey].(string)
	if !ok {
		// 尝试从 float64 转换（JSON 数字可能被解析为 float64）
		if uidFloat, ok := claims[IdentityKey].(float64); ok {
			uid = fmt.Sprintf("%.0f", uidFloat)
		} else {
			return "", fmt.Errorf("user ID not found in token")
		}
	}

	return uid, nil
}