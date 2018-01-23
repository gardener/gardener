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

package logger_test

import (
	"fmt"
	"os"

	. "github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("logger", func() {
	Describe("logger", func() {
		AfterEach(func() {
			Logger = nil
		})

		Describe("#NewLogger", func() {
			It("should return a pointer to a Logger object ('info' level)", func() {
				logger := NewLogger("info")

				Expect(logger.Out).To(Equal(os.Stderr))
				Expect(logger.Level).To(Equal(logrus.InfoLevel))
				Expect(Logger).To(Equal(logger))
			})

			It("should return a pointer to a Logger object ('debug' level)", func() {
				logger := NewLogger("debug")

				Expect(logger.Out).To(Equal(os.Stderr))
				Expect(logger.Level).To(Equal(logrus.DebugLevel))
				Expect(Logger).To(Equal(logger))
			})

			It("should return a pointer to a Logger object ('error' level)", func() {
				logger := NewLogger("error")

				Expect(logger.Out).To(Equal(os.Stderr))
				Expect(logger.Level).To(Equal(logrus.ErrorLevel))
				Expect(Logger).To(Equal(logger))
			})
		})

		Describe("#NewShootLogger", func() {
			It("should return an Entry object with additional fields (w/o operationID)", func() {
				logger := NewLogger("info")
				namespace := "core"
				name := "shoot01"

				shootLogger := NewShootLogger(logger, name, namespace, "")

				Expect(shootLogger.Data).To(HaveKeyWithValue("shoot", fmt.Sprintf("%s/%s", namespace, name)))
			})

			It("should return an Entry object with additional fields (w/ operationID)", func() {
				logger := NewLogger("info")
				namespace := "core"
				name := "shoot01"
				operationID := "1234"

				shootLogger := NewShootLogger(logger, name, namespace, operationID)

				Expect(shootLogger.Data).To(HaveKeyWithValue("opid", operationID))
			})
		})

		Describe("#NewSeedLogger", func() {
			It("should return an Entry object with additional fields", func() {
				logger := NewLogger("info")
				name := "seed01"

				shootLogger := NewSeedLogger(logger, name)

				Expect(shootLogger.Data).To(HaveKeyWithValue("seed", name))
			})
		})
	})
})
