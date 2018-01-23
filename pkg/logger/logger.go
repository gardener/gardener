// Copyright 2018 The Gardener Authors.
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
	"os"

	"github.com/sirupsen/logrus"
)

// Logger is the standard logger for the Gardener which is used for all messages which are not Shoot
// cluster specific.
var Logger *logrus.Logger

// NewLogger creates a new logrus logger.
// It uses STDERR as output channel and evaluates the value of the --log-level command line argument in order
// to set the log level.
// Example output: time="2017-06-08T13:00:28+02:00" level=info msg="gardener started successfully".
func NewLogger(logLevel string) *logrus.Logger {
	var level logrus.Level

	switch logLevel {
	case "debug":
		level = logrus.DebugLevel
	case "", "info":
		level = logrus.InfoLevel
	case "error":
		level = logrus.ErrorLevel
	default:
		panic("The specified log level is not supported.")
	}

	logger := &logrus.Logger{
		Out:   os.Stderr,
		Level: level,
		Formatter: &logrus.TextFormatter{
			DisableColors: true,
		},
	}
	Logger = logger
	return logger
}

// NewShootLogger extends an existing logrus logger and adds an additional field containing the Shoot cluster name
// and the project in the Garden cluster to the output. If an <operationID> is provided it will be printed for every
// log message.
// Example output: time="2017-06-08T13:00:49+02:00" level=info msg="Creating namespace in seed cluster" shoot=core/crazy-botany.
func NewShootLogger(logger *logrus.Logger, shoot, project, operationID string) *logrus.Entry {
	fields := logrus.Fields{
		"shoot": fmt.Sprintf("%s/%s", project, shoot),
	}
	if operationID != "" {
		fields["opid"] = operationID
	}
	return logger.WithFields(fields)
}

// NewSeedLogger extends an existing logrus logger and adds an additional field containing the Seed name.
// Example output: time="2017-06-08T13:00:49+02:00" level=info msg="Creating namespace in seed cluster" seed=crazy-botany.
func NewSeedLogger(logger *logrus.Logger, seed string) *logrus.Entry {
	fields := logrus.Fields{
		"seed": seed,
	}
	return logger.WithFields(fields)
}
