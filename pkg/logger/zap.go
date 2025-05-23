// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"fmt"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func setCommonEncoderConfigOptions(encoderConfig *zapcore.EncoderConfig) {
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
}

// MustNewZapLogger is like NewZapLogger but panics on invalid input.
func MustNewZapLogger(level string, format string, additionalOpts ...logzap.Opts) logr.Logger {
	logger, err := NewZapLogger(level, format, additionalOpts...)
	utilruntime.Must(err)

	return logger
}

// NewZapLogger creates a new logr.Logger backed by Zap.
func NewZapLogger(level string, format string, additionalOpts ...logzap.Opts) (logr.Logger, error) {
	var opts []logzap.Opts

	// map our log levels to zap levels
	var zapLevel zapcore.LevelEnabler

	switch level {
	case DebugLevel:
		zapLevel = zap.DebugLevel
	case ErrorLevel:
		zapLevel = zap.ErrorLevel
	case "", InfoLevel:
		zapLevel = zap.InfoLevel
	default:
		return logr.Logger{}, fmt.Errorf("invalid log level %q", level)
	}

	opts = append(opts, logzap.Level(zapLevel))

	// map our log format to encoder
	switch format {
	case FormatText:
		opts = append(opts, logzap.ConsoleEncoder(setCommonEncoderConfigOptions))
	case "", FormatJSON:
		opts = append(opts, logzap.JSONEncoder(setCommonEncoderConfigOptions))
	default:
		return logr.Logger{}, fmt.Errorf("invalid log format %q", format)
	}

	return logzap.New(append(opts, additionalOpts...)...), nil
}
