package service

import (
	"AreYouOK/internal/model"

	"AreYouOK/storage/database"
	"context"
	"fmt"
	"time"

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

	return db.Transaction(func(tx *gorm.DB) error {
		// 1. 查询钱包（使用悲观锁防止竞态）
		var wallet model.QuotaWallet
		if err := tx.Where("user_id = ? AND channel = ?", userID, channel).
			First(&wallet).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// 钱包不存在，创建新钱包, 理论上来讲，用户创建的时候钱包j
				wallet = model.QuotaWallet{
					UserID:          userID,
					Channel:         channel,
					AvailableAmount: 0,
					FrozenAmount:    0,
					UsedAmount:      0,
					TotalGranted:    0,
				}
				if err := tx.Create(&wallet).Error; err != nil {
					return fmt.Errorf("failed to create wallet: %w", err)
				}
			} else {
				return fmt.Errorf("failed to query wallet: %w", err)
			}
		}

		// 2. 检查可用额度
		if wallet.AvailableAmount < amount {
			return fmt.Errorf("%w", errors.QuotaInsufficient)
		}

		// 3. 冻结额度
		updates := map[string]interface{}{
			"available_amount": gorm.Expr("available_amount - ?", amount),
			"frozen_amount":   gorm.Expr("frozen_amount + ?", amount),
			"updated_at":      time.Now(),
		}
		if err := tx.Model(&wallet).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to freeze quota: %w", err)
		}

		// 4. 创建预扣减交易记录
		newBalance := wallet.AvailableAmount - amount
		transaction := &model.QuotaTransaction{
			UserID:          userID,
			Channel:         channel,
			TransactionType: model.TransactionTypeDeduct,
			Reason:          model.QuotaReasonPreDeduct, // 预扣减标识
			Amount:          amount,
			BalanceAfter:    newBalance,
		}

		if err := tx.Create(transaction).Error; err != nil {
			return fmt.Errorf("failed to create pre-deduct transaction: %w", err)
		}

		logger.Logger.Info("Quota pre-deducted",
			zap.Int64("user_id", userID),
			zap.String("channel", string(channel)),
			zap.Int("amount", amount),
			zap.Int("available_before", wallet.AvailableAmount),
			zap.Int("available_after", newBalance),
			zap.Int("frozen_after", wallet.FrozenAmount+amount),
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
		// 1. 查询钱包
		var wallet model.QuotaWallet
		if err := tx.Where("user_id = ? AND channel = ?", userID, channel).
			First(&wallet).Error; err != nil {
			return fmt.Errorf("failed to query wallet: %w", err)
		}

		// 2. 检查冻结额度是否足够
		if wallet.FrozenAmount < amount {
			return fmt.Errorf("insufficient frozen amount: have %d, need %d", wallet.FrozenAmount, amount)
		}

		// 3. 解冻并转移到已使用额度
		updates := map[string]interface{}{
			"frozen_amount": gorm.Expr("frozen_amount - ?", amount),
			"used_amount":   gorm.Expr("used_amount + ?", amount),
			"updated_at":    time.Now(),
		}
		if err := tx.Model(&wallet).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to confirm deduction: %w", err)
		}

		// 4. 创建确认扣减交易记录
		transaction := &model.QuotaTransaction{
			UserID:          userID,
			Channel:         channel,
			TransactionType: model.TransactionTypeDeduct,
			Reason:          model.QuotaReasonConfirmDeduct, // 确认扣减标识
			Amount:          amount,
			BalanceAfter:    wallet.AvailableAmount, // 可用余额不变
		}

		if err := tx.Create(transaction).Error; err != nil {
			return fmt.Errorf("failed to create confirm-deduct transaction: %w", err)
		}

		logger.Logger.Info("Quota deduction confirmed",
			zap.Int64("user_id", userID),
			zap.String("channel", string(channel)),
			zap.Int("amount", amount),
			zap.Int("frozen_before", wallet.FrozenAmount),
			zap.Int("frozen_after", wallet.FrozenAmount-amount),
			zap.Int("used_after", wallet.UsedAmount+amount),
		)

		return nil
	})
}

// Refund 退款（解冻）
// 将余额回复(balance_after 增加)

