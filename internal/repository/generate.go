package repository

import (
	"fmt"
	"os"

	"gorm.io/gen"

	"AreYouOK/internal/model"
	"AreYouOK/pkg/errors"
	"AreYouOK/storage/database"
)

// ========== User 相关查询接口 ==========

// UserQuerier 用户查询接口
type UserQuerier interface {
	// GetByAlipayOpenID 根据支付宝 OpenID 查询用户
	//
	// SELECT * FROM @@table WHERE alipay_open_id = @openID LIMIT 1
	GetByAlipayOpenID(openID string) (*gen.T, error)

	// GetByPhoneHash 根据手机号哈希查询用户
	//
	// SELECT * FROM @@table WHERE phone_hash = @phoneHash LIMIT 1
	GetByPhoneHash(phoneHash string) (*gen.T, error)

	// GetByPublicID 根据 PublicID 查询用户（最常用，API 中 userID 是 public_id）
	//
	// SELECT * FROM @@table WHERE public_id = @publicID LIMIT 1
	GetByPublicID(publicID int64) (*gen.T, error)

	// GetByID 根据数据库主键 ID 查询用户
	//
	// SELECT * FROM @@table WHERE id = @id LIMIT 1
	GetByID(id int64) (*gen.T, error)

	// ListByStatus 根据状态查询用户列表（用于管理后台或定时任务）
	//
	// SELECT * FROM @@table
	// WHERE status = @status
	// {{if limit > 0}}
	// LIMIT @limit
	// {{end}}
	// {{if offset > 0}}
	// OFFSET @offset
	// {{end}}
	ListByStatus(status string, limit, offset int) ([]*gen.T, error)

	// ListActiveUsersWithCheckInEnabled 查询开启打卡的活跃用户（用于定时任务）
	//
	// SELECT * FROM @@table
	// WHERE status = 'active'
	//   AND daily_check_in_enabled = true
	ListActiveUsersWithCheckInEnabled() ([]*gen.T, error)

	// CountByStatus 统计各状态的用户数量
	//
	// SELECT status, COUNT(*) as count
	// FROM @@table
	// GROUP BY status
	CountByStatus() ([]gen.M, error)

	// GetWithQuotas 获取用户及额度信息（JOIN quota_transactions 计算余额）
	//
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

	// GetByPublicIDWithQuotas 根据 PublicID 获取用户及额度信息（API 常用）
	//
	// SELECT u.*,
	//   COALESCE(SUM(CASE WHEN qt.channel = 'sms' AND qt.transaction_type = 'grant' THEN qt.amount ELSE 0 END) -
	//            SUM(CASE WHEN qt.channel = 'sms' AND qt.transaction_type = 'deduct' THEN qt.amount ELSE 0 END), 0) as sms_balance,
	//   COALESCE(SUM(CASE WHEN qt.channel = 'voice' AND qt.transaction_type = 'grant' THEN qt.amount ELSE 0 END) -
	//            SUM(CASE WHEN qt.channel = 'voice' AND qt.transaction_type = 'deduct' THEN qt.amount ELSE 0 END), 0) as voice_balance
	// FROM @@table u
	// LEFT JOIN quota_transactions qt ON qt.user_id = u.id
	// WHERE u.public_id = @publicID
	// GROUP BY u.id
	// LIMIT 1
	GetByPublicIDWithQuotas(publicID int64) (gen.M, error)
}

// ========== DailyCheckIn 相关查询接口 ==========

