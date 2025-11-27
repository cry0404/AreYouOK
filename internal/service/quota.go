package service

import (
	"AreYouOK/internal/model"
	"AreYouOK/internal/repository/query"
	"AreYouOK/storage/database"
	"context"
	"fmt"

	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type QuotaService struct{}

// 主要是会不会造成竟态？ 同一时间内日程和打电话冲突
var (
	quotaService *QuotaService
	quotaOnce    sync.Once
)

func Quota() *QuotaService {

	quotaOnce.Do(func() {
		quotaService = &QuotaService{}
	})
	return quotaService
}

// PreDeduct 预扣减额度（冻结）
// 将 available 减少，通过创建一条 reason='pre_deduct' 的交易记录实现

func (s *QuotaService) PreDeduct(ctx context.Context, userID int64, channel model.QuotaChannel, amount int) error {
	db := database.DB().WithContext(ctx)

	//q := query.Use(db)

	return db.Transaction(func(tx *gorm.DB) error {
		txQ := query.Use(tx)

		//(使用 balance_after 作为最新的余额来查询
		transactions, err := txQ.QuotaTransaction.
			Where(txQ.QuotaTransaction.UserID.Eq(userID)).
			Where(txQ.QuotaTransaction.Channel.Eq(string(channel))).
			Order(txQ.QuotaTransaction.CreatedAt.Desc()).
			Limit(1).
			Find()

		if err != nil {
			return fmt.Errorf("failed to query quota balance: %w", err)
		}

		var currentBalance int

		if len(transactions) > 0 {
			currentBalance = transactions[0].BalanceAfter
		}

		if currentBalance < amount {
			return fmt.Errorf("%w", errors.QuotaInsufficient)
		}

		newBalance := currentBalance - amount

		// 考虑打卡和旅程同时结束，会不会引起竟态？
		transaction := &model.QuotaTransaction{
			UserID:          userID,
			Channel:         channel,
			TransactionType: model.TransactionTypeDeduct,
			Reason:          model.QuotaReasonPreDeduct, // 预扣减标识
			Amount:          amount,
			BalanceAfter:    newBalance,
		}

		if err := txQ.QuotaTransaction.Create(transaction); err != nil {
			return fmt.Errorf("failed to create pre-deduct transaction: %w", err)
		}

		logger.Logger.Info("Quota pre-deducted",
			zap.Int64("user_id", userID),
			zap.String("channel", string(channel)),
			zap.Int("amount", amount),
			zap.Int("balance_before", currentBalance),
			zap.Int("balance_after", newBalance),
		)

		return nil
	})
}

// ConfirmDeduction 确认扣减（解冻并正式扣除）
// 余额不变（因为已经预扣减了），但创建一条确认记录用于审计
// 也就是消费者正式消费后扣减, 需要根据消费是否成功来扣减

func (s *QuotaService) ConfirmDeduction(ctx context.Context, userID int64, channel model.QuotaChannel, amount int) error {
	db := database.DB().WithContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		txQ := query.Use(tx)

		// 查询当前余额
		transactions, err := txQ.QuotaTransaction.
			Where(txQ.QuotaTransaction.UserID.Eq(userID)).
			Where(txQ.QuotaTransaction.Channel.Eq(string(channel))).
			Order(txQ.QuotaTransaction.CreatedAt.Desc()).
			Limit(1).
			Find()
		if err != nil {
			return fmt.Errorf("failed to query quota balance: %w", err)
		}

		var currentBalance int
		if len(transactions) > 0 {
			currentBalance = transactions[0].BalanceAfter
		}

		// 确认扣减：余额不变（因为已经预扣减了），但创建一条确认记录
		transaction := &model.QuotaTransaction{
			UserID:          userID,
			Channel:         channel,
			TransactionType: model.TransactionTypeDeduct,
			Reason:          model.QuotaReasonConfirmDeduct, // 确认扣减标识
			Amount:          amount,
			BalanceAfter:    currentBalance, // 余额不变（已预扣减）
		}

		if err := txQ.QuotaTransaction.Create(transaction); err != nil {
			return fmt.Errorf("failed to create confirm-deduct transaction: %w", err)
		}

		logger.Logger.Info("Quota deduction confirmed",
			zap.Int64("user_id", userID),
			zap.String("channel", string(channel)),
			zap.Int("amount", amount),
		)

		return nil
	})
}

// Refund 退款（解冻）
// 将余额回复(balance_after 增加)

func (s *QuotaService) Refund(ctx context.Context, userID int64, channel model.QuotaChannel, amount int) error {
	db := database.DB().WithContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		txQ := query.Use(tx)

		// 查询当前余额
		transactions, err := txQ.QuotaTransaction.
			Where(txQ.QuotaTransaction.UserID.Eq(userID)).
			Where(txQ.QuotaTransaction.Channel.Eq(string(channel))).
			Order(txQ.QuotaTransaction.CreatedAt.Desc()).
			Limit(1).
			Find()
		if err != nil {
			return fmt.Errorf("failed to query quota balance: %w", err)
		}

		var currentBalance int
		if len(transactions) > 0 {
			currentBalance = transactions[0].BalanceAfter
		}

		// 退款：余额恢复
		newBalance := currentBalance + amount

		transaction := &model.QuotaTransaction{
			UserID:          userID,
			Channel:         channel,
			TransactionType: model.TransactionTypeGrant,   // 退款算作充值
			Reason:          model.QuotaReasonGrantRefund, // 退款标识
			Amount:          amount,
			BalanceAfter:    newBalance,
		}

		if err := txQ.QuotaTransaction.Create(transaction); err != nil {
			return fmt.Errorf("failed to create refund transaction: %w", err)
		}

		logger.Logger.Info("Quota refunded",
			zap.Int64("user_id", userID),
			zap.String("channel", string(channel)),
			zap.Int("amount", amount),
			zap.Int("balance_before", currentBalance),
			zap.Int("balance_after", newBalance),
		)

		return nil
	})
}
