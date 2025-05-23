// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
