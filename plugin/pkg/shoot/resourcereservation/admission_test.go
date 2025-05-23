// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcereservation

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/utils/ptr"

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
		DescribeTable("should only handle CREATE and UPDATE operation",
			func(typeDependentReservations bool) {
				plugin := New(typeDependentReservations, nil)
				Expect(plugin.Handles(admission.Create)).To(BeTrue())
				Expect(plugin.Handles(admission.Update)).To(BeTrue())
				Expect(plugin.Handles(admission.Connect)).NotTo(BeTrue())
				Expect(plugin.Handles(admission.Delete)).NotTo(BeTrue())
			},
			Entry("for disabled type dependent reservations", false),
			Entry("for enabled type dependent reservations", true),
		)
	})

	Describe("#Admit", func() {
		var (
			ctx      context.Context
			userInfo *user.DefaultInfo

			plugin              *ResourceReservation
			coreInformerFactory gardencoreinformers.SharedInformerFactory
			cloudProfile        gardencorev1beta1.CloudProfile

			shoot           *core.Shoot
			namespace       = "test"
			machineTypeName = "n1-standard-2"

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
					CloudProfileName: ptr.To("profile"),
					Provider: core.Provider{
						Workers: workersBase,
					},
				},
			}
		)

		var parsedLabelSelector labels.Selector
		var labelSelector = &metav1.LabelSelector{}
		var typeDependentReservations bool

		setupProfile := func(typeDependentReservations bool, selector labels.Selector) {
			plugin = New(typeDependentReservations, selector).(*ResourceReservation)
			plugin.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			plugin.SetCoreInformerFactory(coreInformerFactory)
			Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(&cloudProfile)).To(Succeed())
		}

		JustBeforeEach(func() {
			parsedLabelSelector, _ = metav1.LabelSelectorAsSelector(labelSelector)
			setupProfile(typeDependentReservations, parsedLabelSelector)
		})

		BeforeEach(func() {
			ctx = context.Background()
			userInfo = &user.DefaultInfo{Name: "foo"}
			shoot = shootBase.DeepCopy()
			cloudProfile = *cloudProfileBase.DeepCopy()
		})

		Context("with type dependent resource reservations", func() {
			BeforeEach(func() {
				typeDependentReservations = true
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

				Context("with a label selector configured", func() {
					BeforeEach(func() {
						labelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"shoot.gardener.cloud/worker-specific-reservations": "true"}}
					})

					Context("when the Shoot label matches the label selector", func() {
						BeforeEach(func() {
							metav1.SetMetaDataLabel(&shoot.ObjectMeta, "shoot.gardener.cloud/worker-specific-reservations", "true")
						})

						It("should inject worker specific resource reservations", func() {
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

					Context("when the Shoot label doesn't match the label selector", func() {
						BeforeEach(func() {
							metav1.SetMetaDataLabel(&shoot.ObjectMeta, "shoot.gardener.cloud/worker-specific-reservations", "false")
						})

						It("should not inject resource reservations and use default static reservations instead", func() {
							expectedShoot := shoot.DeepCopy()
							cpu := resource.MustParse("80m")
							memory := resource.MustParse("1Gi")
							pid := resource.MustParse("20k")
							expectedShoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{
								KubeReserved: &core.KubeletConfigReserved{
									CPU:    &cpu,
									Memory: &memory,
									PID:    &pid,
								},
							}
							attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
							Expect(shoot).To(Equal(expectedShoot))
						})
					})
				})

				Context("with no label selector configured", func() {
					BeforeEach(func() {
						labelSelector = &metav1.LabelSelector{}
					})

					It("should not overwrite worker pool resource reservations", func() {
						cpu := resource.MustParse("42m")
						memory := resource.MustParse("512Mi")
						pid := resource.MustParse("31k")
						shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{
							Kubelet: &core.KubeletConfig{
								KubeReserved: &core.KubeletConfigReserved{
									CPU:    &cpu,
									Memory: &memory,
									PID:    &pid,
								},
							},
						}

						expectedShoot := shoot.DeepCopy()

						attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
						Expect(shoot).To(Equal(expectedShoot))
					})

					It("should skip shoots with shoot global resource reservations", func() {
						cpu := resource.MustParse("42m")
						memory := resource.MustParse("512Mi")
						pid := resource.MustParse("31k")
						shoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{
							KubeReserved: &core.KubeletConfigReserved{
								CPU:    &cpu,
								Memory: &memory,
								PID:    &pid,
							},
						}

						expectedShoot := shoot.DeepCopy()

						attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
						Expect(shoot).To(Equal(expectedShoot))
					})
				})
			})

			Context("with static resource reservations", func() {
				BeforeEach(func() {
					typeDependentReservations = false
				})

				It("should inject default shoot global resource reservations", func() {
					expectedShoot := shoot.DeepCopy()
					cpu := resource.MustParse("80m")
					memory := resource.MustParse("1Gi")
					pid := resource.MustParse("20k")
					expectedShoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{
						KubeReserved: &core.KubeletConfigReserved{
							CPU:    &cpu,
							Memory: &memory,
							PID:    &pid,
						},
					}

					attrs := admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(plugin.Admit(ctx, attrs, nil)).To(Succeed())
					Expect(shoot).To(Equal(expectedShoot))
				})
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
		Entry("500MiB", 500*mib, 255*mib),
		Entry("1GiB", 1*gib, 256*mib),
		Entry("2GiB", 2*gib, 512*mib),
		Entry("4GiB", 4*gib, 1*gib),
		Entry("8GiB", 8*gib, gib+4*gib/5),
		Entry("16GiB", 16*gib, gib+4*gib/5+8*gib/10),
		Entry("32GiB", 32*gib, gib+4*gib/5+8*gib/10+16*gib/100*6),
		Entry("64GiB", 64*gib, gib+4*gib/5+8*gib/10+48*gib/100*6),
		Entry("128GiB", 128*gib, gib+4*gib/5+8*gib/10+112*gib/100*6),
		Entry("256GiB", 256*gib, gib+4*gib/5+8*gib/10+112*gib/100*6+128*gib/100*2),
		Entry("18GiB", 18*gib, gib+4*gib/5+8*gib/10+2*gib/100*6),
		Entry("42GiB", 42*gib, gib+4*gib/5+8*gib/10+26*gib/100*6),
		Entry("1TiB", 1024*gib, gib+4*gib/5+8*gib/10+112*gib/100*6+896*gib/100*2),
	)
})
