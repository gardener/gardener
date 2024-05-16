// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"github.com/go-logr/logr"
)

func Info(msg string, keysAndValues ...interface{})             {}
func Error(err error, msg string, keysAndValues ...interface{}) {}
func WithValues(keysAndValues ...interface{}) logr.Logger       { return logr.Logger{} }
