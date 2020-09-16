// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// ZapLogger is a Logger implementation.
// If development is true, a Zap development config will be used
// (stacktraces on warnings, no sampling), otherwise a Zap production
// config will be used (stacktraces on errors, sampling).
// Additionally, the time encoding is adjusted to `zapcore.ISO8601TimeEncoder`.
func ZapLogger(development bool) logr.Logger {
	return logzap.New(func(o *logzap.Options) {
		var encCfg zapcore.EncoderConfig
		if development {
			encCfg = zap.NewDevelopmentEncoderConfig()
		} else {
			encCfg = zap.NewProductionEncoderConfig()
		}
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

		o.Encoder = zapcore.NewJSONEncoder(encCfg)
		o.Development = development
	})
}
