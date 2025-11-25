package logger

import (
	"io"
	"os"
	"strings"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	hertzzap "github.com/hertz-contrib/logger/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"AreYouOK/config"
)

var (
	Logger   *zap.Logger
	logClose io.Closer
)

func Init() {
	coreLevel := zap.NewAtomicLevel()
	coreLevel.SetLevel(parseZapLevel(config.Cfg.LoggerLevel))

	opts := []hertzzap.Option{
		hertzzap.WithCoreEnc(buildEncoder()),
		hertzzap.WithCoreWs(buildWriteSyncer()),
		hertzzap.WithCoreLevel(coreLevel),
		hertzzap.WithZapOptions(
			zap.AddCaller(),
			zap.AddStacktrace(zapcore.ErrorLevel),
		),
	}

	hzLogger := hertzzap.NewLogger(opts...)
	hlog.SetLogger(hzLogger)
	hlog.SetLevel(toHlogLevel(coreLevel.Level()))

	Logger = hzLogger.Logger()
	Logger.Info("Logger initialized successfully",
		zap.String("level", strings.ToUpper(config.Cfg.LoggerLevel)),
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

	if logClose != nil {
		_ = logClose.Close()
	}
}

func buildEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	isText := config.Cfg.IsDevelopment() || strings.EqualFold(config.Cfg.LoggerFormat, "text")
	if isText {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		return zapcore.NewConsoleEncoder(encoderConfig)
	}

	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func buildWriteSyncer() zapcore.WriteSyncer {
	if strings.EqualFold(config.Cfg.LoggerOutputPath, "stdout") {
		return zapcore.AddSync(os.Stdout)
	}

	file, err := os.OpenFile(config.Cfg.LoggerOutputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		panic("failed to open log file: " + err.Error())
	}
	logClose = file

	return zapcore.AddSync(file)
}

func parseZapLevel(level string) zapcore.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return zapcore.DebugLevel
	case "INFO":
		return zapcore.InfoLevel
	case "WARN":
		return zapcore.WarnLevel
	case "ERROR":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func toHlogLevel(level zapcore.Level) hlog.Level {
	switch level {
	case zapcore.DebugLevel:
		return hlog.LevelDebug
	case zapcore.InfoLevel:
		return hlog.LevelInfo
	case zapcore.WarnLevel:
		return hlog.LevelWarn
	case zapcore.ErrorLevel:
		return hlog.LevelError
	default:
		return hlog.LevelInfo
	}
}
