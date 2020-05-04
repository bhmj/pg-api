package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger implements logging functionality
type Logger interface {
	L() *zap.SugaredLogger
}

type logger struct {
	l *zap.SugaredLogger
}

// New returns new logger
func New(level uint) (Logger, error) {
	var config zap.Config
	if level < 2 { // 0,1 = none,errors
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	if level > 3 {
		level = 1
	}
	config.Level.SetLevel([]zapcore.Level{zap.FatalLevel, zap.ErrorLevel, zap.WarnLevel, zap.DebugLevel}[level])
	config.Encoding = "json"

	lg, err := config.Build()
	if err != nil {
		return nil, err
	}
	return &logger{l: lg.Sugar()}, nil
}

func (l *logger) L() *zap.SugaredLogger {
	return l.l
}
