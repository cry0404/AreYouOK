package router

import (
	"github.com/cloudwego/hertz/pkg/app/server"

	"AreYouOK/internal/handler"
	"AreYouOK/internal/middleware"
)

func Register(h *server.Hertz) {
	v1 := h.Group("/v1")

	// 认证相关路由
	auth := v1.Group("/auth")
	{
		auth.POST("/miniapp/alipay/exchange", handler.ExchangeAlipayAuth)
		auth.POST("/phone/send-captcha", handler.SendCaptcha)
		auth.POST("/phone/verify-slider", handler.VerifySlider)
		auth.POST("/phone/verify", middleware.AuthMiddleware(), handler.VerifyCaptcha)
		auth.POST("/token/refresh", handler.RefreshToken)
		// auth.GET("/waitlist/status", handler.GetWaitlistStatus)
	}

	// 用户相关路由
	users := v1.Group("/users")
	users.Use(middleware.AuthMiddleware()) // 需要鉴权的路由组
	{
		users.GET("/me/status", handler.GetUserStatus)
		users.GET("/me", handler.GetUserProfile)
		users.PUT("/me/settings", handler.UpdateUserSettings)
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
		checkIns.POST("/ack-reminder", handler.AckCheckInReminder)
	}

	// 行程报备路由
	journeys := v1.Group("/journeys")
	journeys.Use(middleware.AuthMiddleware())
	{
		journeys.GET("", handler.ListJourneys)
		journeys.POST("", handler.CreateJourney)
		journeys.GET("/:journey_id", handler.GetJourneyDetail)
		journeys.PATCH("/:journey_id", handler.UpdateJourney)
		journeys.POST("/:journey_id/complete", handler.CompleteJourney)
		journeys.POST("/:journey_id/ack-alert", handler.AckJourneyAlert)
		journeys.GET("/:journey_id/alerts", handler.GetJourneyAlerts)
	}

	// 通知任务路由
	notifications := v1.Group("/notifications")
	notifications.Use(middleware.AuthMiddleware())
	{
		notifications.GET("/tasks", handler.ListNotificationTasks)
		notifications.GET("/tasks/:task_id", handler.GetNotificationTaskDetail)
		notifications.POST("/ack", handler.AckNotification)
	}
}
