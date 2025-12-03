

-- 用户主表：记录账号基础信息与状态。
-- status 枚举值：
--   waitlisted: 内测排队中, 以及没有注册
--   onboarding: 引导中（已激活，正在完成注册流程）
--   contact: 填手机号和填紧急联系人对应的枚举值
--   active: 正常使用
--
-- 内部 id 加外部 id 模式，id 是数据库自增主键，用于内部外键关联查询
-- 外部，处理 api 请求时使用的是 public_id ，用于 api 对外暴露，防止枚举遍历
CREATE TABLE users (
  id BIGSERIAL PRIMARY KEY, -- 关联的主键 id
  avatar_url VARCHAR(255), -- 头像 url
  public_id BIGINT NOT NULL,
  alipay_open_id VARCHAR(64) NOT NULL, -- aliyun 的 openid, 做主键性能也很差
  nickname VARCHAR(64) NOT NULL DEFAULT '', -- 默认支付宝的 nickname
  phone_cipher BYTEA, -- 手机号密文
  phone_hash CHAR(64), 
  status VARCHAR(16) NOT NULL DEFAULT 'waitlisted',
  emergency_contacts JSONB DEFAULT '[]'::jsonb, -- 紧急联系人数组，最多 3 位，按 priority 排序
  
  -- 用户自定义设置部分
  timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
  daily_check_in_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  daily_check_in_deadline TIME NOT NULL DEFAULT TIME '20:00',
  daily_check_in_grace_until TIME NOT NULL DEFAULT TIME '21:00',
  daily_check_in_remind_at TIME NOT NULL DEFAULT TIME '20:00',
  journey_auto_notify BOOLEAN NOT NULL DEFAULT TRUE,
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  CONSTRAINT users_public_id_key UNIQUE (public_id),
  CONSTRAINT users_alipay_open_id_key UNIQUE (alipay_open_id),
  CONSTRAINT users_phone_hash_key UNIQUE (phone_hash)
);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_emergency_contacts ON users USING GIN (emergency_contacts);

-- JSONB 格式示例（emergency_contacts）：
-- [
--   {
--     "display_name": "妈妈",
--     "relationship": "Mother",
--     "phone_cipher_base64": "base64-encoded-cipher", -- phone_cipher 的 base64 编码（BYTEA 转 base64）
--     "phone_hash": "abc123...",
--     "priority": 1,
--     "created_at": "2025-03-01T10:00:00+08:00"
--   }
-- ]
-- 约束：最多 3 位，priority 唯一（1-3），phone_hash 唯一， 这里只需要在创建时做限制即可

-- 平安打卡记录。
CREATE TABLE daily_check_ins (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  check_in_date DATE NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending', -- 尚未打卡，根据每天预约来创建
  check_in_at TIMESTAMPTZ,                       
  reminder_sent_at TIMESTAMPTZ,                  -- 提醒打卡部分
  alert_triggered_at TIMESTAMPTZ,                -- 在什么时候开始打卡
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, check_in_date)
);
CREATE INDEX idx_daily_check_ins_status ON daily_check_ins(status);
CREATE INDEX idx_daily_check_ins_alert ON daily_check_ins(alert_triggered_at);
CREATE INDEX idx_daily_check_ins_user_date_status ON daily_check_ins(user_id, check_in_date, status); -- 对于每天的打卡的快速查询索引

-- 行程报备。
CREATE TABLE journeys (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  title VARCHAR(64) NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  expected_return_time TIMESTAMPTZ NOT NULL, --设定返回时间
  actual_return_time TIMESTAMPTZ,             -- 实际返回时间
  status VARCHAR(16) NOT NULL DEFAULT 'ongoing',
  reminder_sent_at TIMESTAMPTZ,
  alert_triggered_at TIMESTAMPTZ,
  
  -- 行程提醒执行状态
  alert_status VARCHAR(16) NOT NULL DEFAULT 'pending',
  alert_attempts INTEGER NOT NULL DEFAULT 0,
  alert_last_attempt_at TIMESTAMPTZ,
  
  -- P0.7: 延迟消息追踪（用于取消未触发的超时检查）
  timeout_message_id VARCHAR(128), -- 延迟消息的 message_id，用于在 consumer 中检查行程状态
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ    -- 基于 gorm 创建的？
);
CREATE INDEX idx_journeys_user_status ON journeys(user_id, status);
CREATE INDEX idx_journeys_expected ON journeys(expected_return_time);
CREATE INDEX idx_journeys_timeout_message_id ON journeys(timeout_message_id);

-- 额度钱包：跟踪用户每个渠道的额度状态
-- 提供高效的状态查询和冻结额度管理
CREATE TABLE quota_wallets (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  channel VARCHAR(16) NOT NULL,
  available_amount INTEGER NOT NULL DEFAULT 0,       -- 可用额度
  frozen_amount INTEGER NOT NULL DEFAULT 0,          -- 冻结额度
  used_amount INTEGER NOT NULL DEFAULT 0,            -- 已使用额度
  total_granted INTEGER NOT NULL DEFAULT 0,          -- 总充值额度

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE (user_id, channel)
);
CREATE INDEX idx_quota_user_channel_frozen ON quota_wallets(user_id, channel, frozen_amount);
CREATE INDEX idx_quota_wallets_available ON quota_wallets(available_amount);

