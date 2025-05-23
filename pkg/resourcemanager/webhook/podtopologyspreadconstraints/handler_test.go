// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package podtopologyspreadconstraints_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/podtopologyspreadconstraints"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.Background()
		log = logr.Discard()

		handler *Handler
		pod     *corev1.Pod
	)

	BeforeEach(func() {
		ctx = admission.NewContextWithRequest(ctx, admission.Request{})

		handler = &Handler{Logger: log}
		pod = &corev1.Pod{}
	})

	Describe("#Default", func() {
		It("should not patch topology spread constraints because pod-template-hash is not available", func() {
			pod.Labels = nil

			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.TopologySpreadConstraints).To(BeNil())
		})

		It("should not patch topology spread constraints because it is not defined", func() {
			pod.Labels = map[string]string{"pod-template-hash": "123abc"}

			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.TopologySpreadConstraints).To(BeNil())
		})

		It("should add pod-template-hash to TSCs", func() {
			pod.Labels = map[string]string{"pod-template-hash": "123abc"}
			pod.Spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
				{
					TopologyKey: corev1.LabelTopologyZone,
				},
				{
					TopologyKey:   corev1.LabelHostname,
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
				},
				{
					TopologyKey:   "foo",
					LabelSelector: &metav1.LabelSelector{MatchLabels: pod.Labels},
				},
			}

			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.TopologySpreadConstraints).To(ConsistOf(
				corev1.TopologySpreadConstraint{
					TopologyKey:   corev1.LabelTopologyZone,
					LabelSelector: &metav1.LabelSelector{MatchLabels: pod.Labels},
				},
				corev1.TopologySpreadConstraint{
					TopologyKey:   corev1.LabelHostname,
					LabelSelector: &metav1.LabelSelector{MatchLabels: utils.MergeStringMaps(pod.Labels, map[string]string{"app": "test"})},
				},
				corev1.TopologySpreadConstraint{
					TopologyKey:   "foo",
					LabelSelector: &metav1.LabelSelector{MatchLabels: pod.Labels},
				},
			))
		})
	})
})
