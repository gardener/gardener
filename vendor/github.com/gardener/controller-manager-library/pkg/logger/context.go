/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package logger

import (
	"context"
	"reflect"

	"github.com/gardener/controller-manager-library/pkg/utils"
)

var ctxkey reflect.Type

func init() {
	ctxkey, _ = utils.TypeKey((*LogContext)(nil))
}

func Get(ctx context.Context) LogContext {
	cur := ctx.Value(ctxkey)
	if cur != nil {
		return cur.(LogContext)
	} else {
		return New()
	}
}

func Set(ctx context.Context, log LogContext) context.Context {
	return context.WithValue(ctx, ctxkey, log)
}

func WithLogger(ctx context.Context, key, value string) (context.Context, LogContext) {
	log := Get(ctx).NewContext(key, value)
	return Set(ctx, log), log
}

func CErrorf(ctx context.Context, msgfmt string, args ...interface{}) {
	Get(ctx).Errorf(msgfmt, args...)
}
func CWarnf(ctx context.Context, msgfmt string, args ...interface{}) {
	Get(ctx).Warnf(msgfmt, args...)
}
func CInfof(ctx context.Context, msgfmt string, args ...interface{}) {
	Get(ctx).Infof(msgfmt, args...)
}
func CDebugf(ctx context.Context, msgfmt string, args ...interface{}) {
	Get(ctx).Debugf(msgfmt, args...)
}

func CError(ctx context.Context, msg ...interface{}) {
	Get(ctx).Error(msg...)
}
func CWarn(ctx context.Context, msg ...interface{}) {
	Get(ctx).Warn(msg...)
}
func CInfo(ctx context.Context, msg ...interface{}) {
	Get(ctx).Info(msg...)
}
func CDebug(ctx context.Context, msg ...interface{}) {
	Get(ctx).Debug(msg...)
}
