package token

import (
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/hertz-contrib/jwt"

	"AreYouOK/config"
	"AreYouOK/pkg/errors"
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
		return "", "", 0, errors.ErrTokenGeneratorNotInitialized
	}


	now := time.Now()
	expiresAt := now.Add(time.Duration(config.Cfg.JWTExpireMinutes) * time.Minute)

	accessClaims := jwtv5.MapClaims{
		IdentityKey: userID,
		"iat":       now.Unix(),
		"exp":       expiresAt.Unix(),
	}

	accessTokenObj := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, accessClaims)
	accessToken, err = accessTokenObj.SignedString([]byte(config.Cfg.JWTSecret))
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to generate access token: %w", err)
	}

	expiresIn = int(time.Until(expiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}

	// 生成 refresh token
	refreshClaims := jwtv5.MapClaims{
		IdentityKey: userID,
		"iat":       now.Unix(),
		"type":      "refresh",
		"exp":       now.Add(time.Duration(config.Cfg.JWTRefreshDays) * 24 * time.Hour).Unix(),
	}

	refreshTokenObj := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTokenObj.SignedString([]byte(config.Cfg.JWTSecret))
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return accessToken, refreshToken, expiresIn, nil
}

// ValidateRefreshToken 验证 refresh token 并返回用户 ID
func ValidateRefreshToken(tokenString string) (userID string, err error) {
	token, err := jwtv5.ParseWithClaims(tokenString, jwtv5.MapClaims{}, func(token *jwtv5.Token) (interface{}, error) {
		if token.Method != jwtv5.SigningMethodHS256 {
			return nil, fmt.Errorf("%w: %v, expected HS256", errors.ErrUnexpectedSigningMethod, token.Header["alg"])
		}
		return []byte(config.Cfg.JWTSecret), nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return "", errors.ErrInvalidToken
	}


	claims, ok := token.Claims.(jwtv5.MapClaims)
	if !ok {
		return "", errors.ErrInvalidTokenClaims
	}


	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return "", errors.ErrInvalidTokenType
	}

	uid, ok := claims[IdentityKey].(string)
	if !ok {

		if uidFloat, ok := claims[IdentityKey].(float64); ok {
			uid = fmt.Sprintf("%.0f", uidFloat)
		} else {
			return "", errors.ErrUserIDNotFound
		}
	}

	return uid, nil
}