-- 额度流水：记录充值与扣减，类似于提供支付记录之类的
-- 余额计算：查询最新一条交易的 balance_after 字段
-- 预扣减机制：通过 reason 字段区分（pre_deduct, confirm_deduct, refund）
CREATE TABLE quota_transactions (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  channel VARCHAR(16) NOT NULL, -- 区分 sms、voice
  transaction_type VARCHAR(16) NOT NULL, -- grant(充值), deduct(扣减)
  reason VARCHAR(32) NOT NULL, -- 交易原因：
                                --   充值: "grant_default", "grant_recharge", "grant_refund"
                                --   扣减: "sms_notification", "voice_notification",
                                --        "pre_deduct" (预扣减), "confirm_deduct" (确认扣减)
  amount INTEGER NOT NULL,              -- 本次的金额变动
  balance_after INTEGER NOT NULL,       -- 操作后余额，对账部分
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_quota_transactions_deleted_at ON quota_transactions(deleted_at);
CREATE INDEX idx_quota_transactions_user ON quota_transactions(user_id, created_at);
CREATE INDEX idx_quota_transactions_user_channel_created ON quota_transactions(user_id, channel, created_at DESC);

-- 通知任务：统一调度短信与外呼。
-- 注意：必须先创建 notification_tasks，因为 contact_attempts 依赖它
CREATE TABLE notification_tasks (
  id BIGSERIAL PRIMARY KEY,
  task_code BIGINT NOT NULL, -- 与 task_id 做区分，taskID 生成出来是为了在消息队列中做区别
  user_id BIGINT NOT NULL REFERENCES users(id),
  contact_priority SMALLINT, -- 紧急联系人优先级（1-3），对应 users.emergency_contacts 数组中的 priority
  contact_phone_hash CHAR(64), -- 紧急联系人手机号哈希（用于快速查找）
  category VARCHAR(32) NOT NULL, -- 通知类别：check_in_reminder, check_in_timeout, journey_timeout, journey_reminder
  channel VARCHAR(16) NOT NULL, -- 通知渠道：sms, voice
  payload JSONB NOT NULL, -- 模板变量和通知内容
  status VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending, processing, success, failed
  retry_count SMALLINT NOT NULL DEFAULT 0,
  scheduled_at TIMESTAMPTZ NOT NULL,
  processed_at TIMESTAMPTZ,
  cost_cents INTEGER NOT NULL DEFAULT 0,
  deducted BOOLEAN NOT NULL DEFAULT FALSE,
  
  -- P0.3: 短信发送状态统计字段
  sms_message_id VARCHAR(128), -- 阿里云返回的 MessageID（BizId）
  sms_status_code VARCHAR(32), -- 阿里云返回的状态码（如 "OK", "isv.BUSINESS_LIMIT_CONTROL"）
  sms_error_message VARCHAR(255), -- 错误消息（如果有）
  
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (task_code)
);
CREATE INDEX idx_notification_tasks_status ON notification_tasks(status, scheduled_at);
CREATE INDEX idx_notification_tasks_contact ON notification_tasks(user_id, contact_priority);
CREATE INDEX idx_notification_tasks_user_category_status ON notification_tasks(user_id, category, status);
CREATE INDEX idx_notification_tasks_sms_message_id ON notification_tasks(sms_message_id);
CREATE INDEX idx_notification_tasks_sms_status ON notification_tasks(sms_status_code);

-- 通知尝试：需要保留对应的通知记录方便查询
-- 注意：必须在 notification_tasks 之后创建，因为它引用了 notification_tasks(id)
CREATE TABLE contact_attempts (
  id BIGSERIAL PRIMARY KEY,
  task_id BIGINT NOT NULL REFERENCES notification_tasks(id) ON DELETE CASCADE,
  contact_priority SMALLINT NOT NULL, -- 紧急联系人优先级（1-3）
  contact_phone_hash CHAR(64) NOT NULL, -- 紧急联系人手机号哈希
  channel VARCHAR(16) NOT NULL,  -- sms or voice
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  response_code VARCHAR(32),
  response_message VARCHAR(255),
  cost_cents INTEGER NOT NULL DEFAULT 0,
  deducted BOOLEAN NOT NULL DEFAULT FALSE,
  attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_contact_attempts_task ON contact_attempts(task_id);
CREATE INDEX idx_contact_attempts_contact ON contact_attempts(contact_phone_hash);

-- ============================================
-- 注释说明
-- ============================================

-- quota_transactions.reason 字段说明：
--   充值类型（transaction_type='grant'）:
--     - "grant_default": 默认赠送
--     - "grant_recharge": 用户充值
--     - "grant_refund": 退款（预扣减失败后的退款）
--   
--   扣减类型（transaction_type='deduct'）:
--     - "sms_notification": 短信通知扣减（已废弃，改为预扣减机制）
--     - "voice_notification": 语音通知扣减
--     - "pre_deduct": 预扣减（冻结额度）
--     - "confirm_deduct": 确认扣减（解冻并正式扣除）

-- notification_tasks.category 字段说明：
--   - "check_in_reminder": 打卡提醒
--   - "check_in_timeout": 打卡超时通知
--   - "journey_timeout": 行程超时通知
--   - "journey_reminder": 行程提醒（预留）

-- journeys.timeout_message_id 字段说明：
--   记录投放的延迟消息 ID，用于在 consumer 中检查行程状态
--   如果行程已完成，consumer 可以跳过超时处理
--   RabbitMQ 延迟消息无法直接取消，只能通过检查状态跳过
