// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package podtopologyspreadconstraints_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("PodTopologySpreadConstraints tests", func() {
	var pod *corev1.Pod

	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "foo-container",
						Image: "foo",
					},
				},
			},
		}

		DeferCleanup(func() {
			Expect(testClient.Delete(ctx, pod)).To(Succeed())
		})
	})

	Context("when pod has pod-template-hash (belongs to deployment)", func() {
		var specHash string

		BeforeEach(func() {
			specHash = "123abc"
			metav1.SetMetaDataLabel(&pod.ObjectMeta, "pod-template-hash", specHash)
			pod.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
				{
					MaxSkew:           1,
					TopologyKey:       corev1.LabelTopologyZone,
					WhenUnsatisfiable: corev1.DoNotSchedule,
					MatchLabelKeys: []string{
						"pod-template-hash",
					},
				},
			}
		})

		It("should add label selector to TSC and remove matchLabelKeys if present", func() {
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			Expect(pod.Spec.TopologySpreadConstraints).To(ConsistOf(corev1.TopologySpreadConstraint{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pod-template-hash": specHash,
					},
				},
				MaxSkew:           1,
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
			}))
		})
	})

	Context("when pod does not have pod-template-hash", func() {
		BeforeEach(func() {
			pod.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
				{
					MaxSkew:           1,
					TopologyKey:       corev1.LabelTopologyZone,
					WhenUnsatisfiable: corev1.DoNotSchedule,
				},
			}
		})

		It("should not add label selector to TSC", func() {
			Expect(testClient.Create(ctx, pod)).To(Succeed())

			Expect(pod.Spec.TopologySpreadConstraints).To(ConsistOf(corev1.TopologySpreadConstraint{
				MaxSkew:           1,
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.DoNotSchedule,
			}))
		})
	})

	Context("when pod should not be considered", func() {
		var tsc corev1.TopologySpreadConstraint

		BeforeEach(func() {
			tsc = corev1.TopologySpreadConstraint{
				MaxSkew:           2,
				TopologyKey:       corev1.LabelTopologyZone,
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"pod-template-hash": "789xyz",
					},
				},
			}

			// Deliberately set a different hash here to later one check that it hasn't changed.
			metav1.SetMetaDataLabel(&pod.ObjectMeta, "pod-template-hash", "123abc")
			pod.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{tsc}

			Context("when pod specifies skipping the webhook", func() {
				BeforeEach(func() {
					metav1.SetMetaDataLabel(&pod.ObjectMeta, "topology-spread-constraints.resources.gardener.cloud/skip", "")
				})

				It("should not mutate pod's TSC", func() {
					Expect(testClient.Create(ctx, pod)).To(Succeed())

					Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
					Expect(pod.Spec.TopologySpreadConstraints).To(BeEquivalentTo(tsc))
				})
			})

			Context("when pod belongs to Gardener-Resource-Manager", func() {
				BeforeEach(func() {
					metav1.SetMetaDataLabel(&pod.ObjectMeta, "app", "gardener-resource-manager")
				})

				It("not mutate pod", func() {
					Expect(testClient.Create(ctx, pod)).To(Succeed())

					Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
					Expect(pod.Spec.TopologySpreadConstraints).To(BeEquivalentTo(tsc))
				})
			})
		})
	})
})
