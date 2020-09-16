// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

func TestEvent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Event Suite")
}

var _ = Describe("Event", func() {
	Describe("#NewGenericEventFromObject", func() {
		It("should extract the metadata and return a generic event", func() {
			obj := &corev1.ConfigMap{}

			event := NewFromObject(obj)

			Expect(event.Object).To(BeIdenticalTo(obj))
			Expect(event.Meta).To(BeIdenticalTo(obj))
		})
	})
})
