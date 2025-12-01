package model

import (
	"time"
)

// QuotaChannel 额度渠道枚举
type QuotaChannel string

const (
	QuotaChannelSMS   QuotaChannel = "sms"
	//QuotaChannelVoice QuotaChannel = "voice"  --废弃，不再支持
)

// QuotaWallet 额度钱包模型
type QuotaWallet struct {
	ID             int64         `gorm:"primaryKey" json:"id"`
	UserID         int64         `gorm:"not null;index:idx_quota_user_channel_frozen" json:"user_id"`
	Channel        QuotaChannel  `gorm:"type:varchar(16);not null;index:idx_quota_user_channel_frozen" json:"channel"`
	AvailableAmount int          `gorm:"not null;default:0" json:"available_amount"`
	FrozenAmount   int          `gorm:"not null;default:0" json:"frozen_amount"`
	UsedAmount     int          `gorm:"not null;default:0" json:"used_amount"`
	TotalGranted   int          `gorm:"not null;default:0" json:"total_granted"`
	CreatedAt      time.Time    `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time    `gorm:"not null" json:"updated_at"`
}

// TableName 指定表名
func (QuotaWallet) TableName() string {
	return "quota_wallets"
}

// TransactionType 交易类型枚举
type TransactionType string

const (
	TransactionTypeGrant  TransactionType = "grant"  // 充值
	TransactionTypeDeduct TransactionType = "deduct" // 扣减
)

// QuotaTransactionReason 交易原因常量
// 充值类型（transaction_type='grant'）:
//   - "grant_default": 默认赠送
//   - "grant_recharge": 用户充值
//   - "grant_refund": 退款（预扣减失败后的退款）
//
// 扣减类型（transaction_type='deduct'）:
//   - "sms_notification": 短信通知扣减（已废弃，改为预扣减机制）
// 
//   - "pre_deduct": 预扣减（冻结额度）
//   - "confirm_deduct": 确认扣减（解冻并正式扣除）
const (
	// 充值原因
	QuotaReasonGrantDefault  = "grant_default"  // 默认赠送
	QuotaReasonGrantRecharge = "grant_recharge" // 用户充值
	QuotaReasonGrantRefund   = "grant_refund"   // 退款

	// 扣减原因
	QuotaReasonSMSNotification   = "sms_notification"   // 短信通知扣减（已废弃）
	QuotaReasonVoiceNotification = "voice_notification" // 语音通知扣减
	QuotaReasonPreDeduct         = "pre_deduct"         // 预扣减（冻结额度）
	QuotaReasonConfirmDeduct     = "confirm_deduct"     // 确认扣减（解冻并正式扣除）
)

// QuotaTransaction 额度流水模型
// 余额计算：查询最新一条交易的 balance_after 字段
// 预扣减机制：通过 reason 字段区分（pre_deduct, confirm_deduct, refund）
type QuotaTransaction struct {
	Channel         QuotaChannel    `gorm:"type:varchar(16);not null" json:"channel"`
	TransactionType TransactionType `gorm:"type:varchar(16);not null" json:"transaction_type"`
	Reason          string          `gorm:"type:varchar(32);not null" json:"reason"` // 扩展为 32 字符，支持更详细的 reason
	BaseModel
	UserID       int64 `gorm:"not null;index:idx_quota_transactions_user;index:idx_quota_transactions_user_channel_created" json:"user_id"`
	Amount       int   `gorm:"not null" json:"amount"`
	BalanceAfter int   `gorm:"not null" json:"balance_after"`
}

// TableName 指定表名
func (QuotaTransaction) TableName() string {
	return "quota_transactions"
}
