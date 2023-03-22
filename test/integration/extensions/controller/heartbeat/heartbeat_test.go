// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package heartbeat

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Extensions Heartbeat Controller tests", func() {
	var lease *coordinationv1.Lease

	BeforeEach(func() {
		fakeClock.SetTime(time.Now())

		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-extension-heartbeat",
				Namespace: testNamespace.Name,
			},
		}
	})

	It("should create heartbeat lease resource and keep it updated", func() {
		By("Wait until heartbeat lease resource is created")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(1))))
			g.Expect(lease.Spec.HolderIdentity).To(PointTo(Equal(testID)))
			g.Expect(lease.Spec.RenewTime.Equal(&metav1.MicroTime{Time: fakeClock.Now().Truncate(time.Microsecond)})).To(BeTrue())
		}).Should(Succeed())

		By("Step fake clock")
		fakeClock.Step(time.Second)

		By("Wait until heartbeat lease's RenewTime is updated")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.Spec.RenewTime.Equal(&metav1.MicroTime{Time: fakeClock.Now().Truncate(time.Microsecond)})).To(BeTrue())
		}).Should(Succeed())
	})
})
