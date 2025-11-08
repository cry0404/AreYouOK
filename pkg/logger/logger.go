package logger

import (
	"AreYouOK/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// Init 初始化 zap logger
// 注意：需要在 config 包初始化之后调用
func Init() {
	var zapConfig zap.Config

	if config.Cfg.IsDevelopment() {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	// 设置日志级别
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

	// 设置日志格式
	if config.Cfg.LoggerFormat == "text" {
		zapConfig.Encoding = "console"
	} else {
		zapConfig.Encoding = "json"
	}

	// 设置输出路径
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

// Sync 刷新日志缓冲区
// 应该在程序退出前调用，通常在 main 函数中使用 defer logger.Sync()
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
