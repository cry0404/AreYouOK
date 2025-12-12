package config

import (
	"fmt"
	"log"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

var Cfg Config

type Config struct {
	SessionSecret string `env:"SESSION_SECRET_KEY"`
	CSRFSecret    string `env:"CSRF_SECRET_KEY"`

	CaptchaSceneId     string `env:"CAPTCHA_SCENE_ID"`
	PostgreSQLPassword string `env:"POSTGRESQL_PASSWORD" envDefault:"postgres"`
	Environment        string `env:"ENVIRONMENT" envDefault:"development"`
	ServiceName        string `env:"SERVICE_NAME" envDefault:"areyouok"`
	PostgreSQLHost     string `env:"POSTGRESQL_HOST" envDefault:"localhost"`
	PostgreSQLPort     string `env:"POSTGRESQL_PORT" envDefault:"5432"`
	PostgreSQLUser     string `env:"POSTGRESQL_USER" envDefault:"postgres"`
	AlipayPrivateKey   string `env:"ALIPAY_PRIVATE_KEY"`
	AlipayAppID        string `env:"ALIPAY_APP_ID"`
	PostgreSQLDatabase string `env:"POSTGRESQL_DATABASE" envDefault:"areyouok"`
	PostgreSQLSchema   string `env:"POSTGRESQL_SCHEMA" envDefault:"public"`
	PostgreSQLSSLMode  string `env:"POSTGRESQL_SSLMODE" envDefault:"disable"`
	CaptchaProvider    string `env:"CAPTCHA_PROVIDER" envDefault:"aliyun"`
	AlipayPublicKey    string `env:"ALIPAY_PUBLIC_KEY"`
	RedisAddr          string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisPassword      string `env:"REDIS_PASSWORD" envDefault:""`
	LoggerOutputPath   string `env:"LOGGER_OUTPUT_PATH" envDefault:"stdout"`
	RedisPrefix        string `env:"REDIS_PREFIX" envDefault:"ayok"`

	// Redis Sentinel 配置
	RedisSentinelEnabled bool   `env:"REDIS_SENTINEL_ENABLED" envDefault:"false"`
	RedisSentinelMaster  string `env:"REDIS_SENTINEL_MASTER" envDefault:"mymaster"`
	RedisSentinelAddrs   string `env:"REDIS_SENTINEL_ADDRS" envDefault:""` // 逗号分隔，如 "redis-sentinel:26379"

	RabbitMQAddr     string `env:"RABBITMQ_ADDR" envDefault:"localhost"`
	RabbitMQPort     string `env:"RABBITMQ_PORT" envDefault:"5672"`
	RabbitMQUsername string `env:"RABBITMQ_USERNAME" envDefault:"guest"`
	RabbitMQPassword string `env:"RABBITMQ_PASSWORD" envDefault:"guest"`
	RabbitMQVhost    string `env:"RABBITMQ_VHOST" envDefault:"/"`

	JWTSecret    string `env:"JWT_SECRET"`
	LoggerFormat string `env:"LOGGER_FORMAT" envDefault:"text"`
	LoggerLevel  string `env:"LOGGER_LEVEL" envDefault:"INFO"`
	//AlipayAppID             string `env:"ALIPAY_APP_ID"`
	ServerHost              string `env:"SERVER_HOST" envDefault:"0.0.0.0"`
	PhoneHashSalt           string `env:"PHONEHASH_SALT"`
	ServerPort              string `env:"SERVER_PORT" envDefault:"8888"`
	AlipayGateway           string `env:"ALIPAY_GATEWAY" envDefault:"https://openapi.alipay.com/gateway.do"`
	AliCloudAccessKeyID     string `env:"ALIBABA_CLOUD_ACCESS_KEY_ID"`
	AliCloudAccessKeySecret string `env:"ALIBABA_CLOUD_ACCESS_KEY_SECRET"`

	AliPayAESKey    string `env:"ALIPAY_AES_KEY"`
	AlipayAppSecret string `env:"ALIPAY_APP_SECRET"`

	SMSProvider string `env:"SMS_PROVIDER" envDefault:"aliyun"`
	// 短信验证码配置
	SMSSignName     string `env:"SMS_SIGN_NAME"`
	SMSTemplateCode string `env:"SMS_TEMPLATE_CODE"`
	// 打卡提醒配置（发送给用户本人）
	SMSCheckInReminderSignName string `env:"SMS_CHECKIN_REMINDER_SIGN_NAME"`
	SMSCheckInReminderTemplate string `env:"SMS_CHECKIN_REMINDER_TEMPLATE"`
	// 打卡通知紧急联系人配置
	SMSCheckInReminderContactSignName string `env:"SMS_CHECKIN_REMINDER_CONTACT_SIGN_NAME"`
	SMSCheckInReminderContactTemplate string `env:"SMS_CHECKIN_REMINDER_CONTACT_TEMPLATE"`
	// 旅行联系紧急联系人配置
	SMSJourneyReminderContactSignName string `env:"SMS_JOURNEY_REMINDER_CONTACT_SIGN_NAME"`
	SMSJourneyReminderContactTemplate string `env:"SMS_JOURNEY_REMINDER_CONTACT_TEMPLATE"`
	// 行程超时提醒配置
	SMSJourneyTimeoutSignName string `env:"SMS_JOURNEY_TIMEOUT_SIGN_NAME"`
	SMSJourneyTimeoutTemplate string `env:"SMS_JOURNEY_TIMEOUT_TEMPLATE"`
	// 打卡超时提醒配置
	SMSCheckInTimeoutSignName string `env:"SMS_CHECKIN_TIMEOUT_SIGN_NAME"`
	SMSCheckInTimeoutTemplate string `env:"SMS_CHECKIN_TIMEOUT_TEMPLATE"`
	// 额度耗尽提醒配置
	SMSQuotaDepletedSignName string `env:"SMS_QUOTA_DEPLETED_SIGN_NAME"`
	SMSQuotaDepletedTemplate string `env:"SMS_QUOTA_DEPLETED_TEMPLATE"`
	EncryptionKey            string `env:"ENCRYPTION_KEY"`

	CaptchaExpireSeconds   int   `env:"CAPTCHA_EXPIRE_SECONDS" envDefault:"120"`
	CaptchaSliderThreshold int   `env:"CAPTCHA_SLIDER_THRESHOLD" envDefault:"2"`
	SnowflakeDataCenter    int64 `env:"SNOWFLAKE_DATACENTER_ID" envDefault:"1"`
	JWTRefreshDays         int   `env:"JWT_REFRESH_DAYS" envDefault:"7"`
	JWTExpireMinutes       int   `env:"JWT_EXPIRE_MINUTES" envDefault:"30"`
	WaitlistMaxUsers       int   `env:"WAITLIST_MAX_USERS" envDefault:"1000"`
	SnowflakeMachineID     int64 `env:"SNOWFLAKE_MACHINE_ID" envDefault:"1"`
	PostgreSQLMaxOpen      int   `env:"POSTGRESQL_MAX_OPEN" envDefault:"200"`
	RedisDB                int   `env:"REDIS_DB" envDefault:"0"`
	CaptchaMaxDaily        int   `env:"CAPTCHA_MAX_DAILY" envDefault:"10"`
	RateLimitRPS           int   `env:"RATE_LIMIT_RPS" envDefault:"100"`
	PostgreSQLMaxIdle      int   `env:"POSTGRESQL_MAX_IDLE" envDefault:"30"`
	DefaultSMSQuota        int   `env:"DEFAULT_SMS_QUOTA" envDefault:"100"` // 默认 SMS 额度（cents），100 cents = 20 次短信（每次 5 cents）
	RateLimitEnabled       bool  `env:"RATE_LIMIT_ENABLED" envDefault:"true"`

	OTELEXPORTERENDPOINT string `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"localhost:4317"`
}

func init() {
	if err := godotenv.Load(); err != nil {
		log.Printf("WARN: Cannot load .env file: %v, using environment variables", err)
	}

	Cfg = Config{}
	if err := env.Parse(&Cfg); err != nil {
		log.Fatalf("Failed to parse environment variables: %v", err)
	}

	validateConfig()
}

func validateConfig() {
	if Cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	if Cfg.EncryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY is required (32 bytes for AES-256)")
	}

	if len(Cfg.EncryptionKey) != 32 {
		log.Fatal("ENCRYPTION_KEY must be exactly 32 bytes for AES-256")
	}

	// if Cfg.AlipayAppID == "" {
	// 	log.Printf("WARN: ALIPAY_APP_ID is not set, Alipay integration will not work")
	// }

	// 验证 SMS 配置
	validateSMSConfig()
}

