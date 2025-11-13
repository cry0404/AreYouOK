package middleware

import (
	"context"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/jwt"

	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/token"
)

const (
	IdentityKey = token.IdentityKey
)

var (
	authMiddleware *jwt.HertzJWTMiddleware
)

func initAuthMiddleware() error {
	// 使用 token 包中共享的生成器
	sharedGenerator := token.GetGenerator()
	if sharedGenerator == nil {
		return errors.ErrTokenGeneratorNotInitialized
	}

	authMiddleware = &jwt.HertzJWTMiddleware{
		Realm:       "AreYouOK API",
		Key:         sharedGenerator.Key,
		Timeout:     sharedGenerator.Timeout,
		MaxRefresh:  sharedGenerator.MaxRefresh,
		IdentityKey: sharedGenerator.IdentityKey,
		TimeFunc:    sharedGenerator.TimeFunc,

		IdentityHandler: func(ctx context.Context, c *app.RequestContext) interface{} {
			claims := jwt.ExtractClaims(ctx, c)
			uid, ok := claims[IdentityKey].(string)
			if !ok {
				if uidFloat, ok := claims[IdentityKey].(float64); ok {
					uid = fmt.Sprintf("%.0f", uidFloat)
				} else {
					return nil
				}
			}
			return uid
		},

		Unauthorized: func(ctx context.Context, c *app.RequestContext, code int, message string) {
			c.JSON(code, map[string]interface{}{
				"error": map[string]interface{}{
					"code":    "UNAUTHORIZED",
					"message": message,
				},
			})
		},

		TokenLookup:   "header: Authorization, query: token, cookie: jwt",
		TokenHeadName: "Bearer",
	}

	return nil
}

func AuthMiddleware() app.HandlerFunc {
	if authMiddleware == nil {
		panic("AuthMiddleware not initialized, call Init() first")
	}
	return authMiddleware.MiddlewareFunc()
}

// GetUserID 从请求上下文中获取用户ID（public_id，字符串格式）
func GetUserID(ctx context.Context, c *app.RequestContext) (string, bool) {
	userID, exists := c.Get(IdentityKey)
	if !exists {
		return "", false
	}

	id, ok := userID.(string)
	if !ok {
		return "", false
	}

	return id, true
}