// DailyCheckInQuerier 打卡记录查询接口
type DailyCheckInQuerier interface {
	// GetTodayCheckIn 获取今日打卡记录（根据 userID 是数据库主键）
	//
	// SELECT * FROM @@table
	// WHERE user_id = @userID AND check_in_date = CURRENT_DATE
	// LIMIT 1
	GetTodayCheckIn(userID int64) (*gen.T, error)

	// GetTodayCheckInByPublicID 根据 PublicID 获取今日打卡记录（API 常用）
	//
	// SELECT dci.* FROM @@table dci
	// INNER JOIN users u ON u.id = dci.user_id
	// WHERE u.public_id = @publicID AND dci.check_in_date = CURRENT_DATE
	// LIMIT 1
	GetTodayCheckInByPublicID(publicID int64) (*gen.T, error)

	// GetByUserIDAndDate 根据用户ID和日期查询打卡记录
	//
	// SELECT * FROM @@table
	// WHERE user_id = @userID AND check_in_date = @date::date
	// LIMIT 1
	GetByUserIDAndDate(userID int64, date string) (*gen.T, error)

	// ListByUserIDAndDateRange 按用户和日期范围查询打卡记录（分页）
	//
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

	// ListByPublicIDAndDateRange 根据 PublicID 和日期范围查询打卡记录（API 常用）
	//
	// SELECT dci.* FROM @@table dci
	// INNER JOIN users u ON u.id = dci.user_id
	// WHERE u.public_id = @publicID
	//   AND dci.check_in_date >= @fromDate::date
	//   AND dci.check_in_date <= @toDate::date
	//   {{if status != ""}}
	//   AND dci.status = @status
	//   {{end}}
	// ORDER BY dci.check_in_date DESC
	// LIMIT @limit OFFSET @offset
	ListByPublicIDAndDateRange(publicID int64, fromDate, toDate string, status string, limit, offset int) ([]*gen.T, error)

	// GetStreakDays 计算用户连续打卡天数（用于奖励计算）
	//
	// WITH RECURSIVE streak AS (
	//   SELECT check_in_date, 1 as days
	//   FROM @@table
	//   WHERE user_id = @userID AND status = 'done'
	//   ORDER BY check_in_date DESC
	//   LIMIT 1
	//   UNION ALL
	//   SELECT dci.check_in_date, s.days + 1
	//   FROM @@table dci
	//   INNER JOIN streak s ON dci.check_in_date = s.check_in_date - INTERVAL '1 day'
	//   WHERE dci.user_id = @userID AND dci.status = 'done'
	// )
	// SELECT MAX(days) as streak_days FROM streak
	GetStreakDays(userID int64) (int, error)

	// ListPendingCheckIns 查询待打卡的记录（用于定时任务）
	//
	// SELECT dci.* FROM @@table dci
	// INNER JOIN users u ON u.id = dci.user_id
	// WHERE dci.status = 'pending'
	//   AND dci.check_in_date = CURRENT_DATE
	//   AND u.daily_check_in_enabled = true
	//   AND u.status = 'active'
	ListPendingCheckIns() ([]*gen.T, error)

	// ListTimeoutCheckIns 查询超时未打卡的记录（用于定时任务）
	//
	// SELECT dci.* FROM @@table dci
	// INNER JOIN users u ON u.id = dci.user_id
	// WHERE dci.status = 'pending'
	//   AND dci.check_in_date = CURRENT_DATE
	//   AND dci.reminder_sent_at IS NOT NULL
	//   AND u.daily_check_in_enabled = true
	//   AND u.status = 'active'
	ListTimeoutCheckIns() ([]*gen.T, error)

	// CountByUserIDAndStatus 统计用户打卡记录数量（按状态）
	//
	// SELECT status, COUNT(*) as count
	// FROM @@table
	// WHERE user_id = @userID
	// GROUP BY status
	CountByUserIDAndStatus(userID int64) ([]gen.M, error)
}

// ========== Journey 相关查询接口 ==========

