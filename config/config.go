package config

import (
	"log"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

var Cfg Config

type Config struct {
	AlipayAppSecret         string `env:"ALIPAY_APP_SECRET"`
	PostgreSQLPassword      string `env:"POSTGRESQL_PASSWORD" envDefault:"postgres"`
	Environment             string `env:"ENVIRONMENT" envDefault:"development"`
	ServiceName             string `env:"SERVICE_NAME" envDefault:"areyouok"`
	PostgreSQLHost          string `env:"POSTGRESQL_HOST" envDefault:"localhost"`
	PostgreSQLPort          string `env:"POSTGRESQL_PORT" envDefault:"5432"`
	PostgreSQLUser          string `env:"POSTGRESQL_USER" envDefault:"postgres"`
	AlipayPrivateKey        string `env:"ALIPAY_PRIVATE_KEY"`
	PostgreSQLDatabase      string `env:"POSTGRESQL_DATABASE" envDefault:"areyouok"`
	PostgreSQLSchema        string `env:"POSTGRESQL_SCHEMA" envDefault:"public"`
	PostgreSQLSSLMode       string `env:"POSTGRESQL_SSLMODE" envDefault:"disable"`
	CaptchaProvider         string `env:"CAPTCHA_PROVIDER" envDefault:"aliyun"`
	AlipayPublicKey         string `env:"ALIPAY_PUBLIC_KEY"`
	RedisAddr               string `env:"REDIS_ADDR" envDefault:"localhost:6379"`
	RedisPassword           string `env:"REDIS_PASSWORD" envDefault:""`
	LoggerOutputPath        string `env:"LOGGER_OUTPUT_PATH" envDefault:"stdout"`
	RedisPrefix             string `env:"REDIS_PREFIX" envDefault:"ayok"`
	RabbitMQAddr            string `env:"RABBITMQ_ADDR" envDefault:"localhost"`
	RabbitMQPort            string `env:"RABBITMQ_PORT" envDefault:"5672"`
	RabbitMQUsername        string `env:"RABBITMQ_USERNAME" envDefault:"guest"`
	RabbitMQPassword        string `env:"RABBITMQ_PASSWORD" envDefault:"guest"`
	RabbitMQVhost           string `env:"RABBITMQ_VHOST" envDefault:"/"`
	JWTSecret               string `env:"JWT_SECRET"`
	LoggerFormat            string `env:"LOGGER_FORMAT" envDefault:"text"`
	LoggerLevel             string `env:"LOGGER_LEVEL" envDefault:"INFO"`
	AlipayAppID             string `env:"ALIPAY_APP_ID"`
	ServerHost              string `env:"SERVER_HOST" envDefault:"0.0.0.0"`
	PhoneHashSalt           string `env:"PHONEHASH_SALT"`
	ServerPort              string `env:"SERVER_PORT" envDefault:"8888"`
	AlipayGateway           string `env:"ALIPAY_GATEWAY" envDefault:"https://openapi.alipay.com/gateway.do"`
	AliCloudAccessKeyID     string `env:"ALIBABA_CLOUD_ACCESS_KEY_ID"`
	AliCloudAccessKeySecret string `env:"ALIBABA_CLOUD_ACCESS_KEY_SECRET"`
	SMSProvider             string `env:"SMS_PROVIDER" envDefault:"aliyun"`
	SMSSignName             string `env:"SMS_SIGN_NAME"`
	SMSTemplateCode         string `env:"SMS_TEMPLATE_CODE"`
	VoiceProvider           string `env:"VOICE_PROVIDER" envDefault:"aliyun"`
	VoiceAccessKey          string `env:"VOICE_ACCESS_KEY"`
	VoiceSecretKey          string `env:"VOICE_SECRET_KEY"`
	VoiceAppID              string `env:"VOICE_APP_ID"`
	EncryptionKey           string `env:"ENCRYPTION_KEY"`
	CaptchaExpireSeconds    int    `env:"CAPTCHA_EXPIRE_SECONDS" envDefault:"120"`
	CaptchaSliderThreshold  int    `env:"CAPTCHA_SLIDER_THRESHOLD" envDefault:"2"`
	SnowflakeDataCenter     int64  `env:"SNOWFLAKE_DATACENTER_ID" envDefault:"1"`
	JWTRefreshDays          int    `env:"JWT_REFRESH_DAYS" envDefault:"7"`
	JWTExpireMinutes        int    `env:"JWT_EXPIRE_MINUTES" envDefault:"30"`
	WaitlistMaxUsers        int    `env:"WAITLIST_MAX_USERS" envDefault:"1000"`
	SnowflakeMachineID      int64  `env:"SNOWFLAKE_MACHINE_ID" envDefault:"1"`
	PostgreSQLMaxOpen       int    `env:"POSTGRESQL_MAX_OPEN" envDefault:"200"`
	RedisDB                 int    `env:"REDIS_DB" envDefault:"0"`
	CaptchaMaxDaily         int    `env:"CAPTCHA_MAX_DAILY" envDefault:"10"`
	RateLimitRPS            int    `env:"RATE_LIMIT_RPS" envDefault:"100"`
	PostgreSQLMaxIdle       int    `env:"POSTGRESQL_MAX_IDLE" envDefault:"30"`
	DefaultSMSQuota         int    `env:"DEFAULT_SMS_QUOTA" envDefault:"100"`
	DefaultVoiceQuota       int    `env:"DEFAULT_VOICE_QUOTA" envDefault:"50"`
	RateLimitEnabled        bool   `env:"RATE_LIMIT_ENABLED" envDefault:"true"`
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

	if Cfg.SMSSignName == "" {
		log.Printf("WARN: SMS_SIGN_NAME is not set, SMS service may not work properly")
	}
	if Cfg.SMSTemplateCode == "" {
		log.Printf("WARN: SMS_TEMPLATE_CODE is not set, SMS service may not work properly")
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

func (c *Config) GetRabbitMQURL() string {
	return "amqp://" + c.RabbitMQUsername + ":" + c.RabbitMQPassword + "@" + c.RabbitMQAddr + ":" + c.RabbitMQPort + c.RabbitMQVhost
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}
