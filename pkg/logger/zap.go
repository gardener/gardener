// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// ZapLogger is a Logger implementation.
// If development is true, a Zap development config will be used
// (stacktraces on warnings, no sampling), otherwise a Zap production
// config will be used (stacktraces on errors, sampling).
// Additionally, the time encoding is adjusted to `zapcore.ISO8601TimeEncoder`.
// This is used by extensions for historical reasons.
// TODO: consolidate this with NewZapLogger and make everything configurable in a harmonized way
func ZapLogger(development bool) logr.Logger {
	return logzap.New(func(o *logzap.Options) {
		var encCfg zapcore.EncoderConfig
		if development {
			encCfg = zap.NewDevelopmentEncoderConfig()
		} else {
			encCfg = zap.NewProductionEncoderConfig()
		}
		setCommonEncoderConfigOptions(&encCfg)

		o.Encoder = zapcore.NewJSONEncoder(encCfg)
		o.Development = development
	})
}
