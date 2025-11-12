package middleware

import (
	"context"
	"fmt"
	"time"

	"AreYouOK/config"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/jwt"
)

const (
	// IdentityKey 用于在请求上下文中存储用户ID的key
	IdentityKey = "uid"
)

var (
	authMiddleware *jwt.HertzJWTMiddleware
)

func initAuthMiddleware() error {
	var err error
	authMiddleware, err = jwt.New(&jwt.HertzJWTMiddleware{
		Realm:       "AreYouOK API",
		Key:         []byte(config.Cfg.JWTSecret),
		Timeout:     time.Duration(config.Cfg.JWTExpireMinutes) * time.Minute,
		MaxRefresh:  time.Duration(config.Cfg.JWTRefreshDays) * 24 * time.Hour,
		IdentityKey: IdentityKey,

		// 从token中提取用户ID（public_id，字符串格式）并存入上下文
		IdentityHandler: func(ctx context.Context, c *app.RequestContext) interface{} {
			claims := jwt.ExtractClaims(ctx, c)

			// 从claims中提取uid（对应public_id，字符串格式）
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

		// Token查找方式：优先从Header的Authorization，其次从Query参数，最后从Cookie
		TokenLookup: "header: Authorization, query: token, cookie: jwt",

		TokenHeadName: "Bearer",

		TimeFunc: time.Now,
	})

	if err != nil {
		return fmt.Errorf("failed to initialize JWT middleware: %w", err)
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
// 在handler中使用：userID := middleware.GetUserID(ctx, c)
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

func GetAuthMiddleware() *jwt.HertzJWTMiddleware {
	return authMiddleware
}
