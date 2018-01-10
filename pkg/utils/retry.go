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

package utils

import (
	"errors"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// Retry tries a condition function <f> until it returns true or the timeout <maxWaitTime> is reached.
// Retry always waits the 5 seconds before retrying <f> the next time.
// It ensures that the function <f> is always executed at least once.
func Retry(logger *logrus.Entry, maxWaitTime time.Duration, f func() (bool, error)) error {
	var (
		startTime = time.Now().UTC()
		tries     = 1
	)

	for {
		success, err := f()
		if success {
			logger.Debug("Try number #" + strconv.Itoa(tries) + " succeeded")
			return nil
		}

		if time.Since(startTime) >= maxWaitTime {
			if err != nil {
				logger.Error("Maximum waiting time exceeded after " + strconv.Itoa(tries) + " tries, returning error")
				return err
			}
			return errors.New("Maximum waiting time exceeded after " + strconv.Itoa(tries) + " tries, but no error occurred")
		}

		logger.Info("Try number #" + strconv.Itoa(tries) + " did not succeed, trying again...")
		if err != nil {
			logger.Info(err)
		}
		tries++
		time.Sleep(5 * time.Second)
	}
}

// RetryFunc is a convenience wrapper which returns a condition function that fits the requirements of
// the Retry function.
// The function <f> must not require any arguments and only return an error. It will be executed and if it
// returns an error, the returned-tuple will be (false, err), whereby it will be (true, nil) if the execution
// of <f> was successful.
func RetryFunc(f func() error) func() (bool, error) {
	return func() (bool, error) {
		err := f()
		if err != nil {
			return false, err
		}
		return true, nil
	}
}
