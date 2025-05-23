// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("And", func() {
	var (
		obj                client.Object
		healthy, unhealthy health.Func
	)

	BeforeEach(func() {
		obj = &corev1.Pod{}
		healthy = func(_ client.Object) error {
			return nil
		}
		unhealthy = func(_ client.Object) error {
			return fmt.Errorf("unhealthy")
		}
	})

	It("should succeed if no funcs are given", func() {
		Expect(health.And()(obj)).To(Succeed())
	})
	It("should succeed if all funcs succeed", func() {
		Expect(health.And(healthy)(obj)).To(Succeed())
		Expect(health.And(healthy, healthy)(obj)).To(Succeed())
	})
	It("should fail if at least one func fails", func() {
		Expect(health.And(unhealthy)(obj)).NotTo(Succeed())
		Expect(health.And(healthy, unhealthy)(obj)).NotTo(Succeed())
		Expect(health.And(unhealthy, healthy)(obj)).NotTo(Succeed())
	})
	It("should fail if all funcs fail", func() {
		Expect(health.And(unhealthy, unhealthy)(obj)).NotTo(Succeed())
	})
})
