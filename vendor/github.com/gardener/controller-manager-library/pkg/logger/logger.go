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
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

type LogContext interface {
	NewContext(key, value string) LogContext

	Info(msg ...interface{})
	Debug(msg ...interface{})
	Warn(msg ...interface{})
	Error(msg ...interface{})

	Infof(msgfmt string, args ...interface{})
	Debugf(msgfmt string, args ...interface{})
	Warnf(msgfmt string, args ...interface{})
	Errorf(msgfmt string, args ...interface{})
}

func SetLevel(name string) error {
	lvl, err := logrus.ParseLevel(name)
	if err != nil {
		return err
	}
	defaultLogger.Infof("Setting log level to %s", lvl.String())
	logrus.SetLevel(lvl)
	defaultLogger.SetLevel(lvl)
	return nil
}

type _context struct {
	key   string
	entry *logrus.Entry
}

var _ LogContext = _context{}

var defaultLogContext = New().(_context)
var defaultLogger = &logrus.Logger{
	Out:   os.Stderr,
	Level: logrus.InfoLevel,
	Formatter: &logrus.TextFormatter{
		DisableColors: true,
	},
}

func NewContext(key, value string) LogContext {
	return _context{key: fmt.Sprintf("%s: ", value), entry: defaultLogger.WithFields(nil)}
}

func New() LogContext {
	return _context{key: "", entry: logrus.NewEntry(defaultLogger)}
}

func (this _context) NewContext(key, value string) LogContext {
	return _context{key: fmt.Sprintf("%s%s: ", this.key, value), entry: this.entry}
}

func (this _context) Info(msg ...interface{}) {
	this.entry.Infof("%s%s", this.key, fmt.Sprint(msg...))
}
func (this _context) Infof(msgfmt string, args ...interface{}) {
	this.entry.Infof(this.key+msgfmt, args...)
}

func (this _context) Debug(msg ...interface{}) {
	this.entry.Debugf("%s%s", this.key, fmt.Sprint(msg...))
}
func (this _context) Debugf(msgfmt string, args ...interface{}) {
	this.entry.Debugf(this.key+msgfmt, args...)
}

func (this _context) Warn(msg ...interface{}) {
	this.entry.Warnf("%s%s", this.key, fmt.Sprint(msg...))
}
func (this _context) Warnf(msgfmt string, args ...interface{}) {
	this.entry.Warnf(this.key+msgfmt, args...)
}

func (this _context) Error(msg ...interface{}) {
	this.entry.Errorf("%s%s", this.key, fmt.Sprint(msg...))
}
func (this _context) Errorf(msgfmt string, args ...interface{}) {
	this.entry.Errorf(this.key+msgfmt, args...)
}

func Info(msg ...interface{}) {
	defaultLogContext.Info(msg...)
}
func Infof(msgfmt string, args ...interface{}) {
	defaultLogContext.Infof(msgfmt, args...)
}

func Debug(msg ...interface{}) {
	defaultLogContext.Debug(msg...)
}
func Debugf(msgfmt string, args ...interface{}) {
	defaultLogContext.Debugf(msgfmt, args...)
}

func Warn(msg ...interface{}) {
	defaultLogContext.Warn(msg...)
}
func Warnf(msgfmt string, args ...interface{}) {
	defaultLogContext.Warnf(msgfmt, args...)
}

func Error(msg ...interface{}) {
	defaultLogContext.Error(msg...)
}
func Errorf(msgfmt string, args ...interface{}) {
	defaultLogContext.Errorf(msgfmt, args...)
}
