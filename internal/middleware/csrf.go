package middleware

import (
	"AreYouOK/config"
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/csrf"
	"github.com/hertz-contrib/sessions"
	"github.com/hertz-contrib/sessions/cookie"
)




func CSRFMiddleware() app.HandlerFunc{
	store := cookie.NewStore([]byte(config.Cfg.CSRFSecret))
	return func(ctx context.Context, c *app.RequestContext) {
		session := sessions.DefaultMany(c, "csrf-session")
		if session == nil {
			c.AbortWithStatus(http.StatusInternalServerError)
		}

		sessions.New("csrf-session", store)(ctx, c)
		csrf.New(
			csrf.WithSecret(config.Cfg.SessionSecret),
		) (ctx, c)
	}
}