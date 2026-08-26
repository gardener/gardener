// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package secretnamechange_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/nodeagent/controller/secretnamechange"
)

var _ = Describe("Add", func() {
	Describe("#NodePredicate", func() {
		var (
			p    predicate.Predicate
			node *corev1.Node
		)

		BeforeEach(func() {
			p = (&Reconciler{}).NodePredicate()
			node = &corev1.Node{}
		})

		Describe("#Create", func() {
			It("should return false when label is not present", func() {
				Expect(p.Create(event.CreateEvent{Object: node})).To(BeFalse())
			})

			It("should return true when label is present", func() {
				metav1.SetMetaDataLabel(&node.ObjectMeta, v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName, "cloud-config-foo")
				Expect(p.Create(event.CreateEvent{Object: node})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false when label is not present on either object", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: node, ObjectNew: node})).To(BeFalse())
			})

			It("should return true when label got set", func() {
				oldNode := node.DeepCopy()
				metav1.SetMetaDataLabel(&node.ObjectMeta, v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName, "cloud-config-foo")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: node})).To(BeTrue())
			})

			It("should return true when label value changed", func() {
				metav1.SetMetaDataLabel(&node.ObjectMeta, v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName, "cloud-config-old")
				oldNode := node.DeepCopy()
				metav1.SetMetaDataLabel(&node.ObjectMeta, v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName, "cloud-config-new")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: node})).To(BeTrue())
			})

			It("should return false when label value did not change", func() {
				metav1.SetMetaDataLabel(&node.ObjectMeta, v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName, "cloud-config-foo")
				Expect(p.Update(event.UpdateEvent{ObjectOld: node, ObjectNew: node})).To(BeFalse())
			})

			It("should return true when label got removed", func() {
				metav1.SetMetaDataLabel(&node.ObjectMeta, v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName, "cloud-config-foo")
				oldNode := node.DeepCopy()
				node = &corev1.Node{}
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: node})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
