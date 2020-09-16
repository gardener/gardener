// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logger

import (
	"fmt"
	"io"
	"io/ioutil"
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

// NewNopLogger instantiates a new logger that logs to ioutil.Discard.
func NewNopLogger() *logrus.Logger {
	logger := logrus.New()
	logger.Out = ioutil.Discard
	return logger
}

// AddWriter returns a logger that uses the tests writer (e.g., GingkoWriter) as output channel
func AddWriter(logger *logrus.Logger, writer io.Writer) *logrus.Logger {
	logger.Out = writer
	return logger
}

// NewShootLogger extends an existing logrus logger and adds an additional field containing the Shoot cluster name
// and the project in the Garden cluster to the output. If an <operationID> is provided it will be printed for every
// log message.
// Example output: time="2017-06-08T13:00:49+02:00" level=info msg="Creating namespace in seed cluster" shoot=core/crazy-botany.
func NewShootLogger(logger *logrus.Logger, shoot, project string) *logrus.Entry {
	return logger.WithField("shoot", fmt.Sprintf("%s/%s", project, shoot))
}

// NewFieldLogger extends an existing logrus logger and adds the provided additional field.
// Example output: time="2017-06-08T13:00:49+02:00" level=info msg="something" <fieldKey>=<fieldValue>.
func NewFieldLogger(logger *logrus.Logger, fieldKey, fieldValue string) *logrus.Entry {
	return logger.WithField(fieldKey, fieldValue)
}
