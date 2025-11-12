package repository

import (
	"AreYouOK/storage/database"
	"fmt"
	"os"

	"gorm.io/gen"
)

// ========== User 相关查询接口 ==========

// UserQuerier 用户查询接口
type UserQuerier interface {
	// GetByAlipayOpenID 根据支付宝 OpenID 查询用户
	// SELECT * FROM @@table WHERE alipay_open_id = @openID LIMIT 1
	GetByAlipayOpenID(openID string) (*gen.T, error)

	// GetByPhoneHash 根据手机号哈希查询用户
	// SELECT * FROM @@table WHERE phone_hash = @phoneHash LIMIT 1
	GetByPhoneHash(phoneHash string) (*gen.T, error)

	// GetWithQuotas 获取用户及额度信息（JOIN quota_transactions 计算余额）
	// SELECT u.*,
	//   COALESCE(SUM(CASE WHEN qt.channel = 'sms' AND qt.transaction_type = 'grant' THEN qt.amount ELSE 0 END) -
	//            SUM(CASE WHEN qt.channel = 'sms' AND qt.transaction_type = 'deduct' THEN qt.amount ELSE 0 END), 0) as sms_balance,
	//   COALESCE(SUM(CASE WHEN qt.channel = 'voice' AND qt.transaction_type = 'grant' THEN qt.amount ELSE 0 END) -
	//            SUM(CASE WHEN qt.channel = 'voice' AND qt.transaction_type = 'deduct' THEN qt.amount ELSE 0 END), 0) as voice_balance
	// FROM @@table u
	// LEFT JOIN quota_transactions qt ON qt.user_id = u.id
	// WHERE u.id = @userID
	// GROUP BY u.id
	// LIMIT 1
	GetWithQuotas(userID int64) (gen.M, error)
}

// ========== DailyCheckIn 相关查询接口 ==========

// DailyCheckInQuerier 打卡记录查询接口
type DailyCheckInQuerier interface {
	// GetTodayCheckIn 获取今日打卡记录
	// SELECT * FROM @@table
	// WHERE user_id = @userID AND check_in_date = @date::date
	// LIMIT 1
	GetTodayCheckIn(userID int64, date string) (*gen.T, error)

	// ListByUserIDAndDateRange 按用户和日期范围查询打卡记录（分页）
	// SELECT * FROM @@table
	// WHERE user_id = @userID
	//   AND check_in_date >= @fromDate::date
	//   AND check_in_date <= @toDate::date
	//   {{if status != ""}}
	//   AND status = @status
	//   {{end}}
	// ORDER BY check_in_date DESC
	// LIMIT @limit OFFSET @offset
	ListByUserIDAndDateRange(userID int64, fromDate, toDate string, status string, limit, offset int) ([]*gen.T, error)

	// ListPendingCheckIns 查询待打卡的记录（用于定时任务）
	// SELECT * FROM @@table
	// WHERE status = 'pending'
	//   AND check_in_date = CURRENT_DATE
	//   AND user_id IN (
	//     SELECT id FROM users WHERE daily_check_in_enabled = true AND status = 'active'
	//   )
	ListPendingCheckIns() ([]*gen.T, error)

	// ListTimeoutCheckIns 查询超时未打卡的记录（用于定时任务）
	// SELECT * FROM @@table
	// WHERE status = 'pending'
	//   AND check_in_date = CURRENT_DATE
	//   AND reminder_sent_at IS NOT NULL
	//   AND user_id IN (
	//     SELECT id FROM users WHERE daily_check_in_enabled = true AND status = 'active'
	//   )
	ListTimeoutCheckIns() ([]*gen.T, error)
}

// ========== Journey 相关查询接口 ==========

// JourneyQuerier 行程查询接口
type JourneyQuerier interface {
	// ListByUserIDAndStatus 按用户和状态查询行程（分页）
	// SELECT * FROM @@table
	// WHERE user_id = @userID
	//   {{if status != ""}}
	//   AND status = @status
	//   {{end}}
	// ORDER BY created_at DESC
	// LIMIT @limit OFFSET @offset
	ListByUserIDAndStatus(userID int64, status string, limit, offset int) ([]*gen.T, error)

	// GetOngoingJourneys 查询进行中的行程（用于定时任务）
	// SELECT * FROM @@table
	// WHERE status = 'ongoing'
	//   AND expected_return_time < NOW()
	ListOngoingJourneys() ([]*gen.T, error)

	// ListTimeoutJourneys 查询超时的行程（用于定时任务）
	// SELECT * FROM @@table
	// WHERE status = 'ongoing'
	//   AND expected_return_time < NOW() - INTERVAL '10 minutes'
	ListTimeoutJourneys() ([]*gen.T, error)
}

