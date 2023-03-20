// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package healthz_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/healthz"
)

var _ = Describe("Healthz", func() {
	Describe("#HandlerFunc", func() {
		var (
			ctx     = context.TODO()
			healthz Manager
		)

		BeforeEach(func() {
			healthz = NewDefaultHealthz()
			Expect(healthz.Start(ctx)).To(Succeed())
		})

		It("should return a function that returns nil when the health check passes", func() {
			healthz.Set(true)
			Expect(CheckerFunc(healthz)(nil)).To(BeNil())
		})

		It("should return a function that returns an error when the health check does not pass", func() {
			healthz.Set(false)
			Expect(CheckerFunc(healthz)(nil)).To(MatchError(ContainSubstring("current health status is 'unhealthy'")))
		})
	})
})
