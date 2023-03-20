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
		healthy = func(o client.Object) error {
			return nil
		}
		unhealthy = func(o client.Object) error {
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
