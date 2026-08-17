package logger

import (
	"fmt"

	"github.com/Tantrum4132/task-service/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(cfg *config.Config) (*zap.Logger, error) {
	if cfg == nil {
		return zap.NewNop(), nil
	}

	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Logging.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	var zapCfg zap.Config
	if cfg.Environment == "production" {
		zapCfg = zap.NewProductionConfig()
		zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)

	switch cfg.Logging.Format {
	case "json":
		zapCfg.Encoding = "json"
		zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	case "console":
		zapCfg.Encoding = "console"
	}

	log, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build zap logger: %w", err)
	}

	return log, nil
}