// validateSMSConfig 验证 SMS 模板配置
func validateSMSConfig() {
	// 验证默认 SMS 配置（作为降级选项）
	if Cfg.SMSSignName == "" {
		log.Printf("WARN: SMS_SIGN_NAME is not set, SMS service may not work properly")
	}
	if Cfg.SMSTemplateCode == "" {
		log.Printf("WARN: SMS_TEMPLATE_CODE is not set, SMS service may not work properly")
	}

	// 验证 checkin_reminder 配置（必须配置，无降级）
	if Cfg.SMSCheckInReminderSignName == "" {
		log.Fatal("SMS_CHECKIN_REMINDER_SIGN_NAME is required for checkin_reminder messages")
	}
	// templateCode 可以为空，会使用默认的 SMSTemplateCode
	if Cfg.SMSCheckInReminderTemplate == "" && Cfg.SMSTemplateCode == "" {
		log.Fatal("Either SMS_CHECKIN_REMINDER_TEMPLATE or SMS_TEMPLATE_CODE must be configured for checkin_reminder messages")
	}

	// 验证其他消息类型的配置（有降级逻辑，但需要确保至少有一个可用的配置）
	// checkin_reminder_contact: 如果没有配置，会使用默认的 SMSSignName 和 SMSTemplateCode
	if Cfg.SMSCheckInReminderContactSignName == "" && Cfg.SMSSignName == "" {
		log.Fatal("Either SMS_CHECKIN_REMINDER_CONTACT_SIGN_NAME or SMS_SIGN_NAME must be configured")
	}
	if Cfg.SMSCheckInReminderContactTemplate == "" && Cfg.SMSTemplateCode == "" {
		log.Fatal("Either SMS_CHECKIN_REMINDER_CONTACT_TEMPLATE or SMS_TEMPLATE_CODE must be configured")
	}

	// journey_reminder_contact
	if Cfg.SMSJourneyReminderContactSignName == "" && Cfg.SMSSignName == "" {
		log.Fatal("Either SMS_JOURNEY_REMINDER_CONTACT_SIGN_NAME or SMS_SIGN_NAME must be configured")
	}
	if Cfg.SMSJourneyReminderContactTemplate == "" && Cfg.SMSTemplateCode == "" {
		log.Fatal("Either SMS_JOURNEY_REMINDER_CONTACT_TEMPLATE or SMS_TEMPLATE_CODE must be configured")
	}

	// journey_timeout
	if Cfg.SMSJourneyTimeoutSignName == "" && Cfg.SMSSignName == "" {
		log.Fatal("Either SMS_JOURNEY_TIMEOUT_SIGN_NAME or SMS_SIGN_NAME must be configured")
	}
	if Cfg.SMSJourneyTimeoutTemplate == "" && Cfg.SMSTemplateCode == "" {
		log.Fatal("Either SMS_JOURNEY_TIMEOUT_TEMPLATE or SMS_TEMPLATE_CODE must be configured")
	}

	// checkin_timeout
	if Cfg.SMSCheckInTimeoutSignName == "" && Cfg.SMSSignName == "" {
		log.Fatal("Either SMS_CHECKIN_TIMEOUT_SIGN_NAME or SMS_SIGN_NAME must be configured")
	}
	if Cfg.SMSCheckInTimeoutTemplate == "" && Cfg.SMSTemplateCode == "" {
		log.Fatal("Either SMS_CHECKIN_TIMEOUT_TEMPLATE or SMS_TEMPLATE_CODE must be configured")
	}
}

