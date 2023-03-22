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

package healthz

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Default", func() {
	ctx := context.TODO()

	Describe("defaultHealthz", func() {
		var d *defaultHealthz

		BeforeEach(func() {
			d = &defaultHealthz{}
		})

		Describe("#Name", func() {
			It("should return the correct name", func() {
				Expect(d.Name()).To(Equal("default"))
			})
		})

		Describe("#Start", func() {
			It("should start the manager", func() {
				Expect(d.Start(ctx)).To(Succeed())
				Expect(d.health).To(BeTrue())
			})
		})

		Describe("#Stop", func() {
			It("should stop the manager", func() {
				d.Stop()
				Expect(d.health).To(BeFalse())
			})
		})

		Describe("#Set", func() {
			It("should correctly set the status to true", func() {
				Expect(d.Start(ctx)).To(Succeed())
				d.Set(true)
				Expect(d.health).To(BeTrue())
			})

			It("should correctly set the status to false", func() {
				Expect(d.Start(ctx)).To(Succeed())
				d.Set(false)
				Expect(d.health).To(BeFalse())
			})

			It("should set the status to true (HealthManager not started)", func() {
				d.Set(true)
				Expect(d.health).To(BeTrue())
			})

			It("should set the status to true (HealthManager already stopped)", func() {
				Expect(d.Start(ctx)).To(Succeed())
				Expect(d.health).To(BeTrue())
				d.Stop()
				Expect(d.health).To(BeFalse())

				d.Set(true)
				Expect(d.health).To(BeTrue())
			})
		})

		Describe("#Get", func() {
			It("should get the correct status (true)", func() {
				d.health = true
				Expect(d.Get()).To(BeTrue())
			})

			It("should get the correct status (false)", func() {
				d.health = false
				Expect(d.Get()).To(BeFalse())
			})
		})
	})
})
