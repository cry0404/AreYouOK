package config

import (
	"log"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

var Cfg Config

type Config struct {
	// 服务配置
	ServerPort  string `env:"SERVER_PORT" envDefault:"8888"`
	ServerHost  string `env:"SERVER_HOST" envDefault:"0.0.0.0"`
	Environment string `env:"ENVIRONMENT" envDefault:"development"` // development, staging, production
	ServiceName string `env:"SERVICE_NAME" envDefault:"areyouok"`

	// PostgreSQL 配置
	PostgreSQLHost     string `env:"POSTGRESQL_HOST" envDefault:"localhost"`
	PostgreSQLPort     string `env:"POSTGRESQL_PORT" envDefault:"5432"`
	PostgreSQLUser     string `env:"POSTGRESQL_USER" envDefault:"postgres"`
	PostgreSQLPassword string `env:"POSTGRESQL_PASSWORD" envDefault:"postgres"`
	PostgreSQLDatabase string `env:"POSTGRESQL_DATABASE" envDefault:"areyouok"`
	PostgreSQLSchema   string `env:"POSTGRESQL_SCHEMA" envDefault:"public"`
	PostgreSQLSSLMode  string `env:"POSTGRESQL_SSLMODE" envDefault:"disable"`
	PostgreSQLMaxIdle  int    `env:"POSTGRESQL_MAX_IDLE" envDefault:"30"`
	PostgreSQLMaxOpen  int    `env:"POSTGRESQL_MAX_OPEN" envDefault:"200"`

	// Redis 配置
	RedisAddr     string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisPassword string `env:"REDIS_PASSWORD" envDefault:""`
	RedisDB       int    `env:"REDIS_DB" envDefault:"0"`
	RedisPrefix   string `env:"REDIS_PREFIX" envDefault:"ayok"`

	// RabbitMQ 配置
	RabbitMQAddr     string `env:"RABBITMQ_ADDR" envDefault:"localhost"`
	RabbitMQPort     string `env:"RABBITMQ_PORT" envDefault:"5672"`
	RabbitMQUsername string `env:"RABBITMQ_USERNAME" envDefault:"guest"`
	RabbitMQPassword string `env:"RABBITMQ_PASSWORD" envDefault:"guest"`
	RabbitMQVhost    string `env:"RABBITMQ_VHOST" envDefault:"/"`

	// JWT 配置
	JWTSecret        string `env:"JWT_SECRET"` // 必填，用于签名 JWT
	JWTExpireMinutes int    `env:"JWT_EXPIRE_MINUTES" envDefault:"30"`
	JWTRefreshDays   int    `env:"JWT_REFRESH_DAYS" envDefault:"7"`

	// 支付宝小程序配置
	AlipayAppID      string `env:"ALIPAY_APP_ID"`
	AlipayAppSecret  string `env:"ALIPAY_APP_SECRET"`
	AlipayPublicKey  string `env:"ALIPAY_PUBLIC_KEY"`
	AlipayPrivateKey string `env:"ALIPAY_PRIVATE_KEY"`
	AlipayGateway    string `env:"ALIPAY_GATEWAY" envDefault:"https://openapi.alipay.com/gateway.do"`

	// 短信服务配置
	SMSProvider     string `env:"SMS_PROVIDER" envDefault:"aliyun"` // aliyun, tencent, etc.
	SMSAccessKey    string `env:"SMS_ACCESS_KEY"`
	SMSSecretKey    string `env:"SMS_SECRET_KEY"`
	SMSSignName     string `env:"SMS_SIGN_NAME"`
	SMSTemplateCode string `env:"SMS_TEMPLATE_CODE"`

	// 外呼服务配置
	VoiceProvider  string `env:"VOICE_PROVIDER" envDefault:"aliyun"` // aliyun, tencent, etc.
	VoiceAccessKey string `env:"VOICE_ACCESS_KEY"`
	VoiceSecretKey string `env:"VOICE_SECRET_KEY"`
	VoiceAppID     string `env:"VOICE_APP_ID"`

	// 加密配置
	EncryptionKey string `env:"ENCRYPTION_KEY"` // 用于加密手机号等敏感数据，32字节 AES-256

	// Snowflake ID 生成器配置
	SnowflakeMachineID  int64 `env:"SNOWFLAKE_MACHINE_ID" envDefault:"1"`
	SnowflakeDataCenter int64 `env:"SNOWFLAKE_DATACENTER_ID" envDefault:"1"`

	// 日志配置
	LoggerLevel      string `env:"LOGGER_LEVEL" envDefault:"INFO"`
	LoggerFormat     string `env:"LOGGER_FORMAT" envDefault:"text"` // json, text
	LoggerOutputPath string `env:"LOGGER_OUTPUT_PATH" envDefault:"stdout"`

	//最后扩展时考虑是否加入
	// // 链路追踪配置
	// TracingEnabled  bool    `env:"TRACING_ENABLED" envDefault:"false"`
	// TracingEndpoint string  `env:"TRACING_ENDPOINT" envDefault:"http://localhost:14268/api/traces"`
	// TracingSampler  float64 `env:"TRACING_SAMPLER" envDefault:"0.01"`

	// 速率限制配置, 配置在中间件内
	RateLimitEnabled bool `env:"RATE_LIMIT_ENABLED" envDefault:"true"`
	RateLimitRPS     int  `env:"RATE_LIMIT_RPS" envDefault:"100"` // 每秒请求数

	// 验证码配置
	CaptchaExpireSeconds   int `env:"CAPTCHA_EXPIRE_SECONDS" envDefault:"60"`
	CaptchaMaxDaily        int `env:"CAPTCHA_MAX_DAILY" envDefault:"10"`
	CaptchaSliderThreshold int `env:"CAPTCHA_SLIDER_THRESHOLD" envDefault:"2"` // 超过此次数需要滑块验证

	// 额度配置
	DefaultSMSQuota   int `env:"DEFAULT_SMS_QUOTA" envDefault:"100"`  // 新用户默认短信额度
	DefaultVoiceQuota int `env:"DEFAULT_VOICE_QUOTA" envDefault:"50"` // 新用户默认外呼额度

	// 内测配置
	WaitlistMaxUsers int    `env:"WAITLIST_MAX_USERS" envDefault:"1000"` // 内测最大用户数
	WaitlistBatchTag string `env:"WAITLIST_BATCH_TAG" envDefault:"beta_v1"`
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

	if Cfg.AlipayAppID == "" {
		log.Printf("WARN: ALIPAY_APP_ID is not set, Alipay integration will not work")
	}

	if Cfg.SMSAccessKey == "" {
		log.Printf("WARN: SMS_ACCESS_KEY is not set, SMS service will not work")
	}

	if Cfg.VoiceAccessKey == "" {
		log.Printf("WARN: VOICE_ACCESS_KEY is not set, Voice service will not work")
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


func (c *Config) GetRedisAddr() string {
	return c.RedisAddr
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