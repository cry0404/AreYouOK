package router

import (
	"AreYouOK/internal/handler"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Register(h *server.Hertz) {
	v1 := h.Group("/v1")

	auth := v1.Group("/auth")
	auth.POST("/miniapp/alipay/exchange", handler.ExchangeAlipay)
	
	// phone 属于 auth 部分
	phone := auth.Group("/phone")
	phone.POST("/send-captcha", handler.SendPhoneCaptcha)
	phone.POST("/verify-slider", handler.VerifySlider)
	phone.POST("/verify", handler.VerifyPhone)
	auth.POST("/token/refresh", handler.RefreshToken)
	auth.GET("/waitlist/status", handler.GetWaitlistStatus)

	onboarding := v1.Group("/onboarding")
	onboarding.GET("/progress", handler.GetOnboardingProgress)

	v1.GET("/users/me", handler.GetCurrentUser)
	v1.PUT("/users/me/settings", handler.UpdateUserSettings)
	v1.GET("/users/me/quotas", handler.GetUserQuotas)

	v1.GET("/contacts", handler.ListContacts)
	v1.POST("/contacts", handler.CreateContact)
	v1.DELETE("/contacts/:contact_id", handler.DeleteContact)

	checkIns := v1.Group("/check-ins")
	checkIns.GET("/today", handler.GetTodayCheckIn)
	checkIns.POST("/today/complete", handler.CompleteTodayCheckIn)
	checkIns.GET("/history", handler.GetCheckInHistory)
	checkIns.POST("/ack-reminder", handler.AckCheckInReminder)

	v1.GET("/journeys", handler.ListJourneys)
	v1.POST("/journeys", handler.CreateJourney)
	v1.GET("/journeys/:journey_id", handler.GetJourney)
	v1.PATCH("/journeys/:journey_id", handler.UpdateJourney)
	v1.POST("/journeys/:journey_id/complete", handler.CompleteJourney)
	v1.POST("/journeys/:journey_id/ack-alert", handler.AckJourneyAlert)
	v1.GET("/journeys/:journey_id/alerts", handler.GetJourneyAlerts)

	notifications := v1.Group("/notifications")
	notifications.GET("/tasks", handler.ListNotificationTasks)
	notifications.GET("/tasks/:task_id", handler.GetNotificationTask)
	notifications.POST("/ack", handler.AckNotification)
	notifications.GET("/templates", handler.ListNotificationTemplates)
}
