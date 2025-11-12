package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

// ListJourneys 查询当前与历史行程
// GET /v1/journeys
func ListJourneys(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现列表行程逻辑
}

// CreateJourney 创建行程报备
// POST /v1/journeys
func CreateJourney(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现创建行程逻辑
}

// GetJourneyDetail 查看行程详情
// GET /v1/journeys/:journey_id
func GetJourneyDetail(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现获取行程详情逻辑
}

// UpdateJourney 更新行程
// PATCH /v1/journeys/:journey_id
func UpdateJourney(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现更新行程逻辑
}

// CompleteJourney 归来打卡，标记行程结束
// POST /v1/journeys/:journey_id/complete
func CompleteJourney(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现完成行程逻辑
}

// AckJourneyAlert 确认已知晓行程超时提醒
// POST /v1/journeys/:journey_id/ack-alert
func AckJourneyAlert(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现确认提醒逻辑
}

// GetJourneyAlerts 查询行程提醒执行状态
// GET /v1/journeys/:journey_id/alerts
func GetJourneyAlerts(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现获取提醒状态逻辑
}





