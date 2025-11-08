package router

import (
	"AreYouOK/internal/handler"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Register(h *server.Hertz) {
	v1 := h.Group("/v1")

	auth := v1.Group("/auth")
	auth.POST("/miniapp/alipay/exchange", handler.ExchangeAlipay)
	auth.POST("/token/refresh", handler.RefreshToken)
	auth.GET("/waitlist/status", handler.GetWaitlistStatus)

	// phone 属于 auth 部分
	phone := auth.Group("/phone")
	phone.POST("/send-captcha", handler.SendPhoneCaptcha)
	phone.POST("/verify-slider", handler.VerifySlider)
	phone.POST("/verify", handler.VerifyPhone)

	// authRequired 分组用于挂载需要鉴权的业务接口
	authRequired := v1.Group("")
	// TODO: 在此添加 JWT 鉴权、限流等中间件，例如 authRequired.Use(middleware.Auth())

	onboarding := authRequired.Group("/onboarding")
	onboarding.GET("/progress", handler.GetOnboardingProgress)

	authRequired.GET("/users/me", handler.GetCurrentUser)
	authRequired.PUT("/users/me/settings", handler.UpdateUserSettings)
	authRequired.GET("/users/me/quotas", handler.GetUserQuotas)

	authRequired.GET("/contacts", handler.ListContacts)
	authRequired.POST("/contacts", handler.CreateContact)
	authRequired.DELETE("/contacts/:contact_id", handler.DeleteContact)

	checkIns := authRequired.Group("/check-ins")
	checkIns.GET("/today", handler.GetTodayCheckIn)
	checkIns.POST("/today/complete", handler.CompleteTodayCheckIn)
	checkIns.GET("/history", handler.GetCheckInHistory)
	checkIns.POST("/ack-reminder", handler.AckCheckInReminder)

	authRequired.GET("/journeys", handler.ListJourneys)
	authRequired.POST("/journeys", handler.CreateJourney)
	authRequired.GET("/journeys/:journey_id", handler.GetJourney)
	authRequired.PATCH("/journeys/:journey_id", handler.UpdateJourney)
	authRequired.POST("/journeys/:journey_id/complete", handler.CompleteJourney)
	authRequired.POST("/journeys/:journey_id/ack-alert", handler.AckJourneyAlert)
	authRequired.GET("/journeys/:journey_id/alerts", handler.GetJourneyAlerts)

	notifications := authRequired.Group("/notifications")
	notifications.GET("/tasks", handler.ListNotificationTasks)
	notifications.GET("/tasks/:task_id", handler.GetNotificationTask)
	notifications.POST("/ack", handler.AckNotification)
	notifications.GET("/templates", handler.ListNotificationTemplates)
}