// ========== NotificationTask 相关查询接口 ==========

// NotificationTaskQuerier 通知任务查询接口
type NotificationTaskQuerier interface {
	// ListByUserID 按用户查询通知任务（分页，支持筛选）
	// SELECT * FROM @@table
	// WHERE user_id = @userID
	//   {{if category != ""}}
	//   AND category = @category
	//   {{end}}
	//   {{if channel != ""}}
	//   AND channel = @channel
	//   {{end}}
	//   {{if status != ""}}
	//   AND status = @status
	//   {{end}}
	// ORDER BY created_at DESC
	// LIMIT @limit OFFSET @offset
	ListByUserID(userID int64, category, channel, status string, limit, offset int) ([]*gen.T, error)

	// ListPendingTasks 查询待处理的任务（用于消息消费）
	// SELECT * FROM @@table
	// WHERE status = 'pending'
	//   AND scheduled_at <= NOW()
	// ORDER BY scheduled_at ASC
	// LIMIT @limit
	ListPendingTasks(limit int) ([]*gen.T, error)

	// GetWithAttempts 获取任务及尝试记录（JOIN contact_attempts）
	// SELECT t.*,
	//   json_agg(
	//     json_build_object(
	//       'id', ca.id,
	//       'contact_priority', ca.contact_priority,
	//       'contact_phone_hash', ca.contact_phone_hash,
	//       'channel', ca.channel,
	//       'status', ca.status,
	//       'response_code', ca.response_code,
	//       'response_message', ca.response_message,
	//       'cost_cents', ca.cost_cents,
	//       'deducted', ca.deducted,
	//       'attempted_at', ca.attempted_at
	//     ) ORDER BY ca.attempted_at DESC
	//   ) FILTER (WHERE ca.id IS NOT NULL) as attempts
	// FROM @@table t
	// LEFT JOIN contact_attempts ca ON ca.task_id = t.id
	// WHERE t.id = @taskID
	// GROUP BY t.id
	// LIMIT 1
	GetWithAttempts(taskID int64) (gen.M, error)
}

// ========== QuotaTransaction 相关查询接口 ==========

// QuotaTransactionQuerier 额度流水查询接口
type QuotaTransactionQuerier interface {
	// GetBalanceByUserID 计算用户余额（按渠道分组）
	// SELECT
	//   channel,
	//   COALESCE(SUM(CASE WHEN transaction_type = 'grant' THEN amount ELSE 0 END) -
	//            SUM(CASE WHEN transaction_type = 'deduct' THEN amount ELSE 0 END), 0) as balance
	// FROM @@table
	// WHERE user_id = @userID
	// GROUP BY channel
	GetBalanceByUserID(userID int64) ([]gen.M, error)

	// ListByUserID 按用户查询流水（分页）
	// SELECT * FROM @@table
	// WHERE user_id = @userID
	// ORDER BY created_at DESC
	// LIMIT @limit OFFSET @offset
	ListByUserID(userID int64, limit, offset int) ([]*gen.T, error)
}

func Generate() error {

	if err := database.Init(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// 运行数据库迁移（确保表存在）
	if err := database.Migrate(); err != nil {
		return fmt.Errorf("failed to run database migration: %w", err)
	}

	db := database.DB()
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	g := gen.NewGenerator(gen.Config{
		OutPath:           "./internal/repository/query", // 生成代码的输出路径
		Mode:              gen.WithDefaultQuery | gen.WithQueryInterface | gen.WithoutContext,
		FieldNullable:     true, // 字段可以为 null
		FieldCoverable:    false,
		FieldSignable:     false,
		FieldWithIndexTag: false,
		FieldWithTypeTag:  true,
	})

	g.UseDB(db)

	//从表生成模型
	userModel := g.GenerateModel("users")
	checkInModel := g.GenerateModel("daily_check_ins")
	journeyModel := g.GenerateModel("journeys")
	notificationTaskModel := g.GenerateModel("notification_tasks")
	quotaTransactionModel := g.GenerateModel("quota_transactions")

	g.ApplyInterface(func(UserQuerier) {}, userModel)
	g.ApplyInterface(func(DailyCheckInQuerier) {}, checkInModel)
	g.ApplyInterface(func(JourneyQuerier) {}, journeyModel)
	g.ApplyInterface(func(NotificationTaskQuerier) {}, notificationTaskModel)
	g.ApplyInterface(func(QuotaTransactionQuerier) {}, quotaTransactionModel)

	g.Execute()

	return nil
}

func RunGenerate() {
	if err := Generate(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate code: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Code generation completed successfully!")
}