// JourneyQuerier 行程查询接口
type JourneyQuerier interface {
	// GetByID 根据数据库主键 ID 查询行程
	//
	// SELECT * FROM @@table WHERE id = @id LIMIT 1
	GetByID(id int64) (*gen.T, error)

	// GetByPublicIDAndJourneyID 根据 PublicID 和 Journey ID 查询行程（API 常用）
	//
	// SELECT j.* FROM @@table j
	// INNER JOIN users u ON u.id = j.user_id
	// WHERE u.public_id = @publicID AND j.id = @journeyID
	// LIMIT 1
	GetByPublicIDAndJourneyID(publicID int64, journeyID int64) (*gen.T, error)

	// ListByUserIDAndStatus 按用户和状态查询行程（分页）
	//
	// SELECT * FROM @@table
	// WHERE user_id = @userID
	//   {{if status != ""}}
	//   AND status = @status
	//   {{end}}
	// ORDER BY created_at DESC
	// LIMIT @limit OFFSET @offset
	ListByUserIDAndStatus(userID int64, status string, limit, offset int) ([]*gen.T, error)

	// ListByPublicIDAndStatus 根据 PublicID 和状态查询行程（API 常用，支持游标分页）
	//
	// SELECT j.* FROM @@table j
	// INNER JOIN users u ON u.id = j.user_id
	// WHERE u.public_id = @publicID
	//   {{if status != ""}}
	//   AND j.status = @status
	//   {{end}}
	//   {{if cursorID > 0}}
	//   AND j.id < @cursorID
	//   {{end}}
	// ORDER BY j.created_at DESC
	// LIMIT @limit
	ListByPublicIDAndStatus(publicID int64, status string, cursorID int64, limit int) ([]*gen.T, error)

	// GetOngoingJourneyByUserID 查询用户进行中的行程（用于检查行程重叠）
	//
	// SELECT * FROM @@table
	// WHERE user_id = @userID
	//   AND status = 'ongoing'
	// ORDER BY expected_return_time ASC
	// LIMIT 1
	GetOngoingJourneyByUserID(userID int64) (*gen.T, error)

	// GetOngoingJourneyByPublicID 根据 PublicID 查询用户进行中的行程（API 常用）
	//
	// SELECT j.* FROM @@table j
	// INNER JOIN users u ON u.id = j.user_id
	// WHERE u.public_id = @publicID
	//   AND j.status = 'ongoing'
	// ORDER BY j.expected_return_time ASC
	// LIMIT 1
	GetOngoingJourneyByPublicID(publicID int64) (*gen.T, error)

	// GetOngoingJourneys 查询进行中的行程（用于定时任务）
	//
	// SELECT * FROM @@table
	// WHERE status = 'ongoing'
	//   AND expected_return_time < NOW()
	ListOngoingJourneys() ([]*gen.T, error)

	// ListTimeoutJourneys 查询超时的行程（用于定时任务）
	//
	// SELECT * FROM @@table
	// WHERE status = 'ongoing'
	//   AND expected_return_time < NOW() - INTERVAL '10 minutes'
	ListTimeoutJourneys() ([]*gen.T, error)

	// CountByUserIDAndStatus 统计用户行程数量（按状态）
	//
	// SELECT status, COUNT(*) as count
	// FROM @@table
	// WHERE user_id = @userID
	// GROUP BY status
	CountByUserIDAndStatus(userID int64) ([]gen.M, error)
}

// ========== NotificationTask 相关查询接口 ==========

