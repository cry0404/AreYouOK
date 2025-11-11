package model

// QuotaChannel 额度渠道枚举
type QuotaChannel string

const (
	QuotaChannelSMS   QuotaChannel = "sms"
	QuotaChannelVoice QuotaChannel = "voice"
)

// TransactionType 交易类型枚举
type TransactionType string

const (
	TransactionTypeGrant  TransactionType = "grant"  // 充值
	TransactionTypeDeduct TransactionType = "deduct" // 扣减
)

// QuotaTransaction 额度流水模型
type QuotaTransaction struct {
	BaseModel
	UserID        int64           `gorm:"not null;index:idx_quota_transactions_user" json:"user_id"`
	Channel       QuotaChannel    `gorm:"type:varchar(16);not null" json:"channel"`
	TransactionType TransactionType `gorm:"type:varchar(16);not null" json:"transaction_type"`
	Reason        string          `gorm:"type:varchar(16);not null" json:"reason"`
	Amount        int             `gorm:"not null" json:"amount"`
	BalanceAfter  int             `gorm:"not null" json:"balance_after"`
}

// TableName 指定表名
func (QuotaTransaction) TableName() string {
	return "quota_transactions"
}

