package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"AreYouOK/config"
)

// 也可考虑 hertz 本身自带的日志组件 ？
var Logger *zap.Logger

func Init() {
	var zapConfig zap.Config

	if config.Cfg.IsDevelopment() {
		zapConfig = zap.NewDevelopmentConfig()

		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		zapConfig.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	// 日志级别
	switch config.Cfg.LoggerLevel {
	case "DEBUG":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "INFO":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "WARN":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "ERROR":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	if !config.Cfg.IsDevelopment() {
		if config.Cfg.LoggerFormat == "text" {
			zapConfig.Encoding = "console"
		} else {
			zapConfig.Encoding = "json"
		}
	}

	// 输出路径
	if config.Cfg.LoggerOutputPath != "stdout" {
		zapConfig.OutputPaths = []string{config.Cfg.LoggerOutputPath}
		zapConfig.ErrorOutputPaths = []string{config.Cfg.LoggerOutputPath}
	}

	var err error
	Logger, err = zapConfig.Build(zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		panic("Failed to initialize zap logger: " + err.Error())
	}

	Logger.Info("Logger initialized successfully",
		zap.String("level", config.Cfg.LoggerLevel),
		zap.String("format", config.Cfg.LoggerFormat),
		zap.String("environment", config.Cfg.Environment),
	)
}

func Sync() {
	if Logger != nil {
		if err := Logger.Sync(); err != nil {
			_ = err
		}
	}
}