// NotificationTaskQuerier 通知任务查询接口
type NotificationTaskQuerier interface {
	// GetByID 根据数据库主键 ID 查询通知任务
	//
	// SELECT * FROM @@table WHERE id = @id LIMIT 1
	GetByID(id int64) (*gen.T, error)

	// GetByTaskCode 根据 TaskCode 查询通知任务（幂等性检查）
	//
	// SELECT * FROM @@table WHERE task_code = @taskCode LIMIT 1
	GetByTaskCode(taskCode int64) (*gen.T, error)

	// GetByPublicIDAndTaskID 根据 PublicID 和 Task ID 查询通知任务（API 常用）
	//
	// SELECT nt.* FROM @@table nt
	// INNER JOIN users u ON u.id = nt.user_id
	// WHERE u.public_id = @publicID AND nt.id = @taskID
	// LIMIT 1
	GetByPublicIDAndTaskID(publicID int64, taskID int64) (*gen.T, error)

	// ListByUserID 按用户查询通知任务（分页，支持筛选）
	//
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

	// ListByPublicID 根据 PublicID 查询通知任务（API 常用，支持游标分页）
	//
	// SELECT nt.* FROM @@table nt
	// INNER JOIN users u ON u.id = nt.user_id
	// WHERE u.public_id = @publicID
	//   {{if category != ""}}
	//   AND nt.category = @category
	//   {{end}}
	//   {{if channel != ""}}
	//   AND nt.channel = @channel
	//   {{end}}
	//   {{if status != ""}}
	//   AND nt.status = @status
	//   {{end}}
	//   {{if cursorID > 0}}
	//   AND nt.id < @cursorID
	//   {{end}}
	// ORDER BY nt.created_at DESC
	// LIMIT @limit
	ListByPublicID(publicID int64, category, channel, status string, cursorID int64, limit int) ([]*gen.T, error)

	// ListPendingTasks 查询待处理的任务（用于消息消费）
	//
	// SELECT * FROM @@table
	// WHERE status = 'pending'
	//   AND scheduled_at <= NOW()
	// ORDER BY scheduled_at ASC
	// LIMIT @limit
	ListPendingTasks(limit int) ([]*gen.T, error)

	// ListFailedTasksForRetry 查询失败需要重试的任务
	//
	// SELECT * FROM @@table
	// WHERE status = 'failed'
	//   AND retry_count < 3
	//   AND scheduled_at <= NOW()
	// ORDER BY scheduled_at ASC
	// LIMIT @limit
	ListFailedTasksForRetry(limit int) ([]*gen.T, error)

	// GetWithAttempts 获取任务及尝试记录（JOIN contact_attempts）
	//
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

	// CountByUserIDAndStatus 统计用户通知任务数量（按状态）
	//
	// SELECT status, COUNT(*) as count
	// FROM @@table
	// WHERE user_id = @userID
	// GROUP BY status
	CountByUserIDAndStatus(userID int64) ([]gen.M, error)
}

// ========== QuotaTransaction 相关查询接口 ==========

// QuotaTransactionQuerier 额度流水查询接口
type QuotaTransactionQuerier interface {
	// GetBalanceByUserID 计算用户余额（按渠道分组）
	//
	// SELECT
	//   channel,
	//   COALESCE(SUM(CASE WHEN transaction_type = 'grant' THEN amount ELSE 0 END) -
	//            SUM(CASE WHEN transaction_type = 'deduct' THEN amount ELSE 0 END), 0) as balance
	// FROM @@table
	// WHERE user_id = @userID
	// GROUP BY channel
	GetBalanceByUserID(userID int64) ([]gen.M, error)

	// GetBalanceByPublicID 根据 PublicID 计算用户余额（API 常用）
	//
	// SELECT
	//   qt.channel,
	//   COALESCE(SUM(CASE WHEN qt.transaction_type = 'grant' THEN qt.amount ELSE 0 END) -
	//            SUM(CASE WHEN qt.transaction_type = 'deduct' THEN qt.amount ELSE 0 END), 0) as balance
	// FROM @@table qt
	// INNER JOIN users u ON u.id = qt.user_id
	// WHERE u.public_id = @publicID
	// GROUP BY qt.channel
	GetBalanceByPublicID(publicID int64) ([]gen.M, error)

	// ListByUserID 按用户查询流水（分页）
	//
	// SELECT * FROM @@table
	// WHERE user_id = @userID
	// ORDER BY created_at DESC
	// LIMIT @limit OFFSET @offset
	ListByUserID(userID int64, limit, offset int) ([]*gen.T, error)

	// ListByPublicID 根据 PublicID 查询流水（API 常用，支持游标分页）
	//
	// SELECT qt.* FROM @@table qt
	// INNER JOIN users u ON u.id = qt.user_id
	// WHERE u.public_id = @publicID
	//   {{if cursorID > 0}}
	//   AND qt.id < @cursorID
	//   {{end}}
	// ORDER BY qt.created_at DESC
	// LIMIT @limit
	ListByPublicID(publicID int64, cursorID int64, limit int) ([]*gen.T, error)

	// ListByUserIDAndChannel 按用户和渠道查询流水
	//
	// SELECT * FROM @@table
	// WHERE user_id = @userID
	//   AND channel = @channel
	// ORDER BY created_at DESC
	// LIMIT @limit OFFSET @offset
	ListByUserIDAndChannel(userID int64, channel string, limit, offset int) ([]*gen.T, error)

	// GetLatestTransactionByUserID 获取用户最新的流水记录
	//
	// SELECT * FROM @@table
	// WHERE user_id = @userID
	// ORDER BY created_at DESC
	// LIMIT 1
	GetLatestTransactionByUserID(userID int64) (*gen.T, error)
}