func (s *QuotaService) Refund(ctx context.Context, userID int64, channel model.QuotaChannel, amount int) error {
	db := database.DB().WithContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// 1. 查询钱包
		var wallet model.QuotaWallet
		if err := tx.Where("user_id = ? AND channel = ?", userID, channel).
			First(&wallet).Error; err != nil {
			return fmt.Errorf("failed to query wallet: %w", err)
		}

		// 2. 检查冻结额度是否足够
		if wallet.FrozenAmount < amount {
			return fmt.Errorf("insufficient frozen amount for refund: have %d, need %d", wallet.FrozenAmount, amount)
		}

		// 3. 解冻并恢复到可用额度
		updates := map[string]interface{}{
			"available_amount": gorm.Expr("available_amount + ?", amount),
			"frozen_amount":   gorm.Expr("frozen_amount - ?", amount),
			"updated_at":      time.Now(),
		}
		if err := tx.Model(&wallet).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to refund quota: %w", err)
		}

		// 4. 创建退款交易记录
		newBalance := wallet.AvailableAmount + amount
		transaction := &model.QuotaTransaction{
			UserID:          userID,
			Channel:         channel,
			TransactionType: model.TransactionTypeGrant,   // 退款算作充值
			Reason:          model.QuotaReasonGrantRefund, // 退款标识
			Amount:          amount,
			BalanceAfter:    newBalance,
		}

		if err := tx.Create(transaction).Error; err != nil {
			return fmt.Errorf("failed to create refund transaction: %w", err)
		}

		logger.Logger.Info("Quota refunded",
			zap.Int64("user_id", userID),
			zap.String("channel", string(channel)),
			zap.Int("amount", amount),
			zap.Int("available_before", wallet.AvailableAmount),
			zap.Int("available_after", newBalance),
			zap.Int("frozen_before", wallet.FrozenAmount),
			zap.Int("frozen_after", wallet.FrozenAmount-amount),
		)

		return nil
	})
}

// GetWallet 获取用户钱包信息
func (s *QuotaService) GetWallet(ctx context.Context, userID int64, channel model.QuotaChannel) (*model.QuotaWallet, error) {
	db := database.DB().WithContext(ctx)

	var wallet model.QuotaWallet
	if err := db.Where("user_id = ? AND channel = ?", userID, channel).
		First(&wallet).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 钱包不存在，返回空钱包
			return &model.QuotaWallet{
				UserID:          userID,
				Channel:         channel,
				AvailableAmount: 0,
				FrozenAmount:    0,
				UsedAmount:      0,
				TotalGranted:    0,
			}, nil
		}
		return nil, fmt.Errorf("failed to query wallet: %w", err)
	}

	return &wallet, nil
}

// GrantQuota 充值额度
func (s *QuotaService) GrantQuota(ctx context.Context, userID int64, channel model.QuotaChannel, amount int, reason string) error {
	db := database.DB().WithContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// 1. 查询或创建钱包
		var wallet model.QuotaWallet
		if err := tx.Where("user_id = ? AND channel = ?", userID, channel).
			First(&wallet).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// 创建新钱包
				wallet = model.QuotaWallet{
					UserID:          userID,
					Channel:         channel,
					AvailableAmount: amount,
					FrozenAmount:    0,
					UsedAmount:      0,
					TotalGranted:    amount,
				}
				if err := tx.Create(&wallet).Error; err != nil {
					return fmt.Errorf("failed to create wallet: %w", err)
				}
			} else {
				return fmt.Errorf("failed to query wallet: %w", err)
			}
		} else {
			// 更新现有钱包
			updates := map[string]interface{}{
				"available_amount": gorm.Expr("available_amount + ?", amount),
				"total_granted":   gorm.Expr("total_granted + ?", amount),
				"updated_at":      time.Now(),
			}
			if err := tx.Model(&wallet).Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to grant quota: %w", err)
			}
			wallet.AvailableAmount += amount
		}

		// 2. 创建充值交易记录
		transaction := &model.QuotaTransaction{
			UserID:          userID,
			Channel:         channel,
			TransactionType: model.TransactionTypeGrant,
			Reason:          reason,
			Amount:          amount,
			BalanceAfter:    wallet.AvailableAmount,
		}

		if err := tx.Create(transaction).Error; err != nil {
			return fmt.Errorf("failed to create grant transaction: %w", err)
		}

		logger.Logger.Info("Quota granted",
			zap.Int64("user_id", userID),
			zap.String("channel", string(channel)),
			zap.Int("amount", amount),
			zap.String("reason", reason),
			zap.Int("balance_after", wallet.AvailableAmount),
		)

		return nil
	})
}