func (c *Config) GetDSN() string {
	return "host=" + c.PostgreSQLHost +
		" port=" + c.PostgreSQLPort +
		" user=" + c.PostgreSQLUser +
		" password=" + c.PostgreSQLPassword +
		" dbname=" + c.PostgreSQLDatabase +
		" sslmode=" + c.PostgreSQLSSLMode +
		" search_path=" + c.PostgreSQLSchema
}

func (c *Config) GetRabbitMQURL() string {
	return "amqp://" + c.RabbitMQUsername + ":" + c.RabbitMQPassword + "@" + c.RabbitMQAddr + ":" + c.RabbitMQPort + c.RabbitMQVhost
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// GetSMSTemplateConfig 根据消息类型获取对应的签名和模板代码
// messageType: 消息类型，如 "checkin_reminder_contact", "journey_reminder_contact" 等
// 返回: (signName, templateCode, error)
func (c *Config) GetSMSTemplateConfig(messageType string) (signName, templateCode string, err error) {
	switch messageType {
	case "checkin_reminder":
		signName = c.SMSCheckInReminderSignName
		templateCode = c.SMSCheckInReminderTemplate
		// signName 在 validateConfig 中已验证，不会为空
	case "checkin_reminder_contact":
		signName = c.SMSCheckInReminderContactSignName
		templateCode = c.SMSCheckInReminderContactTemplate

	case "journey_reminder_contact":
		signName = c.SMSJourneyReminderContactSignName
		templateCode = c.SMSJourneyReminderContactTemplate

	case "journey_timeout":
		signName = c.SMSJourneyTimeoutSignName
		templateCode = c.SMSJourneyTimeoutTemplate

	case "checkin_timeout":
		signName = c.SMSCheckInTimeoutSignName
		templateCode = c.SMSCheckInTimeoutTemplate

	case "quota_depleted":
		signName = c.SMSQuotaDepletedSignName
		templateCode = c.SMSQuotaDepletedTemplate
	default:

		signName = c.SMSSignName
		templateCode = c.SMSTemplateCode
	}

	// 验证配置是否完整
	if signName == "" {
		return "", "", fmt.Errorf("SMS sign name not configured for message type: %s", messageType)
	}
	if templateCode == "" {
		return "", "", fmt.Errorf("SMS template code not configured for message type: %s", messageType)
	}

	return signName, templateCode, nil
}