// ========== ContactAttempt 相关查询接口 ==========

// ContactAttemptQuerier 通知尝试记录查询接口
type ContactAttemptQuerier interface {
	// ListByTaskID 根据任务ID查询尝试记录
	//
	// SELECT * FROM @@table
	// WHERE task_id = @taskID
	// ORDER BY attempted_at DESC
	ListByTaskID(taskID int64) ([]*gen.T, error)

	// ListByTaskIDAndStatus 根据任务ID和状态查询尝试记录
	//
	// SELECT * FROM @@table
	// WHERE task_id = @taskID
	//   {{if status != ""}}
	//   AND status = @status
	//   {{end}}
	// ORDER BY attempted_at DESC
	ListByTaskIDAndStatus(taskID int64, status string) ([]*gen.T, error)

	// CountByTaskID 统计任务的尝试次数
	//
	// SELECT COUNT(*) as count
	// FROM @@table
	// WHERE task_id = @taskID
	CountByTaskID(taskID int64) (int64, error)

	// GetLatestByTaskID 获取任务最新的尝试记录
	//
	// SELECT * FROM @@table
	// WHERE task_id = @taskID
	// ORDER BY attempted_at DESC
	// LIMIT 1
	GetLatestByTaskID(taskID int64) (*gen.T, error)
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
		return errors.ErrDatabaseConnectionNil
	}

	g := gen.NewGenerator(gen.Config{
		OutPath:           "./internal/repository/query", // 生成代码的输出路径
		ModelPkgPath:      "AreYouOK/internal/model",
		Mode:              gen.WithDefaultQuery | gen.WithQueryInterface | gen.WithoutContext,
		FieldNullable:     true, // 字段可以为 null
		FieldCoverable:    false,
		FieldSignable:     false,
		FieldWithIndexTag: false,
		FieldWithTypeTag:  true,
	})

	g.UseDB(db)

	// 注册现有的 model，GORM Gen 会使用这些 model 而不是生成新的
	g.ApplyBasic(
		&model.User{},
		&model.DailyCheckIn{},
		&model.Journey{},
		&model.NotificationTask{},
		&model.QuotaTransaction{},
		&model.ContactAttempt{}, // 添加 ContactAttempt model
	)

	// 直接应用接口，GORM Gen 会根据接口中的类型自动匹配已注册的 model
	g.ApplyInterface(func(UserQuerier) {}, &model.User{})
	g.ApplyInterface(func(DailyCheckInQuerier) {}, &model.DailyCheckIn{})
	g.ApplyInterface(func(JourneyQuerier) {}, &model.Journey{})
	g.ApplyInterface(func(NotificationTaskQuerier) {}, &model.NotificationTask{})
	g.ApplyInterface(func(QuotaTransactionQuerier) {}, &model.QuotaTransaction{})
	g.ApplyInterface(func(ContactAttemptQuerier) {}, &model.ContactAttempt{})

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
