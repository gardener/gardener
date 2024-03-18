// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package resourcereservation

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
)

var _ = Describe("resourcereservation", func() {

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ShootResourceReservation"))
		})
	})

	Describe("#Handles", func() {
		It("should only handle CREATE and UPDATE operation", func() {
			plugin := New()
			Expect(plugin.Handles(admission.Create)).To(BeTrue())
			Expect(plugin.Handles(admission.Update)).To(BeTrue())
			Expect(plugin.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(plugin.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#Admit", func() {
		var (
			ctx      context.Context
			userInfo *user.DefaultInfo

			plugin              *ResourceReservation
			coreInformerFactory gardencoreinformers.SharedInformerFactory
			cloudProfile        gardencorev1beta1.CloudProfile

			shoot *core.Shoot
			// oldShoot            core.Shoot
			namespace       string = "test"
			machineTypeName string = "n1-standard-2"
			volumeTypeName  string = "pd-standard"

			cloudProfileBase = gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					MachineTypes: []gardencorev1beta1.MachineType{
						{
							Name:   machineTypeName,
							CPU:    resource.MustParse("2"),
							GPU:    resource.MustParse("0"),
							Memory: resource.MustParse("5Gi"),
						},
					},
					VolumeTypes: []gardencorev1beta1.VolumeType{
						{
							Name:  volumeTypeName,
							Class: "standard",
						},
					},
				},
			}

			workersBase = []core.Worker{
				{
					Name: "test-worker-1",
					Machine: core.Machine{
						Type: machineTypeName,
					},
					Maximum: 1,
					Minimum: 1,
				},
			}

			shootBase = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "test-shoot",
				},
				Spec: core.ShootSpec{
					CloudProfileName: "profile",
					Provider: core.Provider{
						Workers: workersBase,
					},
				},
			}
		)

		BeforeEach(func() {
			ctx = context.Background()
			userInfo = &user.DefaultInfo{Name: "foo"}
			shoot = shootBase.DeepCopy()
			cloudProfile = *cloudProfileBase.DeepCopy()

			plugin = New().(*ResourceReservation)
			plugin.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			plugin.SetCoreInformerFactory(coreInformerFactory)
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
		})

		Context("inject resource reservation", func() {
			It("should inject resource reservations", func() {
				expectedShoot := shoot.DeepCopy()
				worker := &expectedShoot.Spec.Provider.Workers[0]
				cpu := resource.NewMilliQuantity(70, resource.BinarySI)
				memory := resource.NewQuantity(1288490188, resource.BinarySI)
				pid := resource.MustParse("20k")
				worker.Kubernetes = &core.WorkerKubernetes{
					Kubelet: &core.KubeletConfig{
						KubeReserved: &core.KubeletConfigReserved{
							CPU:    cpu,
							Memory: memory,
							PID:    &pid,
						},
					},
				}

				attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
				Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
				Expect(shoot).To(Equal(expectedShoot))
			})
		})
	})

	DescribeTable("calculate CPU reservations",
		func(cpuMillis int, expectedReservationMillis int) {
			reservation := calculateCPUReservation(int64(cpuMillis))
			Expect(reservation).To(Equal(int64(expectedReservationMillis)))
		},
		Entry("one cpu core", 1000, 60),
		Entry("two cpu cores", 2000, 70),
		Entry("three cpu cores", 3000, 75),
		Entry("four cpu cores", 4000, 80),
		Entry("five cpu cores", 5000, 82),
		Entry("six cpu cores", 6000, 85),
		Entry("ten cpu cores", 10000, 95),
	)

	DescribeTable("calculate memory reservations",
		func(memory int, expectedReservation int) {
			reservation := calculateMemoryReservation(int64(memory))
			Expect(reservation).To(Equal(int64(expectedReservation)))
		},
		Entry("500MiB", 500*MiB, 255*MiB),
		Entry("1GiB", 1*GiB, 256*MiB),
		Entry("2GiB", 2*GiB, 512*MiB),
		Entry("4GiB", 4*GiB, 1*GiB),
		Entry("8GiB", 8*GiB, GiB+4*GiB/5),
		Entry("16GiB", 16*GiB, GiB+4*GiB/5+8*GiB/10),
		Entry("32GiB", 32*GiB, GiB+4*GiB/5+8*GiB/10+16*GiB/100*6),
		Entry("64GiB", 64*GiB, GiB+4*GiB/5+8*GiB/10+48*GiB/100*6),
		Entry("128GiB", 128*GiB, GiB+4*GiB/5+8*GiB/10+112*GiB/100*6),
		Entry("256GiB", 256*GiB, GiB+4*GiB/5+8*GiB/10+112*GiB/100*6+128*GiB/100*2),
		Entry("18GiB", 18*GiB, GiB+4*GiB/5+8*GiB/10+2*GiB/100*6),
		Entry("42GiB", 42*GiB, GiB+4*GiB/5+8*GiB/10+26*GiB/100*6),
		Entry("1TiB", 1024*GiB, GiB+4*GiB/5+8*GiB/10+112*GiB/100*6+896*GiB/100*2),
	)
})
