package router

import (
	"github.com/cloudwego/hertz/pkg/app/server"

	"AreYouOK/internal/handler"
	"AreYouOK/internal/middleware"
)

func Register(h *server.Hertz) {

	h.Use(middleware.RecoverMiddleware())
	h.Use(middleware.CORSMiddleware())
	h.Use(middleware.OpenTelemetryMiddleware())
	//h.Use(middleware.CSRFMiddleware()) csrf 中间件，支付宝小程序似乎不需要
	v1 := h.Group("/v1")

	// 认证相关路由
	auth := v1.Group("/auth")
	auth.Use(middleware.AuthRateLimitMiddleware()) // 认证接口限流
	{
		auth.POST("/miniapp/alipay/exchange", handler.ExchangeAlipayAuth)
		auth.POST("/token/refresh", handler.RefreshToken)

		// 验证码相关路由
		captcha := auth.Group("/phone", middleware.CaptchaRateLimitMiddleware())
		{
			captcha.POST("/send-captcha", handler.SendCaptcha)
			captcha.POST("/verify-slider", handler.VerifySlider)
			captcha.POST("/verify", handler.VerifyCaptcha)
		}

		// auth.GET("/waitlist/status", handler.GetWaitlistStatus)
	}

	// 用户相关路由
	users := v1.Group("/users")
	users.Use(middleware.AuthMiddleware()) // 需要鉴权的路由组
	{
		users.GET("/me/status", handler.GetUserStatus)
		users.GET("/me", handler.GetUserProfile)
		users.PUT("/me/settings", /*middleware.UserSettingsRateLimitMiddleware(),*/ handler.UpdateUserSettings) // 用户设置修改限流
		users.GET("/me/quotas", handler.GetUserQuotas)
	}

	// 紧急联系人路由
	contacts := v1.Group("/contacts")
	contacts.Use(middleware.AuthMiddleware())
	{
		contacts.GET("", handler.ListContacts)
		contacts.POST("", handler.CreateContact)
		contacts.DELETE("/:priority", handler.DeleteContact)
	}

	// 平安打卡路由
	checkIns := v1.Group("/check-ins")
	checkIns.Use(middleware.AuthMiddleware())
	{
		checkIns.GET("/today", handler.GetTodayCheckIn)
		checkIns.POST("/today/complete", handler.CompleteTodayCheckIn)
		checkIns.GET("/history", handler.GetCheckInHistory)
		//checkIns.POST("/ack-reminder", handler.AckCheckInReminder)
	}

	// 行程报备路由
	journeys := v1.Group("/journeys")
	journeys.Use(middleware.AuthMiddleware())
	{
		journeys.GET("", handler.ListJourneys)
		journeys.POST("", handler.CreateJourney)
		journeys.GET("/:journey_id", handler.GetJourneyDetail)
		journeys.PATCH("/:journey_id", middleware.JourneySettingsRateLimitMiddleware(), handler.UpdateJourney) //行程修改限流，这里最后还需要考虑最后几分钟就不能修改了的问题
		journeys.POST("/:journey_id/complete", handler.CompleteJourney)
		//journeys.POST("/:journey_id/ack-alert", handler.AckJourneyAlert)
		journeys.GET("/:journey_id/alerts", handler.GetJourneyAlerts)
	}
}

	
